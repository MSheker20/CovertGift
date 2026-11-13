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

func TestBodyLimitWithRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(BodyLimitWithRoutes(16, map[string]RouteLimit{
		"PUT /groups/:id/cover": {MaxBytes: 64, Message: "görsel çok büyük"},
	}))
	echo := func(c *gin.Context) {
		body, err := io.ReadAll(c.Request.Body)
		if err != nil {
			c.String(http.StatusBadRequest, "read failed")
			return
		}
		c.String(http.StatusOK, "%d", len(body))
	}
	router.PUT("/groups/:id/cover", echo)
	router.POST("/groups/:id/cover", echo)
	router.PUT("/groups/:id/other", echo)

	send := func(method, path string, size int) *httptest.ResponseRecorder {
		res := httptest.NewRecorder()
		router.ServeHTTP(res, httptest.NewRequest(method, path, strings.NewReader(strings.Repeat("x", size))))
		return res
	}

	if res := send(http.MethodPut, "/groups/42/cover", 64); res.Code != http.StatusOK || res.Body.String() != "64" {
		t.Fatalf("özel sınırdaki gövde: HTTP %d %q", res.Code, res.Body)
	}
	if res := send(http.MethodPut, "/groups/42/cover", 65); res.Code != http.StatusRequestEntityTooLarge || !strings.Contains(res.Body.String(), "görsel çok büyük") {
		t.Fatalf("özel sınırı aşan gövde: HTTP %d %q", res.Code, res.Body)
	}
	// Aynı yolun başka metodu ve başka rotalar varsayılan sınırda kalır.
	for _, r := range [][2]string{{http.MethodPost, "/groups/42/cover"}, {http.MethodPut, "/groups/42/other"}, {http.MethodPut, "/yok"}} {
		if res := send(r[0], r[1], 17); res.Code != http.StatusRequestEntityTooLarge || !strings.Contains(res.Body.String(), "İstek gövdesi çok büyük") {
			t.Fatalf("%s %s: HTTP %d %q", r[0], r[1], res.Code, res.Body)
		}
	}
}

func TestCoverUploadMaxBodyBytes(t *testing.T) {
	// Fotoğrafı tarayıcı küçültür: kapak gövdesi 2 MiB dosya + 64 KiB multipart zarfıdır.
	if want := int64(2<<20 + 64<<10); CoverUploadMaxBodyBytes != want {
		t.Fatalf("CoverUploadMaxBodyBytes = %d, beklenen %d", CoverUploadMaxBodyBytes, want)
	}

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(BodyLimitWithRoutes(DefaultMaxBodyBytes, map[string]RouteLimit{
		"PUT /groups/:id/cover": {MaxBytes: CoverUploadMaxBodyBytes, Message: "görsel çok büyük"},
	}))
	router.PUT("/groups/:id/cover", func(c *gin.Context) {
		n, err := io.Copy(io.Discard, c.Request.Body)
		if err != nil {
			c.String(http.StatusBadRequest, "read failed")
			return
		}
		c.String(http.StatusOK, "%d", n)
	})
	send := func(size int64) *httptest.ResponseRecorder {
		res := httptest.NewRecorder()
		router.ServeHTTP(res, httptest.NewRequest(http.MethodPut, "/groups/42/cover", strings.NewReader(strings.Repeat("x", int(size)))))
		return res
	}
	if res := send(CoverUploadMaxBodyBytes); res.Code != http.StatusOK {
		t.Fatalf("sınırdaki kapak gövdesi: HTTP %d", res.Code)
	}
	// Sınırın 1 bayt üstü (eski 10 MiB sınırının çok altında): okunmadan 413.
	if res := send(CoverUploadMaxBodyBytes + 1); res.Code != http.StatusRequestEntityTooLarge || !strings.Contains(res.Body.String(), "görsel çok büyük") {
		t.Fatalf("sınırı aşan kapak gövdesi: HTTP %d %q", res.Code, res.Body)
	}
}
