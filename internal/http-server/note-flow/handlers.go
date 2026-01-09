package note_flow

import (
	"gateway/external/httpClient"
	"gateway/internal/config"
	"gateway/internal/http-server/middleware"
	"net/http"
	"net/url"

	"github.com/labstack/echo/v4"
)

type NoteFlow struct {
	clientHttp *httpClient.APIClient
	config     *config.Config
}

func NewNoteFlow(config *config.Config, clientHttp *httpClient.APIClient) *NoteFlow {
	return &NoteFlow{
		config:     config,
		clientHttp: clientHttp,
	}
}

func userIDFromContext(c echo.Context) (string, error) {
	userID, ok := c.Get(middleware.UserIDContextKey).(string)
	if !ok || userID == "" {
		return "", echo.NewHTTPError(http.StatusUnauthorized, "User not authorized")
	}

	return userID, nil
}

type userContext struct {
	id      string
	headers http.Header
}

func buildUserContext(c echo.Context) (*userContext, error) {
	userID, err := userIDFromContext(c)
	if err != nil {
		return nil, err
	}

	headers := http.Header{}
	headers.Set("X-User-ID", userID)
	authHeader := c.Request().Header.Get("Authorization")
	if authHeader != "" {
		headers.Set("Authorization", authHeader)
	}

	return &userContext{
		id:      userID,
		headers: headers,
	}, nil
}

func (u *userContext) withUserQuery(base url.Values) url.Values {
	if base == nil {
		base = url.Values{}
	}

	base.Set("user_id", u.id)
	return base
}

// Notes обрабатывает запрос на /notes
// @Summary Получить все активные заметки
// @Description Проксирует запрос по пути /notes
// @Tags note-flow
// @Param Authorization header string true "Bearer token"
// @Success 200 {object} NotesResponse "OK"
// @Failure 422 {string} string "Ошибка валидации"
// @Router /gateway/api/v1/note-flow/notes [get]
func (a *NoteFlow) Notes(c echo.Context) error {
	uc, err := buildUserContext(c)
	if err != nil {
		return err
	}

	var resp []NotesResponse

	httpRes, err := a.clientHttp.MakeRequest(
		c.Request().Context(),
		httpClient.RequestOptions{
			Method:   http.MethodGet,
			URL:      a.config.BaseUrlNotes + a.config.V1Notes,
			Headers:  uc.headers,
			Query:    uc.withUserQuery(nil),
			Response: &resp,
		},
	)
	if err != nil {
		if httpRes != nil {
			return c.JSON(httpRes.StatusCode, err.Error())
		}
		return c.JSON(http.StatusServiceUnavailable, "Server unavailable")
	}
	return c.JSON(httpRes.StatusCode, resp)
}

// Note обрабатывает запрос на /note/{note_id}
// @Summary Получение заметки по ID
// @Description Проксирует запрос по пути /note/{note_id}
// @Tags note-flow
// @Param Authorization header string true "Bearer token"
// @Param note_id path int true "ID заметки"
// @Success 200 {object} NotesResponse "OK"
// @Failure 400 {string} string "Неверный ID"
// @Failure 422 {string} string "Ошибка валидации"
// @Router /gateway/api/v1/note-flow/note/{note_id} [get]
func (a *NoteFlow) Note(c echo.Context) error {
	uc, err := buildUserContext(c)
	if err != nil {
		return err
	}

	noteID := c.Param("note_id")

	var resp NotesResponse
	query := uc.withUserQuery(nil)

	httpRes, err := a.clientHttp.MakeRequest(
		c.Request().Context(),
		httpClient.RequestOptions{
			Method:   http.MethodGet,
			URL:      a.config.BaseUrlNotes + a.config.V1Note + "/" + noteID,
			Headers:  uc.headers,
			Query:    query,
			Response: &resp,
		},
	)
	if err != nil {
		if httpRes != nil {
			return c.JSON(httpRes.StatusCode, err.Error())
		}
		return c.JSON(http.StatusServiceUnavailable, "Server unavailable")
	}

	return c.JSON(httpRes.StatusCode, resp)
}

// NoteActive обрабатывает запрос на /note/active
// @Summary Пометить заметку как не активную
// @Description Проксирует запрос по пути /note/active
// @Tags note-flow
// @Param Authorization header string true "Bearer token"
// @Param id query int true "ID заметки"
// @Success 200 {string} string "OK"
// @Failure 422 {string} string "Ошибка валидации"
// @Router /gateway/api/v1/note-flow/note/active [put]
func (a *NoteFlow) NoteActive(c echo.Context) error {
	uc, err := buildUserContext(c)
	if err != nil {
		return err
	}

	query := uc.withUserQuery(url.Values{
		"id": []string{c.QueryParam("id")},
	})

	httpRes, err := a.clientHttp.MakeRequest(
		c.Request().Context(),
		httpClient.RequestOptions{
			Method:  http.MethodPut,
			URL:     a.config.BaseUrlNotes + a.config.V1NoteActive,
			Headers: uc.headers,
			Query:   query,
		},
	)
	if err != nil {
		if httpRes != nil {
			return c.JSON(httpRes.StatusCode, err.Error())
		}
		return c.JSON(http.StatusServiceUnavailable, "Server unavailable")
	}

	return c.JSON(httpRes.StatusCode, nil)
}

// CreateNote обрабатывает запрос на /note
// @Summary Создать заметку
// @Description Проксирует запрос по пути /note
// @Tags note-flow
// @Param Authorization header string true "Bearer token"
// @Param request body CreateNoteRequest true "Данные запроса"
// @Success 200 {object} CreateNoteResponse "OK"
// @Failure 422 {string} string "Ошибка валидации"
// @Router /gateway/api/v1/note-flow/note [post]
func (a *NoteFlow) CreateNote(c echo.Context) error {
	uc, err := buildUserContext(c)
	if err != nil {
		return err
	}

	var req CreateNoteRequest
	var resp CreateNoteResponse

	if err = c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, nil)
	}

	httpRes, err := a.clientHttp.MakeRequest(
		c.Request().Context(),
		httpClient.RequestOptions{
			Method:   http.MethodPost,
			URL:      a.config.BaseUrlNotes + a.config.V1Note,
			Headers:  uc.headers,
			Body:     &req,
			Response: &resp,
		},
	)
	if err != nil {
		if httpRes != nil {
			return c.JSON(httpRes.StatusCode, err.Error())
		}
		return c.JSON(http.StatusServiceUnavailable, "Server unavailable")
	}

	return c.JSON(httpRes.StatusCode, resp)
}

// NoteLike обрабатывает запрос на /note/like
// @Summary Лайкнуть заметку
// @Description Проксирует запрос по пути /note/like
// @Tags likes
// @Param Authorization header string true "Bearer token"
// @Param request body LikeNoteRequest true "Данные запроса"
// @Success 201 {object} LikeNoteResponse "Created"
// @Failure 400 {string} string "Неверный запрос"
// @Failure 422 {string} string "Ошибка валидации"
// @Router /gateway/api/v1/note-flow/note/like [post]
func (a *NoteFlow) NoteLike(c echo.Context) error {
	uc, err := buildUserContext(c)
	if err != nil {
		return err
	}
	var req LikeNoteRequest
	var resp NoteLikesCountResponse

	if err = c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, nil)
	}
	httpRes, err := a.clientHttp.MakeRequest(
		c.Request().Context(),
		httpClient.RequestOptions{
			Method:   http.MethodPost,
			URL:      a.config.BaseUrlNotes + a.config.V1NoteLike,
			Headers:  uc.headers,
			Body:     &req,
			Response: &resp,
		},
	)
	if err != nil {
		if httpRes != nil {
			return c.JSON(httpRes.StatusCode, err.Error())
		}
		return c.JSON(http.StatusServiceUnavailable, "Server unavailable")
	}

	return c.JSON(httpRes.StatusCode, resp)
}

// NoteComments обрабатывает запрос на /note/comments
// @Summary Создать комментарий
// @Description Проксирует запрос по пути /note/comments
// @Tags Comments
// @Param Authorization header string true "Bearer token"
// @Param request body NoteCommentsRequest true "Данные запроса"
// @Success 200 {object} NoteCommentsResponse "OK"
// @Failure 422 {string} string "Ошибка валидации"
// @Router /gateway/api/v1/note-flow/note/comment [post]
func (a *NoteFlow) NoteComments(c echo.Context) error {
	uc, err := buildUserContext(c)
	if err != nil {
		return err
	}
	var req NoteCommentsRequest
	var resp NoteCommentsResponse

	if err = c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, nil)
	}

	httpRes, err := a.clientHttp.MakeRequest(
		c.Request().Context(),
		httpClient.RequestOptions{
			Method:   http.MethodPost,
			URL:      a.config.BaseUrlNotes + a.config.V1NoteComment,
			Headers:  uc.headers,
			Body:     &req,
			Response: &resp,
		},
	)
	if err != nil {
		if httpRes != nil {
			return c.JSON(httpRes.StatusCode, err.Error())
		}
		return c.JSON(http.StatusServiceUnavailable, "Server unavailable")
	}

	return c.JSON(httpRes.StatusCode, resp)
}
