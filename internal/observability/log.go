// Package observability sets up structured logging and OpenTelemetry tracing.
// Both are off by default — set OTEL_EXPORTER_OTLP_ENDPOINT to enable export
// to Cloud Trace (or any OTLP collector).
package observability

import (
	"log/slog"
	"os"
	"strings"
)

// NewLogger returns a slog.Logger writing JSON to stdout. levelStr accepts
// "debug" | "info" | "warn" | "error"; anything unknown defaults to info.
func NewLogger(levelStr string) *slog.Logger {
	var lvl slog.Level
	switch strings.ToLower(levelStr) {
	case "debug":
		lvl = slog.LevelDebug
	case "warn":
		lvl = slog.LevelWarn
	case "error":
		lvl = slog.LevelError
	default:
		lvl = slog.LevelInfo
	}
	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: lvl})
	return slog.New(handler)
}
