package app

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"gateway/external/httpClient"
	"gateway/internal/config"
	httpserver "gateway/internal/http-server"
	"gateway/internal/http-server/auth"
	noteflow "gateway/internal/http-server/note-flow"
	"gateway/internal/logger"

	"github.com/labstack/echo/v4"
	"github.com/redis/go-redis/v9"
)

type App struct {
	web        *echo.Echo
	httpServer *http.Server
	configApp  *config.Config
	log        *slog.Logger

	redis *redis.Client
}

func NewApp() (*App, error) {
	configApp, err := config.GetConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	log := logger.New(configApp.LogLevel, nil).With(
		slog.String("service", "gateway"),
		slog.String("log_level", configApp.LogLevel),
		slog.String("addr", configApp.HTTPServer.Address),
		slog.String("api_prefix", configApp.APIPrefix()),
	)

	web := echo.New()

	clientHttp := httpClient.NewAPIClient(configApp)

	var rdb *redis.Client
	if strings.TrimSpace(configApp.RedisConfig.Addr) != "" {
		rdb = redis.NewClient(&redis.Options{
			Addr:     configApp.RedisConfig.Addr,
			Password: configApp.RedisConfig.Password,
			DB:       configApp.RedisConfig.DB,
		})

		// Fail-fast: if Redis is configured but unavailable, refuse to start.
		// This prevents running with a silently broken JWT verify cache.
		pingCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		if err := rdb.Ping(pingCtx).Err(); err != nil {
			_ = rdb.Close()
			return nil, fmt.Errorf("redis is configured (%s) but unavailable: %w", configApp.RedisConfig.Addr, err)
		}

		log.Info("redis connection ok",
			slog.String("redis_addr", configApp.RedisConfig.Addr),
			slog.Int("redis_db", configApp.RedisConfig.DB),
			slog.Duration("jwt_verify_cache_ttl", configApp.RedisConfig.TTL),
		)
	}

	handlersAuth := auth.NewAuth(configApp, clientHttp, log.With(slog.String("component", "auth")), rdb)
	handlersNoteFlow := noteflow.NewNoteFlow(configApp, clientHttp, log.With(slog.String("component", "note_flow")))

	httpserver.RegisterRoutes(
		configApp.APIPrefix(),
		web,
		handlersAuth,
		handlersNoteFlow,
		clientHttp,
		configApp.AuthConfig.BaseUrl,
		configApp,
		rdb,
		log,
	)

	// Harden HTTP server: enforce timeouts/limits to prevent resource exhaustion and improve reliability.
	readTimeout := configApp.HTTPServer.Timeout
	writeTimeout := configApp.HTTPServer.Timeout
	idleTimeout := configApp.HTTPServer.IdleTimeout
	readHeaderTimeout := 5 * time.Second

	srv := &http.Server{
		Addr:              configApp.HTTPServer.Address,
		Handler:           web,
		ReadTimeout:       readTimeout,
		WriteTimeout:      writeTimeout,
		IdleTimeout:       idleTimeout,
		ReadHeaderTimeout: readHeaderTimeout,
		MaxHeaderBytes:    1 << 20, // 1 MiB
	}

	return &App{
		web:        web,
		httpServer: srv,
		configApp:  configApp,
		log:        log,
		redis:      rdb,
	}, nil
}

func (app *App) Run() error {
	return app.httpServer.ListenAndServe()
}

func (app *App) Shutdown(ctx context.Context) error {
	app.log.Info("shutting down application")

	// Close redis client first (stop new cache ops), then shutdown HTTP server.
	if app.redis != nil {
		_ = app.redis.Close()
	}

	return app.httpServer.Shutdown(ctx)
}
