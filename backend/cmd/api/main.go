package main

import (
	"database/sql"
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

	_ "github.com/MSheker20/CovertGift/backend/docs"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

// @title           CovertGift API
// @version         1.0
// @description     Arkadaş gruplarının ortak hediye organize ettiği, bütçeyi birleştirdiği, Gemini 3 Flash ile yapay zekâ destekli semantik hediye önerileri aldığı, önerileri oyladığı (+1 / -1) ve hediye alınacak kişiden süreci gizlediği (Anti-Spoiler) işbirlikçi web platformu REST API.
// @termsOfService  http://swagger.io/terms/

// @contact.name    CovertGift Geliştirici Ekibi
// @contact.url     https://github.com/MSheker20/CovertGift

// @license.name    MIT
// @license.url     https://opensource.org/licenses/MIT

// @host            localhost:8080
// @BasePath        /

// @securityDefinitions.apikey BearerAuth
// @in              header
// @name            Authorization
// @description     JWT Bearer token formatı: "Bearer {token}". Örnek: "Bearer eyJhbGciOiJIUzI1Ni..."

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
	devController := controllers.NewDevController(db, cfg.AppEnv)

	// 6. Gin web motorunu başlat
	r := gin.Default()

	// 7. Sağlık Kontrolü (Healthcheck) Endpoint'i
	r.GET("/health", healthHandler(sqlDB))

	// Swagger Dokümantasyon UI Rotası
	r.GET("/swagger", func(c *gin.Context) {
		c.Redirect(http.StatusMovedPermanently, "/swagger/index.html")
	})
	r.GET("/docs", func(c *gin.Context) {
		c.Redirect(http.StatusMovedPermanently, "/swagger/index.html")
	})
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	// 8. API v1 Rotaları
	apiV1 := r.Group("/api/v1")
	{
		// Geliştirici & Test Yardımcı Rotaları
		apiV1.POST("/dev/reset-db", devController.ResetDB)

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
			protected.DELETE("/me", authController.DeleteMe)

			// Genel Grup İşlemleri
			protected.POST("/groups", groupController.CreateGroup)
			protected.GET("/groups", groupController.GetUserGroups)

			// Belirli Bir Gruba Ait Rotalar (Anti-Spoiler + Üyelik Denetimli)
			groupSpecific := protected.Group("/groups/:id")
			groupSpecific.Use(middleware.GroupAccess(groupRepo))
			{
				groupSpecific.GET("", groupController.GetGroupDetail)
				groupSpecific.DELETE("", groupController.DeleteGroup)
				groupSpecific.POST("/members", groupController.AddMember)
				groupSpecific.POST("/interests", groupController.AddInterest)
				groupSpecific.DELETE("/interests", groupController.DeleteInterest)
				groupSpecific.PATCH("/budget", groupController.UpdateBudget)

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

// HealthResponse sağlık kontrolü yanıt verisi
type HealthResponse struct {
	DB     string `json:"db" example:"up"`
	Status string `json:"status" example:"ok"`
}

// healthHandler sistem sağlık kontrolü handler'ı
// @Summary      Sistem ve Veritabanı Sağlık Kontrolü
// @Description  API sunucusunun ve PostgreSQL veritabanı bağlantı havuzunun durumunu kontrol eder.
// @Tags         Sistem & Sağlık
// @Produce      json
// @Success      200 {object} controllers.Response{data=HealthResponse} "Sistem ve veritabanı çalışıyor"
// @Failure      503 {object} controllers.Response "Veritabanına erişilemiyor"
// @Router       /health [get]
func healthHandler(sqlDB *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		if sqlDB.Ping() != nil {
			controllers.RespondErr(c, http.StatusServiceUnavailable, "veritabanı erişilemez")
			return
		}

		controllers.RespondOK(c, HealthResponse{
			DB:     "up",
			Status: "ok",
		})
	}
}
