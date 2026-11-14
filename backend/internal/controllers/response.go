package controllers

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Response standart API JSON yanıt şablonudur
type Response struct {
	Success bool        `json:"success" example:"true"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty" example:"Hata mesajı"`
}

// MessageData işlem sonuç mesajı taşıyan şablondur
type MessageData struct {
	Message string `json:"message" example:"İşlem başarıyla tamamlandı"`
}

// RespondOK standart başarılı (200 OK) JSON yanıtı döner
func RespondOK(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, Response{
		Success: true,
		Data:    data,
	})
}

// RespondCreated standart oluşturuldu (201 Created) JSON yanıtı döner
func RespondCreated(c *gin.Context, data interface{}) {
	c.JSON(http.StatusCreated, Response{
		Success: true,
		Data:    data,
	})
}

// RespondErr standart hata JSON yanıtı döner
func RespondErr(c *gin.Context, status int, msg string) {
	c.JSON(status, Response{
		Success: false,
		Error:   msg,
	})
}

// RespondInternal 500 yanıtı döner. İç hata istemciye gösterilmez; c.Error ile isteğe
// eklenir ve RequestLogger tarafından ERROR seviyesinde loglanır.
func RespondInternal(c *gin.Context, msg string, err error) {
	if err != nil {
		_ = c.Error(err)
	}
	RespondErr(c, http.StatusInternalServerError, msg)
}

// NoRealtimePublishKey başarılı ama hiçbir şey değiştirmeyen bir isteği canlı yayından çıkaran
// gin context anahtarıdır (middleware.PublishGroupChanges okur).
const NoRealtimePublishKey = "realtimeNoPublish"

// SkipRealtimePublish handler'ın isteğin hiçbir şey değiştirmediğini bildirmesini sağlar; böylece
// aboneler görünür değişikliği olmayan bir "changed" olayı almaz.
func SkipRealtimePublish(c *gin.Context) {
	c.Set(NoRealtimePublishKey, true)
}
