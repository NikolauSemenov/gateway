package middleware

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log/slog"

	"github.com/labstack/echo/v4"
)

type ctxKey int

const (
	requestIDCtxKey ctxKey = iota
	loggerCtxKey
)

const RequestIDContextKey = "request_id"

const RequestIDHeader = "X-Request-ID"

func LoggerFromContext(c echo.Context, base *slog.Logger) *slog.Logger {
	if v := c.Get("slog_logger"); v != nil {
		if lgr, ok := v.(*slog.Logger); ok && lgr != nil {
			return lgr
		}
	}
	if base == nil {
		base = slog.Default()
	}
	return base
}

func RequestIDFromContext(c echo.Context) string {
	if v := c.Get(RequestIDContextKey); v != nil {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

func WithRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, requestIDCtxKey, requestID)
}

func RequestIDFromStdContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if v := ctx.Value(requestIDCtxKey); v != nil {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

func WithLogger(ctx context.Context, log *slog.Logger) context.Context {
	return context.WithValue(ctx, loggerCtxKey, log)
}

func LoggerFromStdContext(ctx context.Context) *slog.Logger {
	if ctx == nil {
		return nil
	}
	if v := ctx.Value(loggerCtxKey); v != nil {
		if lgr, ok := v.(*slog.Logger); ok {
			return lgr
		}
	}
	return nil
}

func RequestIDMiddleware(base *slog.Logger) echo.MiddlewareFunc {
	if base == nil {
		base = slog.Default()
	}

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			req := c.Request()
			res := c.Response()

			rid := req.Header.Get(RequestIDHeader)
			if rid == "" {
				rid = newRequestID()
			}

			res.Header().Set(RequestIDHeader, rid)

			c.Set(RequestIDContextKey, rid)

			lgr := base.With(
				slog.String("request_id", rid),
				slog.String("method", req.Method),
				slog.String("path", req.URL.Path),
			)
			c.Set("slog_logger", lgr)

			ctx := WithRequestID(req.Context(), rid)
			ctx = WithLogger(ctx, lgr)
			c.SetRequest(req.WithContext(ctx))

			return next(c)
		}
	}
}

func newRequestID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "00000000000000000000000000000000"
	}
	return hex.EncodeToString(b[:])
}
