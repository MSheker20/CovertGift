package logging

import (
	"bytes"
	"log"
	"log/slog"
	"strings"
	"testing"
)

func TestSetupFormatAndLevel(t *testing.T) {
	prev := slog.Default()
	t.Cleanup(func() { slog.SetDefault(prev) })

	var buf bytes.Buffer
	Setup(&buf, "", "", "production")
	slog.Info("hello", "k", "v")
	if !strings.HasPrefix(strings.TrimSpace(buf.String()), "{") {
		t.Fatalf("production varsayılanı JSON değil: %q", buf.String())
	}

	buf.Reset()
	Setup(&buf, "", "warn", "development")
	slog.Info("hidden")
	slog.Warn("shown")
	out := buf.String()
	if strings.Contains(out, "hidden") || !strings.Contains(out, "level=WARN") {
		t.Fatalf("development varsayılanı text değil ya da seviye uygulanmadı: %q", out)
	}

	buf.Reset()
	Setup(&buf, "json", "info", "development")
	log.Printf("legacy %d", 1)
	if !strings.Contains(buf.String(), `"msg":"legacy 1"`) {
		t.Fatalf("standart log paketi slog'a yönlenmedi: %q", buf.String())
	}
}
