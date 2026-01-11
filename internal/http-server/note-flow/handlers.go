package note_flow

import (
	"context"
	"net/http"
	"net/url"
	"strings"

	"gateway/external/httpClient"
	"gateway/internal/config"
	"gateway/internal/http-server/common"
	"gateway/internal/http-server/middleware"

	"github.com/labstack/echo/v4"
)

type Doer interface {
	MakeRequest(ctx context.Context, r httpClient.RequestOptions) (*http.Response, error)
}

type Logger interface {
	Debug(msg string, args ...any)
	Info(msg string, args ...any)
	Warn(msg string, args ...any)
	Error(msg string, args ...any)
}

type NoteFlow struct {
	clientHttp Doer
	config     *config.Config
	log        Logger
}

func NewNoteFlow(cfg *config.Config, client Doer, log Logger) *NoteFlow {
	return &NoteFlow{
		config:     cfg,
		clientHttp: client,
		log:        log,
	}
}

type userContext struct {
	id      string
	headers http.Header
}

func (a *NoteFlow) logger(c echo.Context) Logger {
	return LoggerFromEcho(c, a.log)
}

func (a *NoteFlow) userID(c echo.Context) (string, error) {
	userID, ok := c.Get(middleware.UserIDContextKey).(string)
	if !ok || strings.TrimSpace(userID) == "" {
		return "", echo.NewHTTPError(http.StatusUnauthorized, msgUserNotAuthorized)
	}
	return userID, nil
}

func (a *NoteFlow) buildUserContext(c echo.Context) (*userContext, error) {
	userID, err := a.userID(c)
	if err != nil {
		return nil, err
	}

	h := http.Header{}
	h.Set(headerUserID, userID)

	if authHeader := strings.TrimSpace(c.Request().Header.Get("Authorization")); authHeader != "" {
		h.Set("Authorization", authHeader)
	}

	return &userContext{
		id:      userID,
		headers: h,
	}, nil
}

func (u *userContext) withUserQuery(base url.Values) url.Values {
	if base == nil {
		base = url.Values{}
	}
	base.Set(queryUserID, u.id)
	return base
}

func joinURL(base, path string) string {
	b := strings.TrimRight(strings.TrimSpace(base), "/")
	p := strings.TrimSpace(path)
	if p == "" {
		return b
	}
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	return b + p
}

func (a *NoteFlow) doUpstream(c echo.Context, opts httpClient.RequestOptions, op string, extraKV ...any) (*http.Response, error) {
	log := a.logger(c)

	httpRes, err := a.clientHttp.MakeRequest(c.Request().Context(), opts)
	if err != nil {
		if httpRes != nil {
			kv := []any{
				logKeyOp, op,
				logKeyUpstreamURL, opts.URL,
				logKeyStatus, httpRes.StatusCode,
				logKeyErr, err,
			}
			kv = append(kv, extraKV...)
			log.Warn("note-flow upstream returned error", kv...)
			return httpRes, err
		}

		kv := []any{
			logKeyOp, op,
			logKeyUpstreamURL, opts.URL,
			logKeyErr, err,
		}
		kv = append(kv, extraKV...)
		log.Error("note-flow upstream unavailable", kv...)
		return nil, err
	}

	kv := []any{
		logKeyOp, op,
		logKeyUpstreamURL, opts.URL,
		logKeyStatus, httpRes.StatusCode,
		logKeyHasAuthHeader, strings.TrimSpace(c.Request().Header.Get("Authorization")) != "",
	}
	kv = append(kv, extraKV...)
	log.Info("note-flow request proxied successfully", kv...)

	return httpRes, nil
}

func (a *NoteFlow) bindJSON(c echo.Context, dst any) bool {
	if err := c.Bind(dst); err != nil {
		a.logger(c).Warn("failed to bind request body", logKeyErr, err)
		_ = common.WriteBadRequest(c, msgInvalidBody)
		return false
	}
	return true
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
	log := a.logger(c)

	uc, err := a.buildUserContext(c)
	if err != nil {
		log.Warn("user not authorized for notes", logKeyErr, err)
		return err
	}

	var resp []NotesResponse

	opts := httpClient.RequestOptions{
		Method:   http.MethodGet,
		URL:      joinURL(a.config.BaseUrlNotes, a.config.V1Notes),
		Headers:  uc.headers,
		Query:    uc.withUserQuery(nil),
		Response: &resp,
	}

	httpRes, err := a.doUpstream(c, opts, "notes")
	if err != nil {
		if httpRes != nil {
			return common.UpstreamError(c, httpRes.StatusCode, http.StatusBadGateway, err.Error())
		}
		return common.WriteServiceUnavailable(c, msgNoteFlowUnavailable)
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
	log := a.logger(c)

	uc, err := a.buildUserContext(c)
	if err != nil {
		log.Warn("user not authorized for note", logKeyErr, err)
		return err
	}

	noteID := strings.TrimSpace(c.Param("note_id"))
	if noteID == "" {
		// Avoid hitting upstream with empty path param.
		return common.WriteBadRequest(c, "Invalid note_id")
	}

	var resp NotesResponse

	opts := httpClient.RequestOptions{
		Method:   http.MethodGet,
		URL:      joinURL(a.config.BaseUrlNotes, a.config.V1Note) + "/" + url.PathEscape(noteID),
		Headers:  uc.headers,
		Query:    uc.withUserQuery(nil),
		Response: &resp,
	}

	httpRes, err := a.doUpstream(c, opts, "note", logKeyNoteID, noteID)
	if err != nil {
		if httpRes != nil {
			return common.UpstreamError(c, httpRes.StatusCode, http.StatusBadGateway, err.Error())
		}
		return common.WriteServiceUnavailable(c, msgNoteFlowUnavailable)
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
	log := a.logger(c)

	uc, err := a.buildUserContext(c)
	if err != nil {
		log.Warn("user not authorized for note active", logKeyErr, err)
		return err
	}

	id := strings.TrimSpace(c.QueryParam("id"))
	if id == "" {
		return common.WriteBadRequest(c, "Missing id")
	}

	q := url.Values{}
	q.Set("id", id)
	q = uc.withUserQuery(q)

	opts := httpClient.RequestOptions{
		Method:  http.MethodPut,
		URL:     joinURL(a.config.BaseUrlNotes, a.config.V1NoteActive),
		Headers: uc.headers,
		Query:   q,
	}

	httpRes, err := a.doUpstream(c, opts, "note_active", logKeyID, id)
	if err != nil {
		if httpRes != nil {
			return common.UpstreamError(c, httpRes.StatusCode, http.StatusBadGateway, err.Error())
		}
		return common.WriteServiceUnavailable(c, msgNoteFlowUnavailable)
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
	log := a.logger(c)

	uc, err := a.buildUserContext(c)
	if err != nil {
		log.Warn("user not authorized for create note", logKeyErr, err)
		return err
	}

	var req CreateNoteRequest
	if ok := a.bindJSON(c, &req); !ok {
		return nil
	}

	var resp CreateNoteResponse

	opts := httpClient.RequestOptions{
		Method:   http.MethodPost,
		URL:      joinURL(a.config.BaseUrlNotes, a.config.V1Note),
		Headers:  uc.headers,
		Body:     &req,
		Response: &resp,
	}

	httpRes, err := a.doUpstream(c, opts, "create_note")
	if err != nil {
		if httpRes != nil {
			return common.UpstreamError(c, httpRes.StatusCode, http.StatusBadGateway, err.Error())
		}
		return common.WriteServiceUnavailable(c, msgNoteFlowUnavailable)
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
	log := a.logger(c)

	uc, err := a.buildUserContext(c)
	if err != nil {
		log.Warn("user not authorized for note like", logKeyErr, err)
		return err
	}

	var req LikeNoteRequest
	if ok := a.bindJSON(c, &req); !ok {
		return nil
	}

	var resp NoteLikesCountResponse

	opts := httpClient.RequestOptions{
		Method:   http.MethodPost,
		URL:      joinURL(a.config.BaseUrlNotes, a.config.V1NoteLike),
		Headers:  uc.headers,
		Body:     &req,
		Response: &resp,
	}

	httpRes, err := a.doUpstream(c, opts, "note_like")
	if err != nil {
		if httpRes != nil {
			return common.UpstreamError(c, httpRes.StatusCode, http.StatusBadGateway, err.Error())
		}
		return common.WriteServiceUnavailable(c, msgNoteFlowUnavailable)
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
	log := a.logger(c)

	uc, err := a.buildUserContext(c)
	if err != nil {
		log.Warn("user not authorized for note comments", logKeyErr, err)
		return err
	}

	var req NoteCommentsRequest
	if ok := a.bindJSON(c, &req); !ok {
		return nil
	}

	var resp NoteCommentsResponse

	opts := httpClient.RequestOptions{
		Method:   http.MethodPost,
		URL:      joinURL(a.config.BaseUrlNotes, a.config.V1NoteComment),
		Headers:  uc.headers,
		Body:     &req,
		Response: &resp,
	}

	httpRes, err := a.doUpstream(c, opts, "note_comments")
	if err != nil {
		if httpRes != nil {
			return common.UpstreamError(c, httpRes.StatusCode, http.StatusBadGateway, err.Error())
		}
		return common.WriteServiceUnavailable(c, msgNoteFlowUnavailable)
	}

	return c.JSON(httpRes.StatusCode, resp)
}
