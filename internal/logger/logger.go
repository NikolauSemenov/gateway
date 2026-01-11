package logger

import (
	"io"
	"log/slog"
	"os"
	"strings"
)

func New(logLevel string, output io.Writer) *slog.Logger {
	if output == nil {
		output = os.Stdout
	}

	level := parseLevel(logLevel)

	return slog.New(slog.NewJSONHandler(output, &slog.HandlerOptions{
		Level: level,
	}))
}

func parseLevel(s string) slog.Level {
	switch strings.ToUpper(strings.TrimSpace(s)) {
	case "DEBUG":
		return slog.LevelDebug
	case "INFO", "":
		return slog.LevelInfo
	case "WARN", "WARNING":
		return slog.LevelWarn
	case "ERROR":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
