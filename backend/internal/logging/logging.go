// Package logging API'nin yapılandırılmış (slog) loglarını kurar.
package logging

import (
	"io"
	"log/slog"
	"os"
	"strings"
)

// Setup LOG_FORMAT ("json" ya da "text") ve LOG_LEVEL ("debug", "info", "warn", "error")
// değerlerine göre bir slog.Logger kurar ve varsayılan yapar. slog.SetDefault standart
// log paketini de bu işleyiciye yönlendirdiği için eski log.Printf çağrıları da aynı
// biçimde yazılır. Biçim boşsa production'da json, diğer ortamlarda text kullanılır.
func Setup(w io.Writer, format, level, appEnv string) *slog.Logger {
	opts := &slog.HandlerOptions{Level: parseLevel(level)}
	format = strings.ToLower(strings.TrimSpace(format))
	if format == "" {
		format = "text"
		if appEnv == "production" {
			format = "json"
		}
	}
	var handler slog.Handler
	if format == "json" {
		handler = slog.NewJSONHandler(w, opts)
	} else {
		handler = slog.NewTextHandler(w, opts)
	}
	logger := slog.New(handler)
	slog.SetDefault(logger)
	return logger
}

// FromEnv Setup'ı ortam değişkenleriyle çağırır ve stdout'a yazar.
func FromEnv() *slog.Logger {
	return Setup(os.Stdout, os.Getenv("LOG_FORMAT"), os.Getenv("LOG_LEVEL"), os.Getenv("APP_ENV"))
}

func parseLevel(raw string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
