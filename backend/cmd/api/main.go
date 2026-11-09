package main

import (
	"database/sql"
	"log"
	"net/http"
	"time"

	"github.com/MSheker20/CovertGift/backend/internal/config"
	"github.com/MSheker20/CovertGift/backend/internal/controllers"
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
	err = migrate(db)
	if err != nil {
		log.Fatalf("❌ Migrasyon hatası: %v", err)
	}
	log.Println("✅ 5 temel tablo ve ilişkiler başarıyla senkronize edildi!")

	// Gemini AI İstemcisi Başlatma (Fail-Safe)
	var geminiClient *gemini.Client
	if cfg.GeminiKey != "" {
		client, err := gemini.NewClientWithOptions(cfg.GeminiKey, gemini.Options{Model: cfg.GeminiModel, ResearchModel: cfg.GeminiResearchModel, FallbackModel: cfg.GeminiFallbackModel, SearchEnabled: cfg.GeminiSearchEnabled, ThinkingLevel: cfg.GeminiThinkingLevel})
		if err != nil {
			log.Fatalf("Gemini yapılandırması geçersiz: %v", err)
		} else {
			geminiClient = client
			log.Printf("Gemini aktif: model=%s search=%t research=%s fallback=%s", cfg.GeminiModel, cfg.GeminiSearchEnabled, cfg.GeminiResearchModel, cfg.GeminiFallbackModel)
		}
	} else {
		log.Println("⚠️  GEMINI_API_KEY tanımlı değil; AI hediye üretimi devre dışı.")
	}

	r := newRouter(db, cfg, geminiClient)

	// 5. Katmanları, rotaları kur ve sunucuyu ayağa kaldır
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
