package config

import (
	"log"
	"os"

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
		GeminiModel:         getEnv("GEMINI_MODEL", "gemini-3.5-flash-lite"),
		GeminiResearchModel: getEnv("GEMINI_RESEARCH_MODEL", "gemini-3.8-flash"),
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
