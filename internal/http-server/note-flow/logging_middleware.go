package note_flow

import (
	"log/slog"

	myMiddleware "gateway/internal/http-server/middleware"

	"github.com/labstack/echo/v4"
)

const loggingContextKey = "note_flow_logger"

func LoggerFromEcho(c echo.Context, fallback Logger) Logger {
	if base := myMiddleware.LoggerFromContext(c, nil); base != nil {
		return base.With(slog.String("component", "note_flow"))
	}

	if fallback != nil {
		return fallback
	}

	return slog.Default().With(slog.String("component", "note_flow"))
}

func LoggingMiddlewareNoteFlow(appLogger *slog.Logger) echo.MiddlewareFunc {
	if appLogger == nil {
		appLogger = slog.Default()
	}

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			// Prefer request-scoped logger from RequestIDMiddleware if present; otherwise use appLogger.
			base := myMiddleware.LoggerFromContext(c, appLogger)

			lgr := base.With(
				slog.String("component", "note_flow"),
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
