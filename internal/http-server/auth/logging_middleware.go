package auth

import (
	"log/slog"

	myMiddleware "gateway/internal/http-server/middleware"

	"github.com/labstack/echo/v4"
)

const loggingContextKey = "auth_logger"

func LoggerFromEcho(c echo.Context, fallback Logger) Logger {
	if base := myMiddleware.LoggerFromContext(c, nil); base != nil {
		return base.With(slog.String("component", "auth"))
	}

	if fallback != nil {
		return fallback
	}

	return slog.Default().With(slog.String("component", "auth"))
}

func LoggingMiddlewareAuth(appLogger *slog.Logger) echo.MiddlewareFunc {
	if appLogger == nil {
		appLogger = slog.Default()
	}

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			base := myMiddleware.LoggerFromContext(c, appLogger)

			lgr := base.With(
				slog.String("component", "auth"),
				slog.String("request_id", myMiddleware.RequestIDFromContext(c)),
			)

			c.Set(loggingContextKey, lgr)

			return next(c)
		}
	}
}

func GetInjectedLogger(c echo.Context) (*slog.Logger, bool) {
	v := c.Get(loggingContextKey)
	if v == nil {
		return nil, false
	}
	lgr, ok := v.(*slog.Logger)
	return lgr, ok && lgr != nil
}
