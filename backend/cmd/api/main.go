package main

import (
	"database/sql"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/MSheker20/CovertGift/backend/internal/config"
	"github.com/MSheker20/CovertGift/backend/internal/controllers"
	"github.com/MSheker20/CovertGift/backend/internal/logging"
	"github.com/MSheker20/CovertGift/backend/internal/realtime"
	"github.com/MSheker20/CovertGift/backend/pkg/gemini"
	"github.com/gin-gonic/gin"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	_ "github.com/MSheker20/CovertGift/backend/docs"
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
	// 1. Ortam ayarlarını yükle (.env) ve yapılandırılmış loglamayı kur
	config.LoadDotEnv()
	logging.FromEnv()
	cfg := config.Load()

	// 2. PostgreSQL veritabanına bağlan
	db, err := gorm.Open(postgres.Open(cfg.DatabaseURL), &gorm.Config{})
	if err != nil {
		fatal("veritabanına bağlanılamadı", err)
	}

	// 3. Bağlantı Havuzu (Connection Pool) Ayarları
	sqlDB, err := db.DB()
	if err != nil {
		fatal("veritabanı havuzu alınamadı", err)
	}
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetMaxOpenConns(100)
	sqlDB.SetConnMaxLifetime(time.Hour)

	// 4. Veritabanı Tablolarını Senkronize Et (AutoMigrate)
	err = migrate(db)
	if err != nil {
		fatal("migrasyon hatası", err)
	}
	slog.Info("veritabanı tabloları senkronize edildi")

	// Gemini AI İstemcisi Başlatma (Fail-Safe)
	var geminiClient *gemini.Client
	if cfg.GeminiKey != "" {
		client, err := gemini.NewClientWithOptions(cfg.GeminiKey, gemini.Options{Model: cfg.GeminiModel, ResearchModel: cfg.GeminiResearchModel, FallbackModel: cfg.GeminiFallbackModel, SearchEnabled: cfg.GeminiSearchEnabled, ThinkingLevel: cfg.GeminiThinkingLevel})
		if err != nil {
			fatal("Gemini yapılandırması geçersiz", err)
		} else {
			geminiClient = client
			slog.Info("Gemini aktif", "model", cfg.GeminiModel, "search", cfg.GeminiSearchEnabled, "research_model", cfg.GeminiResearchModel, "fallback_model", cfg.GeminiFallbackModel)
		}
	} else {
		slog.Warn("GEMINI_API_KEY tanımlı değil; AI hediye üretimi devre dışı")
	}

	r := newRouter(db, cfg, geminiClient, realtime.NewHub(realtimeMaxStreams))

	// 5. Katmanları, rotaları kur ve sunucuyu ayağa kaldır
	slog.Info("CovertGift API çalışıyor", "port", cfg.Port, "env", cfg.AppEnv)
	if err := r.Run(":" + cfg.Port); err != nil {
		fatal("sunucu başlatılamadı", err)
	}
}

// realtimeMaxStreams aynı anda açık SSE bağlantısı sınırıdır (kaynak tükenmesine karşı).
const realtimeMaxStreams = 2000

// fatal hatayı ERROR seviyesinde loglar ve süreci sonlandırır.
func fatal(msg string, err error) {
	slog.Error(msg, "error", err)
	os.Exit(1)
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
