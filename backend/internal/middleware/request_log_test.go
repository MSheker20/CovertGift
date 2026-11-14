package middleware

import (
	"bytes"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/MSheker20/CovertGift/backend/internal/controllers"
	"github.com/gin-gonic/gin"
)

func newLoggedRouter(buf *bytes.Buffer) *gin.Engine {
	gin.SetMode(gin.TestMode)
	logger := slog.New(slog.NewJSONHandler(buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	r := gin.New()
	r.Use(Recovery(logger), RequestLogger(logger))
	r.GET("/ok", func(c *gin.Context) { controllers.RespondOK(c, "ok") })
	r.GET("/fail", func(c *gin.Context) {
		controllers.RespondInternal(c, "Genel hata", errors.New("pq: connection refused at 10.0.0.5"))
	})
	r.GET("/invites/:token", func(c *gin.Context) { controllers.RespondOK(c, "ok") })
	r.GET("/panic", func(c *gin.Context) { panic("boom") })
	return r
}

func lastLogLine(t *testing.T, buf *bytes.Buffer) map[string]any {
	t.Helper()
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	var entry map[string]any
	if err := json.Unmarshal([]byte(lines[len(lines)-1]), &entry); err != nil {
		t.Fatalf("log satırı JSON değil: %q", buf.String())
	}
	return entry
}

func TestRequestLoggerWritesOneStructuredLine(t *testing.T) {
	var buf bytes.Buffer
	r := newLoggedRouter(&buf)
	req := httptest.NewRequest(http.MethodGet, "/ok?token=secret-value", nil)
	req.Header.Set("Authorization", "Bearer secret-jwt")
	res := httptest.NewRecorder()
	r.ServeHTTP(res, req)

	id := res.Header().Get(RequestIDHeader)
	if id == "" {
		t.Fatal("yanıtta X-Request-ID yok")
	}
	entry := lastLogLine(t, &buf)
	if entry["msg"] != "http_request" || entry["level"] != "INFO" || entry["status"] != float64(200) ||
		entry["path"] != "/ok" || entry["route"] != "/ok" || entry["request_id"] != id {
		t.Fatalf("beklenmeyen log: %v", entry)
	}
	if strings.Contains(buf.String(), "secret") {
		t.Fatalf("log sorgu dizesini veya Authorization başlığını içeriyor: %s", buf.String())
	}
}

func TestRespondInternalHidesErrorFromClientButLogsIt(t *testing.T) {
	var buf bytes.Buffer
	r := newLoggedRouter(&buf)
	res := httptest.NewRecorder()
	r.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/fail", nil))

	if res.Code != http.StatusInternalServerError || strings.Contains(res.Body.String(), "10.0.0.5") {
		t.Fatalf("iç hata istemciye sızdı: HTTP %d %s", res.Code, res.Body)
	}
	entry := lastLogLine(t, &buf)
	if entry["level"] != "ERROR" || !strings.Contains(entry["error"].(string), "connection refused") {
		t.Fatalf("iç hata loglanmadı: %v", entry)
	}
}

func TestRecoveryReturnsEnvelopeAndLogsPanic(t *testing.T) {
	var buf bytes.Buffer
	r := newLoggedRouter(&buf)
	res := httptest.NewRecorder()
	r.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/panic", nil))

	var body controllers.Response
	if res.Code != http.StatusInternalServerError || json.Unmarshal(res.Body.Bytes(), &body) != nil || body.Success || body.Error == "" {
		t.Fatalf("panik standart zarfla dönmedi: HTTP %d %s", res.Code, res.Body)
	}
	if !strings.Contains(buf.String(), `"msg":"panic"`) || !strings.Contains(buf.String(), "boom") {
		t.Fatalf("panik loglanmadı: %s", buf.String())
	}
}

func TestRequestLoggerDoesNotLogInviteToken(t *testing.T) {
	var buf bytes.Buffer
	r := newLoggedRouter(&buf)
	r.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/invites/raw-secret-token", nil))
	if strings.Contains(buf.String(), "raw-secret-token") {
		t.Fatalf("log ham davet token'ını içeriyor: %s", buf.String())
	}
	if entry := lastLogLine(t, &buf); entry["path"] != "/invites/:token" {
		t.Fatalf("beklenmeyen path: %v", entry)
	}
}

func TestRequestLoggerRedactsInviteTokenWhenNoRouteMatched(t *testing.T) {
	var buf bytes.Buffer
	r := newLoggedRouter(&buf)
	for _, c := range []struct{ method, path, want string }{
		{http.MethodPost, "/api/v1/invites/raw-secret-token", "/api/v1/invites/:token"},
		{http.MethodPost, "/api/v1/invites/raw-secret-token/accept/", "/api/v1/invites/:token/accept/"},
		{http.MethodGet, "/invites/raw-secret-token/", "/invites/:token/"},
		{http.MethodGet, "/invite/raw-secret-token", "/invite/:token"},
	} {
		buf.Reset()
		r.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(c.method, c.path, nil))
		if strings.Contains(buf.String(), "raw-secret-token") {
			t.Fatalf("%s %s: log ham token içeriyor: %s", c.method, c.path, buf.String())
		}
		if buf.Len() == 0 {
			continue // gin eğik çizgi yönlendirmesini middleware'den önce yanıtlar; log yok, sızıntı yok
		}
		if entry := lastLogLine(t, &buf); entry["path"] != c.want {
			t.Fatalf("%s: path %v, beklenen %s", c.path, entry["path"], c.want)
		}
	}
}
