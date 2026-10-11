package controllers

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Response standart API JSON yanıt şablonudur
type Response struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
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
