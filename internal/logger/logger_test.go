package logger

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"testing"
)

func TestParseLevel(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		in   string
		want slog.Level
	}{
		{name: "empty defaults to info", in: "", want: slog.LevelInfo},
		{name: "spaces defaults to info", in: "   ", want: slog.LevelInfo},

		{name: "debug", in: "DEBUG", want: slog.LevelDebug},
		{name: "debug lowercase", in: "debug", want: slog.LevelDebug},
		{name: "debug mixed and spaced", in: "  DeBuG  ", want: slog.LevelDebug},

		{name: "info", in: "INFO", want: slog.LevelInfo},
		{name: "warn", in: "WARN", want: slog.LevelWarn},
		{name: "warning", in: "WARNING", want: slog.LevelWarn},
		{name: "error", in: "ERROR", want: slog.LevelError},

		{name: "unknown defaults to info", in: "TRACE", want: slog.LevelInfo},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := parseLevel(tc.in); got != tc.want {
				t.Fatalf("parseLevel(%q) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}

func TestNew_UsesParsedLevel(t *testing.T) {
	t.Parallel()

	t.Run("DEBUG enables debug logs", func(t *testing.T) {
		t.Parallel()

		var buf bytes.Buffer
		log := New("DEBUG", &buf)

		log.Debug("dbg", slog.String("k", "v"))

		if buf.Len() == 0 {
			t.Fatalf("expected debug log to be written at DEBUG level")
		}

		var m map[string]any
		if err := json.Unmarshal(buf.Bytes(), &m); err != nil {
			t.Fatalf("expected JSON log line, unmarshal err=%v raw=%q", err, buf.String())
		}
		if m["msg"] != "dbg" {
			t.Fatalf("expected msg %q, got %#v", "dbg", m["msg"])
		}
	})

	t.Run("INFO suppresses debug logs", func(t *testing.T) {
		t.Parallel()

		var buf bytes.Buffer
		log := New("INFO", &buf)

		log.Debug("dbg")
		if buf.Len() != 0 {
			t.Fatalf("expected debug to be suppressed at INFO level; got %q", buf.String())
		}

		log.Info("info")
		if buf.Len() == 0 {
			t.Fatalf("expected info log to be written at INFO level")
		}
	})

	t.Run("unknown level defaults to INFO", func(t *testing.T) {
		t.Parallel()

		var buf bytes.Buffer
		log := New("TRACE", &buf)

		log.Debug("dbg")
		if buf.Len() != 0 {
			t.Fatalf("expected debug to be suppressed when level defaults to INFO; got %q", buf.String())
		}
	})
}
