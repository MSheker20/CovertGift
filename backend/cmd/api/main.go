package main

import (
	"log"
	"net/http"

	"github.com/MSheker20/CovertGift/backend/internal/config"
	"github.com/gin-gonic/gin"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func main() {
	// 1. Ayarları yükle (.env)
	cfg := config.Load()

	// 2. PostgreSQL'e bağlan
	db, err := gorm.Open(postgres.Open(cfg.DatabaseURL), &gorm.Config{})
	if err != nil {
		log.Fatalf("❌ Veritabanına bağlanılamadı: %v", err)
	}

	// 3. Gin web motorunu başlat
	r := gin.Default()

	// 4. Sağlık kontrolü (Healthcheck) endpoint'i
	r.GET("/health", func(c *gin.Context) {
		sqlDB, err := db.DB()
		if err != nil || sqlDB.Ping() != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"success": false,
				"error":   "veritabanı erişilemez",
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"data": gin.H{
				"status": "ok",
				"db":     "up",
			},
		})
	})

	// 5. Sunucuyu ayağa kaldır
	log.Printf("🚀 CovertGift API :%s portunda çalışıyor", cfg.Port)
	if err := r.Run(":" + cfg.Port); err != nil {
		log.Fatalf("❌ Sunucu başlatılamadı: %v", err)
	}
}
