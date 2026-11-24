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
//
// geminiClient arayüzdür: main *gemini.Client verir (anahtar yoksa tipli nil; istemci yöntemi
// ErrAPIKeyMissing döner, davranış öncekiyle aynıdır), entegrasyon testleri sahte üretici verebilir.
func newRouter(db *gorm.DB, cfg *config.Config, geminiClient services.GiftIdeaGenerator, hub, userHub *realtime.Hub) *gin.Engine {
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
	poolRepo := repository.NewPoolRepository(db)
	avatarRepo := repository.NewAvatarRepository(db)
	friendRepo := repository.NewFriendRepository(db)
	memberInviteRepo := repository.NewMemberInviteRepository(db)

	authService := services.NewAuthService(userRepo, cfg.JWTSecret)
	groupService := services.NewGroupService(groupRepo, userRepo, poolRepo)
	ideaService := services.NewIdeaService(ideaRepo, groupRepo, poolRepo, geminiClient)
	poolService := services.NewPoolService(groupRepo, poolRepo)
	inviteService := services.NewInviteService(inviteRepo, groupRepo, userRepo)
	avatarService := services.NewAvatarService(avatarRepo)
	notificationService := services.NewNotificationService(notificationRepo, groupRepo, poolRepo, userRepo, userHub)
	friendService := services.NewFriendService(friendRepo, memberInviteRepo, groupRepo, userRepo)

	authController := controllers.NewAuthController(authService)
	groupController := controllers.NewGroupController(groupService)
	groupController.Friends = friendService
	friendController := controllers.NewFriendController(friendService, userHub)
	ideaController := controllers.NewIdeaController(ideaService)
	inviteController := controllers.NewInviteController(inviteService)
	poolController := controllers.NewPoolController(poolService)
	avatarController := controllers.NewAvatarController(avatarService)
	eventsController := controllers.NewEventsController(hub, userHub)
	eventsController.PoolKeys = poolService.VisiblePoolIDs
	notificationController := controllers.NewNotificationController(notificationService)
	devController := controllers.NewDevController(db, cfg.AppEnv, cfg.AllowDBReset)
	if cfg.AllowDBReset && cfg.AppEnv != "production" {
		slog.Warn("ALLOW_DB_RESET=true: POST /api/v1/dev/reset-db açık ve tüm tabloları silebilir")
	}

	// Gin web motorunu başlat
	logger := slog.Default()
	r := gin.New()
	r.Use(middleware.Recovery(logger), middleware.RequestLogger(logger), middleware.BodyLimitWithRoutes(middleware.DefaultMaxBodyBytes, map[string]middleware.RouteLimit{
		// Yalnızca kapak ve profil fotoğrafı yüklemesi büyük gövde kabul eder; diğer tüm rotalar 1 MiB ile sınırlı kalır.
		"PUT /api/v1/groups/:id/cover": {MaxBytes: middleware.CoverUploadMaxBodyBytes, Message: imageproc.ErrTooLarge.Error()},
		"PUT /api/v1/me/avatar":        {MaxBytes: middleware.CoverUploadMaxBodyBytes, Message: imageproc.ErrTooLarge.Error()},
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
			protected.PUT("/me/avatar", avatarController.Upload)
			protected.DELETE("/me/avatar", avatarController.Delete)
			// Yetki serviste: sahibi ve onunla bir grubu paylaşanlar; diğerleri 404.
			protected.GET("/users/:id/avatar", avatarController.Get)

			// Bildirimler (kullanıcıya özel; Anti-Spoiler süzgeci sorguda)
			protected.GET("/notifications", notificationController.List)
			protected.GET("/notifications/stream", eventsController.NotificationStream)
			protected.POST("/notifications/read-all", notificationController.MarkAllRead)
			protected.POST("/notifications/:id/read", notificationController.MarkRead)

			// Genel Grup İşlemleri
			// Notify: member_ids ile davet edilen arkadaşlara "grup daveti" bildirimi.
			protected.POST("/groups", middleware.Notify(notificationService), groupController.CreateGroup)

			// Arkadaşlar (kullanıcıya özel; olaylar zil bildirimi üretmez, karşı tarafın akışına
			// yalnızca yeniden çekme sinyali gider)
			protected.GET("/friends", friendController.List)
			protected.POST("/friends/requests", friendController.Request)
			protected.POST("/friends/requests/:id/accept", friendController.AcceptRequest)
			protected.DELETE("/friends/requests/:id", friendController.DeleteRequest)
			protected.DELETE("/friends/users/:userId", friendController.Unfriend)
			protected.GET("/friends/link", friendController.GetLink)
			protected.POST("/friends/link/reset", friendController.ResetLink)
			protected.GET("/friend-links/:code", friendController.PreviewLink)
			protected.POST("/friend-links/:code/accept", friendController.AcceptLink)

			// Gelen grup davetleri (grup henüz bilinmediği için /groups/:id altında değildir;
			// Anti-Spoiler repository'de, SSE ve bildirim burada açıkça bağlanır)
			protected.GET("/me/group-invites", friendController.MyInvites)
			myInvites := protected.Group("/me/group-invites/:inviteId")
			myInvites.Use(middleware.PublishRecordedGroupChanges(hub), middleware.Notify(notificationService))
			{
				myInvites.POST("/accept", friendController.AcceptInvite)
				myInvites.DELETE("", friendController.DeclineInvite)
			}
			protected.GET("/groups", groupController.GetUserGroups)

			// Belirli Bir Gruba Ait Rotalar (Anti-Spoiler + Üyelik Denetimli)
			groupSpecific := protected.Group("/groups/:id")
			groupSpecific.Use(middleware.GroupAccess(groupRepo), middleware.PublishGroupChanges(hub), middleware.Notify(notificationService))
			// Yalnızca SINGLE (tek kişi için) gruplarda açık rotalar; MULTI grupta 409.
			single := middleware.RequireMode(models.GroupModeSingle)
			// Yalnızca MULTI (birden çok kişi için) gruplarda açık rotalar; SINGLE grupta 409.
			multi := middleware.RequireMode(models.GroupModeMulti)
			{
				groupSpecific.GET("", groupController.GetGroupDetail)
				groupSpecific.GET("/events", eventsController.Stream)
				groupSpecific.DELETE("", groupController.DeleteGroup)
				groupSpecific.POST("/members", groupController.AddMember)
				groupSpecific.POST("/interests", single, groupController.AddInterest)
				groupSpecific.DELETE("/interests", single, groupController.DeleteInterest)
				groupSpecific.PATCH("/budget", single, groupController.UpdateBudget)
				groupSpecific.PATCH("/target", single, groupController.LinkTarget)
				groupSpecific.PATCH("/event-date", single, groupController.UpdateEventDate)
				groupSpecific.PATCH("/description", groupController.UpdateDescription)
				groupSpecific.POST("/winner", single, groupController.SelectWinner)
				groupSpecific.DELETE("/winner", single, groupController.ReopenGroup)
				groupSpecific.GET("/cover", groupController.GetCover)
				groupSpecific.PUT("/cover", groupController.UploadCover)
				groupSpecific.DELETE("/cover", groupController.DeleteCover)
				groupSpecific.PATCH("/cover-policy", groupController.UpdateCoverPolicy)

				// Davet Linki (yalnızca admin; Anti-Spoiler GroupAccess'te)
				groupSpecific.GET("/invite", inviteController.GetStatus)
				groupSpecific.POST("/invite", inviteController.Create)
				groupSpecific.DELETE("/invite", inviteController.Revoke)

				// Arkadaş davetleri (listeleme tüm üyeler; davet/iptal yalnızca admin)
				groupSpecific.GET("/member-invites", friendController.ListGroupInvites)
				groupSpecific.POST("/member-invites", friendController.InviteToGroup)
				groupSpecific.DELETE("/member-invites/:userId", friendController.CancelGroupInvite)

				// Hediye Havuzu Rotaları (SINGLE). preview-link veritabanına yazmaz; iki türde de açık.
				groupSpecific.POST("/ideas", single, ideaController.AddIdea)
				groupSpecific.GET("/ideas", single, ideaController.GetIdeas)
				// Yazmaz: canlı yayın yapmaz (NoPublish), bildirim üretmez.
				groupSpecific.POST("/ideas/preview-link", middleware.NoPublish(), ideaController.PreviewLink)
				groupSpecific.POST("/ideas/generate-ai", single, ideaController.GenerateAI)

				// MULTI: alıcılar (yalnızca grup yöneticisi; PoolAccess'ten geçmez, böylece kendi
				// havuzunun alıcısı olan yönetici de çağırabilir) ve havuz listesi.
				groupSpecific.GET("/pools", multi, poolController.ListPools)
				groupSpecific.POST("/recipients", multi, poolController.AddRecipient)
				groupSpecific.PATCH("/recipients/:userId/manager", multi, poolController.SetRecipientManager)
				groupSpecific.DELETE("/recipients/:userId", multi, poolController.RemoveRecipient)

				// MULTI: tek bir havuza ait rotalar. PoolAccess gövde okunmadan önce çalışır; havuzun
				// alıcısı her rotada 403 + sürpriz mesajı alır. GroupAccess, PublishGroupChanges
				// (havuz anahtarıyla) ve Notify (pool_id ile) bu gruba da uygulanır.
				poolSpecific := groupSpecific.Group("/pools/:poolId")
				poolSpecific.Use(middleware.PoolAccess(poolRepo, poolService))
				{
					poolSpecific.GET("", poolController.GetPool)
					poolSpecific.GET("/ideas", ideaController.GetIdeas)
					poolSpecific.POST("/ideas", ideaController.AddIdea)
					poolSpecific.POST("/ideas/generate-ai", ideaController.GenerateAI)
					poolSpecific.POST("/interests", poolController.AddPoolInterest)
					poolSpecific.DELETE("/interests", poolController.DeletePoolInterest)
					poolSpecific.PATCH("/budget", poolController.UpdatePoolBudget)
					poolSpecific.PATCH("/event-date", poolController.UpdatePoolEventDate)
					poolSpecific.POST("/winner", poolController.SelectPoolWinner)
					poolSpecific.DELETE("/winner", poolController.ReopenPool)
					poolSpecific.PUT("/contribution", poolController.SetContribution)
				}
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
			// IdeaGuard gövde doğrulamasından önce Anti-Spoiler'ı denetler (hedef kişi / havuzun alıcısı).
			ideasGroup.Use(middleware.IdeaGuard(ideaService), middleware.PublishIdeaChanges(hub, ideaRepo), middleware.Notify(notificationService))
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
