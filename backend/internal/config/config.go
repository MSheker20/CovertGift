package config

import (
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

// Config projenin tüm ayarlarını tutan merkezi yapıdır
type Config struct {
	Port                string
	DatabaseURL         string
	JWTSecret           string
	GeminiKey           string
	GeminiModel         string
	GeminiResearchModel string
	GeminiFallbackModel string
	GeminiSearchEnabled bool
	GeminiThinkingLevel string
	CORSOrigin          string
	AppEnv              string
}

// Load .env dosyasını okur ve Config yapısını doldurur
func Load() *Config {
	// .env dosyasını hafızaya yükle
	if err := godotenv.Load(); err != nil {
		log.Println("⚠️  .env dosyası bulunamadı, sistem ortam değişkenleri kullanılacak")
	}

	cfg := &Config{
		Port:                getEnv("PORT", "8080"),
		DatabaseURL:         os.Getenv("DATABASE_URL"),
		JWTSecret:           os.Getenv("JWT_SECRET"),
		GeminiKey:           os.Getenv("GEMINI_API_KEY"),
		GeminiModel:         strings.TrimSpace(os.Getenv("GEMINI_MODEL")),
		GeminiResearchModel: strings.TrimSpace(os.Getenv("GEMINI_RESEARCH_MODEL")),
		GeminiFallbackModel: strings.TrimSpace(os.Getenv("GEMINI_FALLBACK_MODEL")),
		GeminiSearchEnabled: searchEnabled(),
		GeminiThinkingLevel: strings.TrimSpace(os.Getenv("GEMINI_THINKING_LEVEL")),
		CORSOrigin:          getEnv("CORS_ORIGIN", "http://localhost:5173"),
		AppEnv:              getEnv("APP_ENV", "development"),
	}

	// Kritik değişkenler yoksa sistemi hiç başlatma (Fail-Fast)
	if cfg.DatabaseURL == "" || cfg.JWTSecret == "" {
		log.Fatal("❌ HATA: DATABASE_URL ve JWT_SECRET ortam değişkenleri zorunludur!")
	}

	return cfg
}

// getEnv değişken tanımlı değilse varsayılan (fallback) değeri döner
func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func searchEnabled() bool {
	raw := strings.TrimSpace(os.Getenv("GEMINI_SEARCH_ENABLED"))
	if raw == "" {
		return false
	}
	enabled, err := strconv.ParseBool(raw)
	if err != nil {
		log.Fatal("GEMINI_SEARCH_ENABLED true veya false olmalıdır")
	}
	return enabled
}
