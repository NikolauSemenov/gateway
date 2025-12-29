package app

import (
	"fmt"
	"gateway/external/httpClient"
	"gateway/internal/config"
	httpserver "gateway/internal/http-server"
	"gateway/internal/http-server/auth"
	noteflow "gateway/internal/http-server/note-flow"

	"github.com/labstack/echo/v4"
)

type App struct {
	web       *echo.Echo
	configApp *config.Config
}

func NewApp() (*App, error) {
	configApp := config.GetConfig()
	if configApp == nil {
		return nil, fmt.Errorf("failed to load config")
	}
	fmt.Println(configApp)

	web := echo.New()

	clientHttp := httpClient.NewAPIClient(configApp)
	handlersAuth := auth.NewAuth(configApp, clientHttp)
	handlersNoteFlow := noteflow.NewNoteFlow(configApp, clientHttp)
	httpserver.RegisterRoutes(
		configApp.APIPrefix(),
		web,
		handlersAuth,
		handlersNoteFlow,
		clientHttp,
		configApp.BaseUrl,
	)
	return &App{web: web, configApp: configApp}, nil

}

func (app *App) Run() error {
	return app.web.Start(app.configApp.HTTPServer.Address)
}
