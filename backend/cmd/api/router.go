package main

import (
	"log/slog"
	"net/http"
	"os"

	"github.com/MSheker20/CovertGift/backend/internal/config"
	"github.com/MSheker20/CovertGift/backend/internal/controllers"
	"github.com/MSheker20/CovertGift/backend/internal/middleware"
	"github.com/MSheker20/CovertGift/backend/internal/models"
	"github.com/MSheker20/CovertGift/backend/internal/realtime"
	"github.com/MSheker20/CovertGift/backend/internal/repository"
	"github.com/MSheker20/CovertGift/backend/internal/services"
	"github.com/MSheker20/CovertGift/backend/pkg/gemini"
	"github.com/MSheker20/CovertGift/backend/pkg/imageproc"
	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"gorm.io/gorm"
)

// migrate şemayı models.Migrate ile senkronize eder.
func migrate(db *gorm.DB) error {
	return models.Migrate(db)
}

// newRouter katmanları bağlar ve tüm rotaları kurar. main ve entegrasyon testleri
// aynı kurulumu kullanır; böylece testler gerçek middleware zincirini dener.
// geminiClient nil olabilir (AI üretimi devre dışı).
// hub grup sayfası akışlarını (grup kimliğiyle), userHub bildirim akışlarını (kullanıcı
// kimliğiyle) taşır.
func newRouter(db *gorm.DB, cfg *config.Config, geminiClient *gemini.Client, hub, userHub *realtime.Hub) *gin.Engine {
	sqlDB, err := db.DB()
	if err != nil {
		slog.Error("veritabanı havuzu alınamadı", "error", err)
		os.Exit(1)
	}

	// Katmanları Başlat (Dependency Injection)
	userRepo := repository.NewUserRepository(db)
	groupRepo := repository.NewGroupRepository(db)
	ideaRepo := repository.NewIdeaRepository(db)
	notificationRepo := repository.NewNotificationRepository(db)
	inviteRepo := repository.NewInviteRepository(db)

	authService := services.NewAuthService(userRepo, cfg.JWTSecret)
	groupService := services.NewGroupService(groupRepo, userRepo)
	ideaService := services.NewIdeaService(ideaRepo, groupRepo, geminiClient)
	inviteService := services.NewInviteService(inviteRepo, groupRepo, userRepo)
	notificationService := services.NewNotificationService(notificationRepo, groupRepo, userRepo, userHub)

	authController := controllers.NewAuthController(authService)
	groupController := controllers.NewGroupController(groupService)
	ideaController := controllers.NewIdeaController(ideaService)
	inviteController := controllers.NewInviteController(inviteService)
	eventsController := controllers.NewEventsController(hub, userHub)
	notificationController := controllers.NewNotificationController(notificationService)
	devController := controllers.NewDevController(db, cfg.AppEnv, cfg.AllowDBReset)
	if cfg.AllowDBReset && cfg.AppEnv != "production" {
		slog.Warn("ALLOW_DB_RESET=true: POST /api/v1/dev/reset-db açık ve tüm tabloları silebilir")
	}

	// Gin web motorunu başlat
	logger := slog.Default()
	r := gin.New()
	r.Use(middleware.Recovery(logger), middleware.RequestLogger(logger), middleware.BodyLimitWithRoutes(middleware.DefaultMaxBodyBytes, map[string]middleware.RouteLimit{
		// Yalnızca kapak yüklemesi büyük gövde kabul eder; diğer tüm rotalar 1 MiB ile sınırlı kalır.
		"PUT /api/v1/groups/:id/cover": {MaxBytes: middleware.CoverUploadMaxBodyBytes, Message: imageproc.ErrTooLarge.Error()},
	}))

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
			protected.PATCH("/me", authController.UpdateMe)
			protected.PUT("/me/password", authController.ChangePassword)
			protected.DELETE("/me", authController.DeleteMe)

			// Bildirimler (kullanıcıya özel; Anti-Spoiler süzgeci sorguda)
			protected.GET("/notifications", notificationController.List)
			protected.GET("/notifications/stream", eventsController.NotificationStream)
			protected.POST("/notifications/read-all", notificationController.MarkAllRead)
			protected.POST("/notifications/:id/read", notificationController.MarkRead)

			// Genel Grup İşlemleri
			protected.POST("/groups", groupController.CreateGroup)
			protected.GET("/groups", groupController.GetUserGroups)

			// Belirli Bir Gruba Ait Rotalar (Anti-Spoiler + Üyelik Denetimli)
			groupSpecific := protected.Group("/groups/:id")
			groupSpecific.Use(middleware.GroupAccess(groupRepo), middleware.PublishGroupChanges(hub), middleware.Notify(notificationService))
			{
				groupSpecific.GET("", groupController.GetGroupDetail)
				groupSpecific.GET("/events", eventsController.Stream)
				groupSpecific.DELETE("", groupController.DeleteGroup)
				groupSpecific.POST("/members", groupController.AddMember)
				groupSpecific.POST("/interests", groupController.AddInterest)
				groupSpecific.DELETE("/interests", groupController.DeleteInterest)
				groupSpecific.PATCH("/budget", groupController.UpdateBudget)
				groupSpecific.PATCH("/target", groupController.LinkTarget)
				groupSpecific.PATCH("/event-date", groupController.UpdateEventDate)
				groupSpecific.PATCH("/description", groupController.UpdateDescription)
				groupSpecific.POST("/winner", groupController.SelectWinner)
				groupSpecific.DELETE("/winner", groupController.ReopenGroup)
				groupSpecific.GET("/cover", groupController.GetCover)
				groupSpecific.PUT("/cover", groupController.UploadCover)
				groupSpecific.DELETE("/cover", groupController.DeleteCover)
				groupSpecific.PATCH("/cover-policy", groupController.UpdateCoverPolicy)

				// Davet Linki (yalnızca admin; Anti-Spoiler GroupAccess'te)
				groupSpecific.GET("/invite", inviteController.GetStatus)
				groupSpecific.POST("/invite", inviteController.Create)
				groupSpecific.DELETE("/invite", inviteController.Revoke)

				// Hediye Havuzu Rotaları
				groupSpecific.POST("/ideas", ideaController.AddIdea)
				groupSpecific.GET("/ideas", ideaController.GetIdeas)
				groupSpecific.POST("/ideas/preview-link", ideaController.PreviewLink)
				groupSpecific.POST("/ideas/generate-ai", ideaController.GenerateAI)
			}

			// Davetle Katılma (grup henüz bilinmediği için /groups/:id altında değildir; bildirim ve
			// SSE yayını burada açıkça bağlanır, Anti-Spoiler serviste denetlenir)
			invites := protected.Group("/invites/:token")
			invites.Use(middleware.PublishRecordedGroupChanges(hub), middleware.Notify(notificationService))
			{
				invites.GET("", inviteController.Preview)
				invites.POST("/accept", inviteController.Accept)
			}

			// Fikir ve Oylama Rotaları
			ideasGroup := protected.Group("/ideas/:id")
			ideasGroup.Use(middleware.PublishIdeaChanges(hub, ideaRepo), middleware.Notify(notificationService))
			{
				ideasGroup.POST("/vote", ideaController.Vote)
				ideasGroup.DELETE("/vote", ideaController.Unvote)
				ideasGroup.POST("/reject", ideaController.RejectIdea)
				ideasGroup.DELETE("", ideaController.DeleteIdea)
			}
		}
	}

	return r
}
