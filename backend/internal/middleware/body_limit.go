package middleware

import (
	"net/http"

	"github.com/MSheker20/CovertGift/backend/internal/controllers"
	"github.com/MSheker20/CovertGift/backend/pkg/imageproc"
	"github.com/gin-gonic/gin"
)

// DefaultMaxBodyBytes API'nin kabul ettiği en büyük istek gövdesidir (1 MiB).
// Kapak fotoğrafı yüklemesi dışındaki en büyük meşru istek, link önizlemesiyle doldurulan
// bir hediye önerisidir ve birkaç KB'ı geçmez.
const DefaultMaxBodyBytes int64 = 1 << 20

// CoverUploadMaxBodyBytes kapak fotoğrafı yüklemesinin gövde sınırıdır: en büyük dosya
// (imageproc.MaxInputBytes, 2 MiB) ve multipart zarfı için 64 KiB pay. Fotoğrafı tarayıcı
// küçültür; bundan büyük gövde okunmadan 413 alır.
const CoverUploadMaxBodyBytes int64 = imageproc.MaxInputBytes + 64<<10

// RouteLimit tek bir rotanın gövde sınırı ve aşıldığında dönen mesajdır.
type RouteLimit struct {
	MaxBytes int64
	Message  string
}

// BodyLimit istek gövdesini maxBytes ile sınırlar. Content-Length sınırı aşıyorsa
// istek okunmadan 413 döner; uzunluğu bildirilmeyen (chunked) gövdeler okunurken
// kesilir ve JSON ayrıştırması hata verir.
func BodyLimit(maxBytes int64) gin.HandlerFunc {
	return BodyLimitWithRoutes(maxBytes, nil)
}

// BodyLimitWithRoutes BodyLimit gibidir; ayrıca "METOT /rota/kalıbı" (gin FullPath)
// anahtarıyla verilen rotalara kendi sınırını uygular. Listede olmayan her rota
// defaultMax ile sınırlı kalır.
func BodyLimitWithRoutes(defaultMax int64, routes map[string]RouteLimit) gin.HandlerFunc {
	return func(c *gin.Context) {
		maxBytes, message := defaultMax, "İstek gövdesi çok büyük"
		if limit, ok := routes[c.Request.Method+" "+c.FullPath()]; ok {
			maxBytes = limit.MaxBytes
			if limit.Message != "" {
				message = limit.Message
			}
		}
		if c.Request.ContentLength > maxBytes {
			controllers.RespondErr(c, http.StatusRequestEntityTooLarge, message)
			c.Abort()
			return
		}
		if c.Request.Body != nil {
			c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxBytes)
		}
		c.Next()
	}
}
