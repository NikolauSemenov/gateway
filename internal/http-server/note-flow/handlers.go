package note_flow

import (
	"gateway/external/httpClient"
	"gateway/internal/config"
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

// Notes обрабатывает запрос на /notes
// @Summary Получить все активные заметки
// @Description Проксирует запрос по пути /notes
// @Tags note-flow
// @Param Authorization header string true "Bearer token"
// @Success 200 {object} NotesResponse "OK"
// @Failure 422 {string} string "Ошибка валидации"
// @Router /gateway/api/v1/note-flow/notes [get]
func (a *NoteFlow) Notes(c echo.Context) error {
	var resp []NotesResponse

	httpRes, err := a.clientHttp.MakeRequest(
		c.Request().Context(),
		httpClient.RequestOptions{
			Method:   http.MethodGet,
			URL:      a.config.BaseUrlNotes + a.config.V1Notes,
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
	noteID := c.Param("note_id")

	var resp NotesResponse

	httpRes, err := a.clientHttp.MakeRequest(
		c.Request().Context(),
		httpClient.RequestOptions{
			Method:   http.MethodGet,
			URL:      a.config.BaseUrlNotes + a.config.V1Note + "/" + noteID,
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
	query := url.Values{}
	query.Set("id", c.QueryParam("id"))

	httpRes, err := a.clientHttp.MakeRequest(
		c.Request().Context(),
		httpClient.RequestOptions{
			Method: http.MethodPut,
			URL:    a.config.BaseUrlNotes + a.config.V1NoteActive,
			Query:  query,
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
// @Summary Создание заметки
// @Description Проксирует запрос по пути /note
// @Tags note-flow
// @Param Authorization header string true "Bearer token"
// @Param request body CreateNoteRequest true "Данные запроса"
// @Success 200 {object} CreateNoteResponse "OK"
// @Failure 422 {string} string "Ошибка валидации"
// @Router /gateway/api/v1/note-flow/note [post]
func (a *NoteFlow) CreateNote(c echo.Context) error {
	var req CreateNoteRequest
	var resp CreateNoteResponse

	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, nil)
	}

	httpRes, err := a.clientHttp.MakeRequest(
		c.Request().Context(),
		httpClient.RequestOptions{
			Method:   http.MethodPost,
			URL:      a.config.BaseUrlNotes + a.config.V1Note,
			Body:     req,
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
