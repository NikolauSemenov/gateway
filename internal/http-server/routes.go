package http_server

import (
	_ "gateway/docs"
	"gateway/external/httpClient"
	myMiddleware "gateway/internal/http-server/middleware"
	"net/http"

	echoSwagger "github.com/swaggo/echo-swagger"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
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
}

func RegisterRoutes(
	prefix string,
	e *echo.Echo,
	handlerAuth HandlersAuth,
	handlerNoteFlow HandlersNoteFlow,
	client *httpClient.APIClient,
	authBaseURL string,
) {
	e.Use(middleware.RequestLogger())
	e.Use(middleware.Recover())
	e.Use(middleware.CORS())

	api := e.Group(prefix)

	api.GET("/docs/*", echoSwagger.WrapHandler)
	api.POST("/auth/send-email-code", handlerAuth.SendEmailCode)
	api.PUT("/auth/verify-email-code", handlerAuth.VerifyEmailCode)
	api.POST("/auth/register", handlerAuth.Register)
	api.POST("/auth/login", handlerAuth.Login)
	api.DELETE("/auth/logout", handlerAuth.Logout)
	api.GET("/health", func(c echo.Context) error {
		return c.NoContent(http.StatusOK)
	})

	noteGroup := api.Group("/note-flow")
	noteGroup.Use(myMiddleware.JWTAuthMiddleware(client, authBaseURL))

	noteGroup.GET("/notes", handlerNoteFlow.Notes)
	noteGroup.GET("/note/:note_id", handlerNoteFlow.Note)
	noteGroup.PUT("/note/active", handlerNoteFlow.NoteActive)
	noteGroup.POST("/note", handlerNoteFlow.CreateNote)
	noteGroup.POST("/note/like", handlerNoteFlow.NoteLike)
	noteGroup.POST("/note/comment", handlerNoteFlow.NoteComments)

}
