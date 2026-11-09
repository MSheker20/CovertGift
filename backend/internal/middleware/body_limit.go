package middleware

import (
	"net/http"

	"github.com/MSheker20/CovertGift/backend/internal/controllers"
	"github.com/gin-gonic/gin"
)

// DefaultMaxBodyBytes API'nin kabul ettiği en büyük istek gövdesidir (1 MiB).
// En büyük meşru istek, link önizlemesiyle doldurulan bir hediye önerisidir ve
// birkaç KB'ı geçmez.
const DefaultMaxBodyBytes int64 = 1 << 20

// BodyLimit istek gövdesini maxBytes ile sınırlar. Content-Length sınırı aşıyorsa
// istek okunmadan 413 döner; uzunluğu bildirilmeyen (chunked) gövdeler okunurken
// kesilir ve JSON ayrıştırması hata verir.
func BodyLimit(maxBytes int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.ContentLength > maxBytes {
			controllers.RespondErr(c, http.StatusRequestEntityTooLarge, "İstek gövdesi çok büyük")
			c.Abort()
			return
		}
		if c.Request.Body != nil {
			c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxBytes)
		}
		c.Next()
	}
}
