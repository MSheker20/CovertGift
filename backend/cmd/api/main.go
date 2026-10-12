package main

import (
	"log"
	"net/http"
	"time"

	"github.com/MSheker20/CovertGift/backend/internal/config"
	"github.com/MSheker20/CovertGift/backend/internal/controllers"
	"github.com/MSheker20/CovertGift/backend/internal/middleware"
	"github.com/MSheker20/CovertGift/backend/internal/models"
	"github.com/MSheker20/CovertGift/backend/internal/repository"
	"github.com/MSheker20/CovertGift/backend/internal/services"
	"github.com/MSheker20/CovertGift/backend/pkg/gemini"
	"github.com/gin-gonic/gin"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func main() {
	// 1. Ortam ayarlarını yükle (.env)
	cfg := config.Load()

	// 2. PostgreSQL veritabanına bağlan
	db, err := gorm.Open(postgres.Open(cfg.DatabaseURL), &gorm.Config{})
	if err != nil {
		log.Fatalf("❌ Veritabanına bağlanılamadı: %v", err)
	}

	// 3. Bağlantı Havuzu (Connection Pool) Ayarları
	sqlDB, err := db.DB()
	if err != nil {
		log.Fatalf("❌ Veritabanı havuzu alınamadı: %v", err)
	}
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetMaxOpenConns(100)
	sqlDB.SetConnMaxLifetime(time.Hour)

	// 4. Veritabanı Tablolarını Senkronize Et (AutoMigrate)
	log.Println("🔄 Veritabanı tabloları senkronize ediliyor (AutoMigrate)...")
	err = db.AutoMigrate(
		&models.User{},
		&models.Group{},
		&models.GroupMember{},
		&models.GiftIdea{},
		&models.Vote{},
	)
	if err != nil {
		log.Fatalf("❌ Migrasyon hatası: %v", err)
	}
	log.Println("✅ 5 temel tablo ve ilişkiler başarıyla senkronize edildi!")

	// 5. Katmanları Başlat (Dependency Injection)
	userRepo := repository.NewUserRepository(db)
	groupRepo := repository.NewGroupRepository(db)
	ideaRepo := repository.NewIdeaRepository(db)

	// Gemini AI İstemcisi Başlatma (Fail-Safe)
	var geminiClient *gemini.Client
	if cfg.GeminiKey != "" {
		client, err := gemini.NewClient(cfg.GeminiKey, cfg.GeminiModel)
		if err != nil {
			log.Printf("⚠️  Gemini istemcisi başlatılamadı: %v", err)
		} else {
			geminiClient = client
			log.Printf("🤖 Gemini AI istemcisi aktif! (Model: %s)", cfg.GeminiModel)
		}
	} else {
		log.Println("⚠️  GEMINI_API_KEY tanımlı değil; AI hediye üretimi devre dışı.")
	}

	authService := services.NewAuthService(userRepo, cfg.JWTSecret)
	groupService := services.NewGroupService(groupRepo, userRepo)
	ideaService := services.NewIdeaService(ideaRepo, groupRepo, geminiClient)

	authController := controllers.NewAuthController(authService)
	groupController := controllers.NewGroupController(groupService)
	ideaController := controllers.NewIdeaController(ideaService)

	// 6. Gin web motorunu başlat
	r := gin.Default()

	// 7. Sağlık Kontrolü (Healthcheck) Endpoint'i
	r.GET("/health", func(c *gin.Context) {
		if sqlDB.Ping() != nil {
			controllers.RespondErr(c, http.StatusServiceUnavailable, "veritabanı erişilemez")
			return
		}

		controllers.RespondOK(c, gin.H{
			"status": "ok",
			"db":     "up",
		})
	})

	// 8. API v1 Rotaları
	apiV1 := r.Group("/api/v1")
	{
		// Açık Rotalar (Kimlik Doğrulama)
		authRoutes := apiV1.Group("/auth")
		{
			authRoutes.POST("/register", authController.Register)
			authRoutes.POST("/login", authController.Login)
		}

		// Korumalı Rotalar (JWT Token Zorunlu)
		protected := apiV1.Group("")
		protected.Use(middleware.Auth(cfg.JWTSecret))
		{
			protected.GET("/me", authController.Me)

			// Genel Grup İşlemleri
			protected.POST("/groups", groupController.CreateGroup)
			protected.GET("/groups", groupController.GetUserGroups)

			// Belirli Bir Gruba Ait Rotalar (Anti-Spoiler + Üyelik Denetimli)
			groupSpecific := protected.Group("/groups/:id")
			groupSpecific.Use(middleware.GroupAccess(groupRepo))
			{
				groupSpecific.GET("", groupController.GetGroupDetail)
				groupSpecific.POST("/members", groupController.AddMember)

				// Hediye Havuzu Rotaları
				groupSpecific.POST("/ideas", ideaController.AddIdea)
				groupSpecific.GET("/ideas", ideaController.GetIdeas)
				groupSpecific.POST("/ideas/preview-link", ideaController.PreviewLink)
				groupSpecific.POST("/ideas/generate-ai", ideaController.GenerateAI)
			}

			// Fikir ve Oylama Rotaları
			ideasGroup := protected.Group("/ideas/:id")
			{
				ideasGroup.POST("/vote", ideaController.Vote)
				ideasGroup.POST("/reject", ideaController.RejectIdea)
				ideasGroup.DELETE("", ideaController.DeleteIdea)
			}
		}
	}

	// 9. Sunucuyu ayağa kaldır
	log.Printf("🚀 CovertGift API :%s portunda çalışıyor", cfg.Port)
	if err := r.Run(":" + cfg.Port); err != nil {
		log.Fatalf("❌ Sunucu başlatılamadı: %v", err)
	}
}
