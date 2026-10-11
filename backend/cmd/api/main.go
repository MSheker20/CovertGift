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

	authService := services.NewAuthService(userRepo, cfg.JWTSecret)
	groupService := services.NewGroupService(groupRepo, userRepo)

	authController := controllers.NewAuthController(authService)
	groupController := controllers.NewGroupController(groupService)

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
			}
		}
	}

	// 9. Sunucuyu ayağa kaldır
	log.Printf("🚀 CovertGift API :%s portunda çalışıyor", cfg.Port)
	if err := r.Run(":" + cfg.Port); err != nil {
		log.Fatalf("❌ Sunucu başlatılamadı: %v", err)
	}
}
