package controllers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// ResetConfirmPhrase, sıfırlama isteğinin gövdesinde açıkça yazılması gereken onay ifadesidir.
const ResetConfirmPhrase = "RESET"

// DevController geliştirme ve test ortamı yardımcı kontrolleri
type DevController struct {
	db         *gorm.DB
	appEnv     string
	allowReset bool
}

// NewDevController yeni bir DevController örneği oluşturur.
// allowReset yalnızca ALLOW_DB_RESET=true ile açılır; varsayılan olarak sıfırlama kapalıdır.
func NewDevController(db *gorm.DB, appEnv string, allowReset bool) *DevController {
	return &DevController{
		db:         db,
		appEnv:     appEnv,
		allowReset: allowReset,
	}
}

// ResetDBRequest sıfırlama onayını taşır
type ResetDBRequest struct {
	Confirm string `json:"confirm" example:"RESET"`
}

// ResetDB test ve demo ortamında veritabanındaki tüm verileri sıfırlar
// @Summary      Veritabanını Sıfırla (Reset Database)
// @Description  Tüm tabloları (users, groups, group_members, gift_ideas, votes) TRUNCATE CASCADE ile siler. Yalnızca API `ALLOW_DB_RESET=true` ile başlatıldıysa ve gövdede `{"confirm": "RESET"}` gönderilirse çalışır. Üretimde (`APP_ENV=production`) her durumda kapalıdır.
// @Tags         Sistem & Test
// @Accept       json
// @Produce      json
// @Param        request body ResetDBRequest true "Onay: confirm alanı tam olarak RESET olmalı"
// @Success      200 {object} Response{data=MessageData} "Veritabanı başarıyla sıfırlandı"
// @Failure      400 {object} Response "Onay ifadesi eksik veya hatalı"
// @Failure      403 {object} Response "Üretim ortamı ya da ALLOW_DB_RESET kapalı"
// @Failure      500 {object} Response "Veritabanı sıfırlanırken hata oluştu"
// @Router       /api/v1/dev/reset-db [post]
func (ctl *DevController) ResetDB(c *gin.Context) {
	if ctl.appEnv == "production" {
		RespondErr(c, http.StatusForbidden, "Veritabanı sıfırlama işlemi üretim (production) ortamında güvenlik nedeniyle devre dışıdır")
		return
	}
	if !ctl.allowReset {
		RespondErr(c, http.StatusForbidden, "Veritabanı sıfırlama kapalı; açmak için API'yi ALLOW_DB_RESET=true ile başlatın")
		return
	}

	var req ResetDBRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.Confirm != ResetConfirmPhrase {
		RespondErr(c, http.StatusBadRequest, `Onay gerekli: gövdede {"confirm": "RESET"} gönderin`)
		return
	}

	err := ctl.db.Transaction(func(tx *gorm.DB) error {
		// PostgreSQL'de tek bir atomik TRUNCATE CASCADE ile tüm tablolar temizlenir ve sequence'lar başa sarılır
		return tx.Exec("TRUNCATE TABLE votes, gift_ideas, group_members, groups, users RESTART IDENTITY CASCADE;").Error
	})

	if err != nil {
		RespondInternal(c, "Veritabanı sıfırlanırken hata oluştu", err)
		return
	}

	RespondOK(c, MessageData{Message: "Veritabanı başarıyla sıfırlandı (tüm test verileri temizlendi)"})
}
