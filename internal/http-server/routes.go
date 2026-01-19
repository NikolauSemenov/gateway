package http_server

import (
	"context"
	_ "gateway/docs"
	"net/http"
	"strings"
	"time"

	"gateway/external/httpClient"
	"gateway/internal/config"
	"gateway/internal/http-server/auth"
	myMiddleware "gateway/internal/http-server/middleware"
	noteflow "gateway/internal/http-server/note-flow"
	"gateway/internal/observability"
	"log/slog"

	echoSwagger "github.com/swaggo/echo-swagger"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/redis/go-redis/v9"
)

type HandlersAuth interface {
	SendEmailCode(c echo.Context) error
	Register(c echo.Context) error
	Login(c echo.Context) error
	VerifyEmailCode(c echo.Context) error
	Logout(c echo.Context) error
}

type HandlersNoteFlow interface {
	Notes(c echo.Context) error
	Note(c echo.Context) error
	NoteActive(c echo.Context) error
	CreateNote(c echo.Context) error
	NoteLike(c echo.Context) error
	NoteComments(c echo.Context) error
	DeleteNote(c echo.Context) error
	DeleteComment(c echo.Context) error
}

type Doer interface {
	MakeRequest(ctx context.Context, r httpClient.RequestOptions) (*http.Response, error)
}

func RegisterRoutes(
	prefix string,
	e *echo.Echo,
	handlerAuth HandlersAuth,
	handlerNoteFlow HandlersNoteFlow,
	client Doer,
	authBaseURL string,
	cfg *config.Config,
	rdb *redis.Client,
	baseLogger *slog.Logger,
) {
	if baseLogger == nil {
		baseLogger = slog.Default()
	}

	metrics := observability.NewMetrics("gateway")
	metrics.RegisterGoAndProcessCollectors()

	e.Use(myMiddleware.RequestIDMiddleware(baseLogger))

	// Prometheus metrics middleware should be early to capture most handlers.
	e.Use(metrics.Middleware())

	e.Use(middleware.RequestLogger())
	e.Use(middleware.Recover())

	// Metrics endpoint (not under API prefix).
	// NOTE: you may want to protect this with network policy / ingress rules in prod.
	e.GET("/metrics", echo.WrapHandler(metrics.Handler()))

	// CORS: explicit allowlist. Enable only if origins are configured.
	if cfg != nil {
		cc := cfg.CORSConfig
		origins := splitCSV(cc.AllowOrigins)
		if len(origins) > 0 {
			corsCfg := middleware.CORSConfig{
				AllowOrigins:     origins,
				AllowMethods:     splitCSV(cc.AllowMethods),
				AllowHeaders:     splitCSV(cc.AllowHeaders),
				ExposeHeaders:    splitCSV(cc.ExposeHeaders),
				AllowCredentials: cc.AllowCredentials,
				MaxAge:           cc.MaxAge,
			}
			if len(corsCfg.AllowMethods) == 0 {
				corsCfg.AllowMethods = []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete, http.MethodOptions}
			}
			if len(corsCfg.AllowHeaders) == 0 {
				corsCfg.AllowHeaders = []string{"Authorization", "Content-Type", "Accept", myMiddleware.RequestIDHeader}
			}
			e.Use(middleware.CORSWithConfig(corsCfg))
		}
	}

	api := e.Group(prefix)

	// Swagger: expose only if enabled (usually disabled in prod).
	if cfg != nil && cfg.SwaggerConfig.Enabled {
		api.GET("/docs/*", echoSwagger.WrapHandler)
	}

	authGroup := api.Group("/auth")
	authGroup.Use(auth.LoggingMiddlewareAuth(baseLogger))
	authGroup.POST("/send-email-code", handlerAuth.SendEmailCode)
	authGroup.PUT("/verify-email-code", handlerAuth.VerifyEmailCode)
	authGroup.POST("/register", handlerAuth.Register)
	authGroup.POST("/login", handlerAuth.Login)
	authGroup.DELETE("/logout", handlerAuth.Logout)

	api.GET("/health", func(c echo.Context) error {
		return c.NoContent(http.StatusOK)
	})

	noteGroup := api.Group("/note-flow")
	noteGroup.Use(noteflow.LoggingMiddlewareNoteFlow(baseLogger))

	// JWT verify via auth service with Redis cache to reduce latency and improve resiliency.
	// Use configured verify path (AUTH_V1_VERIFY) instead of hardcoding it in the middleware.
	ttl := 0 * time.Second
	verifyBaseURL := authBaseURL
	if cfg != nil {
		ttl = cfg.RedisConfig.TTL
		if strings.TrimSpace(cfg.AuthConfig.V1Verify) != "" {
			verifyBaseURL = strings.TrimRight(authBaseURL, "/") + cfg.AuthConfig.V1Verify
		}
	}
	noteGroup.Use(myMiddleware.JWTAuthMiddlewareWithRedis(client, verifyBaseURL, rdb, ttl))

	noteGroup.GET("/notes", handlerNoteFlow.Notes)
	noteGroup.GET("/note/:note_id", handlerNoteFlow.Note)
	noteGroup.PUT("/note/active", handlerNoteFlow.NoteActive)
	noteGroup.POST("/note", handlerNoteFlow.CreateNote)
	noteGroup.POST("/note/like", handlerNoteFlow.NoteLike)
	noteGroup.POST("/note/comment", handlerNoteFlow.NoteComments)
	noteGroup.DELETE("/note/:note_id", handlerNoteFlow.DeleteNote)
	noteGroup.DELETE("/note/comment/:comment_id", handlerNoteFlow.DeleteComment)

}

func splitCSV(s string) []string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
