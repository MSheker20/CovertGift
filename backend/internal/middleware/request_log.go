package middleware

import (
	"log/slog"
	"net/http"
	"regexp"
	"time"

	"github.com/MSheker20/CovertGift/backend/internal/controllers"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// RequestIDHeader her yanıtta dönen ve loglarla eşleştirmeye yarayan istek kimliğidir.
const RequestIDHeader = "X-Request-ID"

// RequestLogger her isteği tek bir yapılandırılmış satırla loglar: yöntem, yol, durum,
// süre ve istek kimliği. Sorgu dizesi, davet token'ı (yola rota kalıbı yazılır), istek gövdesi ve Authorization başlığı loglanmaz.
// 5xx yanıtlar ERROR seviyesinde, controller'ların c.Error ile eklediği iç hatayla
// birlikte yazılır; böylece istemciye genel mesaj dönerken ayrıntı logda kalır.
// /health istekleri yalnızca DEBUG seviyesinde yazılır (Azure sağlık yoklamaları).
func RequestLogger(logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		requestID := uuid.NewString()
		c.Header(RequestIDHeader, requestID)

		c.Next()

		status := c.Writer.Status()
		level := slog.LevelInfo
		switch {
		case status >= http.StatusInternalServerError:
			level = slog.LevelError
		case status >= http.StatusBadRequest:
			level = slog.LevelWarn
		case c.Request.URL.Path == "/health":
			level = slog.LevelDebug
		}
		path := loggablePath(c)
		attrs := []slog.Attr{
			slog.String("request_id", requestID),
			slog.String("method", c.Request.Method),
			slog.String("path", path),
			slog.String("route", c.FullPath()),
			slog.Int("status", status),
			slog.Float64("duration_ms", float64(time.Since(start).Microseconds())/1000),
			slog.String("client_ip", c.ClientIP()),
		}
		if len(c.Errors) > 0 {
			attrs = append(attrs, slog.String("error", c.Errors.String()))
		}
		logger.LogAttrs(c.Request.Context(), level, "http_request", attrs...)
	}
}

// Recovery panikleri yakalar, yığın izini loglar ve istemciye standart 500 zarfı döner.
func Recovery(logger *slog.Logger) gin.HandlerFunc {
	return gin.CustomRecoveryWithWriter(nil, func(c *gin.Context, recovered any) {
		logger.Error("panic", slog.Any("panic", recovered), slog.String("path", loggablePath(c)))
		controllers.RespondErr(c, http.StatusInternalServerError, "Beklenmeyen bir sunucu hatası oluştu")
		c.Abort()
	})
}

// inviteSegment davet token'ı taşıyan yol parçasını (API ve SPA) yakalar.
var inviteSegment = regexp.MustCompile(`(/invites?/)[^/?#]+`)

// loggablePath davet token'ı gibi yolda taşınan sırları loga yazmaz. Rota eşleştiyse ham yol
// yerine rota kalıbı yazılır (ör. /api/v1/invites/:token). Hiçbir rota eşleşmediyse
// (yanlış yöntem, eğik çizgi yönlendirmesi, bilinmeyen yol) rota kalıbı yoktur; bu durumda
// /invites/<parça> ve /invite/<parça> ham yolda :token ile değiştirilir.
func loggablePath(c *gin.Context) string {
	if c.Param("token") != "" {
		return c.FullPath()
	}
	path := c.Request.URL.Path
	if c.FullPath() == "" {
		return inviteSegment.ReplaceAllString(path, "${1}:token")
	}
	return path
}
