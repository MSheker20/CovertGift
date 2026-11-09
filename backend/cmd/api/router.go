package main

import (
	"log"
	"net/http"

	"github.com/MSheker20/CovertGift/backend/internal/config"
	"github.com/MSheker20/CovertGift/backend/internal/controllers"
	"github.com/MSheker20/CovertGift/backend/internal/middleware"
	"github.com/MSheker20/CovertGift/backend/internal/models"
	"github.com/MSheker20/CovertGift/backend/internal/repository"
	"github.com/MSheker20/CovertGift/backend/internal/services"
	"github.com/MSheker20/CovertGift/backend/pkg/gemini"
	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"gorm.io/gorm"
)

// migrate 5 temel tabloyu ve ilişkilerini AutoMigrate ile senkronize eder.
func migrate(db *gorm.DB) error {
	return db.AutoMigrate(
		&models.User{},
		&models.Group{},
		&models.GroupMember{},
		&models.GiftIdea{},
		&models.Vote{},
	)
}

// newRouter katmanları bağlar ve tüm rotaları kurar. main ve entegrasyon testleri
// aynı kurulumu kullanır; böylece testler gerçek middleware zincirini dener.
// geminiClient nil olabilir (AI üretimi devre dışı).
func newRouter(db *gorm.DB, cfg *config.Config, geminiClient *gemini.Client) *gin.Engine {
	sqlDB, err := db.DB()
	if err != nil {
		log.Fatalf("❌ Veritabanı havuzu alınamadı: %v", err)
	}

	// Katmanları Başlat (Dependency Injection)
	userRepo := repository.NewUserRepository(db)
	groupRepo := repository.NewGroupRepository(db)
	ideaRepo := repository.NewIdeaRepository(db)

	authService := services.NewAuthService(userRepo, cfg.JWTSecret)
	groupService := services.NewGroupService(groupRepo, userRepo)
	ideaService := services.NewIdeaService(ideaRepo, groupRepo, geminiClient)

	authController := controllers.NewAuthController(authService)
	groupController := controllers.NewGroupController(groupService)
	ideaController := controllers.NewIdeaController(ideaService)
	devController := controllers.NewDevController(db, cfg.AppEnv, cfg.AllowDBReset)
	if cfg.AllowDBReset && cfg.AppEnv != "production" {
		log.Println("⚠️  ALLOW_DB_RESET=true: POST /api/v1/dev/reset-db açık ve tüm tabloları silebilir")
	}

	// Gin web motorunu başlat
	r := gin.Default()
	r.Use(middleware.BodyLimit(middleware.DefaultMaxBodyBytes))

	// Sağlık Kontrolü (Healthcheck) Endpoint'i
	r.GET("/health", healthHandler(sqlDB))

	// Swagger Dokümantasyon UI Rotası
	r.GET("/swagger", func(c *gin.Context) {
		c.Redirect(http.StatusMovedPermanently, "/swagger/index.html")
	})
	r.GET("/docs", func(c *gin.Context) {
		c.Redirect(http.StatusMovedPermanently, "/swagger/index.html")
	})
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	// API v1 Rotaları
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
				groupSpecific.PATCH("/target", groupController.LinkTarget)

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

	return r
}
