package controllers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// DevController geliştirme ve test ortamı yardımcı kontrolleri
type DevController struct {
	db     *gorm.DB
	appEnv string
}

// NewDevController yeni bir DevController örneği oluşturur
func NewDevController(db *gorm.DB, appEnv string) *DevController {
	return &DevController{
		db:     db,
		appEnv: appEnv,
	}
}

// ResetDB test ve demo ortamında veritabanındaki tüm verileri sıfırlar
// @Summary      Veritabanını Sıfırla (Reset Database)
// @Description  Test, geliştirme ve canlı sunum ortamında tüm tabloları (users, groups, group_members, gift_ideas, votes) sıfırlar (TRUNCATE CASCADE) ve veritabanını fabrika ayarlarına temiz olarak döndürür.
// @Tags         Sistem & Test
// @Produce      json
// @Success      200 {object} Response{data=MessageData} "Veritabanı başarıyla sıfırlandı"
// @Failure      403 {object} Response "Üretim (Production) ortamında bu işlem yapılamaz"
// @Failure      500 {object} Response "Veritabanı sıfırlanırken hata oluştu"
// @Router       /api/v1/dev/reset-db [post]
func (ctl *DevController) ResetDB(c *gin.Context) {
	if ctl.appEnv == "production" {
		RespondErr(c, http.StatusForbidden, "Veritabanı sıfırlama işlemi üretim (production) ortamında güvenlik nedeniyle devre dışıdır")
		return
	}

	err := ctl.db.Transaction(func(tx *gorm.DB) error {
		// PostgreSQL'de tek bir atomik TRUNCATE CASCADE ile tüm tablolar temizlenir ve sequence'lar başa sarılır
		return tx.Exec("TRUNCATE TABLE votes, gift_ideas, group_members, groups, users RESTART IDENTITY CASCADE;").Error
	})

	if err != nil {
		RespondErr(c, http.StatusInternalServerError, "Veritabanı sıfırlanırken hata oluştu: "+err.Error())
		return
	}

	RespondOK(c, MessageData{Message: "Veritabanı başarıyla sıfırlandı (tüm test verileri temizlendi)"})
}
