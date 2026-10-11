package main

import (
	"log"
	"net/http"
	"time"

	"github.com/MSheker20/CovertGift/backend/internal/config"
	"github.com/MSheker20/CovertGift/backend/internal/models"
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

	// 3. Bağlantı Havuzu (Connection Pool) Ayarları
	sqlDB, err := db.DB()
	if err != nil {
		log.Fatalf("❌ Veritabanı havuzu alınamadı: %v", err)
	}
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetMaxOpenConns(100)
	sqlDB.SetConnMaxLifetime(time.Hour)

	// 4. Veritabanı Migrasyonu (AutoMigrate)
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

	// 5. Gin web motorunu başlat
	r := gin.Default()

	// 6. Sağlık kontrolü (Healthcheck) endpoint'i
	r.GET("/health", func(c *gin.Context) {
		if sqlDB.Ping() != nil {
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

	// 7. Sunucuyu ayağa kaldır
	log.Printf("🚀 CovertGift API :%s portunda çalışıyor", cfg.Port)
	if err := r.Run(":" + cfg.Port); err != nil {
		log.Fatalf("❌ Sunucu başlatılamadı: %v", err)
	}
}
