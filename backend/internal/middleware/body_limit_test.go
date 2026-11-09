package middleware

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestBodyLimit(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(BodyLimit(16))
	router.POST("/echo", func(c *gin.Context) {
		body, err := io.ReadAll(c.Request.Body)
		if err != nil {
			c.String(http.StatusBadRequest, "read failed")
			return
		}
		c.String(http.StatusOK, string(body))
	})

	send := func(body string, chunked bool) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/echo", strings.NewReader(body))
		if chunked {
			req.ContentLength = -1 // uzunluğu bildirilmeyen gövde
		}
		res := httptest.NewRecorder()
		router.ServeHTTP(res, req)
		return res
	}

	if res := send("small", false); res.Code != http.StatusOK || res.Body.String() != "small" {
		t.Fatalf("küçük gövde: HTTP %d %q", res.Code, res.Body)
	}
	if res := send(strings.Repeat("x", 16), false); res.Code != http.StatusOK {
		t.Fatalf("sınırdaki gövde reddedildi: HTTP %d", res.Code)
	}
	if res := send(strings.Repeat("x", 17), false); res.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("büyük gövde: HTTP %d, want 413", res.Code)
	}
	if res := send(strings.Repeat("x", 64), true); res.Code != http.StatusBadRequest {
		t.Fatalf("uzunluğu bildirilmeyen büyük gövde okunabildi: HTTP %d", res.Code)
	}
}
