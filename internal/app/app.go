package app

import (
	"context"
	"fmt"
	"log/slog"

	"gateway/external/httpClient"
	"gateway/internal/config"
	httpserver "gateway/internal/http-server"
	"gateway/internal/http-server/auth"
	noteflow "gateway/internal/http-server/note-flow"
	"gateway/internal/logger"

	"github.com/labstack/echo/v4"
)

type App struct {
	web       *echo.Echo
	configApp *config.Config
	log       *slog.Logger
}

func NewApp() (*App, error) {
	configApp := config.GetConfig()
	if configApp == nil {
		return nil, fmt.Errorf("failed to load config: config is nil (check env vars)")
	}

	log := logger.New(configApp.LogLevel, nil).With(
		slog.String("service", "gateway"),
		slog.String("log_level", configApp.LogLevel),
		slog.String("addr", configApp.HTTPServer.Address),
		slog.String("api_prefix", configApp.APIPrefix()),
	)

	web := echo.New()

	clientHttp := httpClient.NewAPIClient(configApp)

	handlersAuth := auth.NewAuth(configApp, clientHttp, log.With(slog.String("component", "auth")))
	handlersNoteFlow := noteflow.NewNoteFlow(configApp, clientHttp, log.With(slog.String("component", "note_flow")))

	httpserver.RegisterRoutes(
		configApp.APIPrefix(),
		web,
		handlersAuth,
		handlersNoteFlow,
		clientHttp,
		configApp.BaseUrl,
		log,
	)

	return &App{web: web, configApp: configApp, log: log}, nil
}

func (app *App) Run() error {
	return app.web.Start(app.configApp.HTTPServer.Address)
}

func (app *App) Shutdown(ctx context.Context) error {
	app.log.Info("shutting down application")
	return app.web.Shutdown(ctx)
}
