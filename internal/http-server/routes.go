package http_server

import (
	"context"
	_ "gateway/docs"
	"net/http"

	"gateway/external/httpClient"
	"gateway/internal/http-server/auth"
	myMiddleware "gateway/internal/http-server/middleware"
	noteflow "gateway/internal/http-server/note-flow"
	"log/slog"

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
	baseLogger *slog.Logger,
) {
	if baseLogger == nil {
		baseLogger = slog.Default()
	}

	e.Use(myMiddleware.RequestIDMiddleware(baseLogger))

	e.Use(middleware.RequestLogger())
	e.Use(middleware.Recover())
	e.Use(middleware.CORS())

	api := e.Group(prefix)

	api.GET("/docs/*", echoSwagger.WrapHandler)

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
	noteGroup.Use(myMiddleware.JWTAuthMiddleware(client, authBaseURL))

	noteGroup.GET("/notes", handlerNoteFlow.Notes)
	noteGroup.GET("/note/:note_id", handlerNoteFlow.Note)
	noteGroup.PUT("/note/active", handlerNoteFlow.NoteActive)
	noteGroup.POST("/note", handlerNoteFlow.CreateNote)
	noteGroup.POST("/note/like", handlerNoteFlow.NoteLike)
	noteGroup.POST("/note/comment", handlerNoteFlow.NoteComments)
	noteGroup.DELETE("/note/:note_id", handlerNoteFlow.DeleteNote)
	noteGroup.DELETE("/note/comment/:comment_id", handlerNoteFlow.DeleteComment)

}
