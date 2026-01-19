package note_flow

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"gateway/external/httpClient"
	"gateway/internal/config"
	"gateway/internal/http-server/common"
	"gateway/internal/http-server/middleware"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
)

type mockDoer struct {
	t *testing.T

	makeRequestFn func(ctx context.Context, r httpClient.RequestOptions) (*http.Response, error)
	calls         []httpClient.RequestOptions
}

func (m *mockDoer) MakeRequest(ctx context.Context, r httpClient.RequestOptions) (*http.Response, error) {
	if m.t != nil {
		m.t.Helper()
	}
	m.calls = append(m.calls, r)
	if m.makeRequestFn == nil {
		return nil, errors.New("mockDoer: makeRequestFn is nil")
	}
	return m.makeRequestFn(ctx, r)
}

func (m *mockDoer) lastCall(t *testing.T) httpClient.RequestOptions {
	t.Helper()
	if len(m.calls) == 0 {
		t.Fatalf("expected at least one MakeRequest call, got 0")
	}
	return m.calls[len(m.calls)-1]
}

func httpResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func newEchoJSONContext(method, target string, body string) (*echo.Echo, echo.Context, *httptest.ResponseRecorder) {
	e := echo.New()
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	return e, e.NewContext(req, rec), rec
}

func decodeEchoJSONBody[T any](t *testing.T, rec *httptest.ResponseRecorder) T {
	t.Helper()

	var out T
	if rec.Body.Len() == 0 {
		t.Fatalf("expected non-empty body to decode, got empty")
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("failed to unmarshal response body: %v; raw=%q", err, rec.Body.String())
	}
	return out
}

func decodeErrorDetail(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()

	var out common.ErrorResponse
	if rec.Body.Len() == 0 {
		t.Fatalf("expected non-empty body to decode, got empty")
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("failed to unmarshal error response: %v; raw=%q", err, rec.Body.String())
	}
	return out.Detail
}

func newHandlerWithMock(t *testing.T, cfg *config.Config, m *mockDoer) *NoteFlow {
	t.Helper()
	return NewNoteFlow(cfg, m, nil)
}

func setUserInContext(c echo.Context, userID string) {
	c.Set(middleware.UserIDContextKey, userID)
}

func TestNoteFlow_Notes_Success_ProxiesWithHeadersAndUserQuery(t *testing.T) {
	cfg := &config.Config{
		NoteFlowConfig: config.NoteFlowConfig{
			BaseUrlNotes: "http://note-flow.local",
			V1Notes:      "/v1/notes",
		},
	}

	const userID = "u123"
	const token = "Bearer abc"

	m := &mockDoer{
		t: t,
		makeRequestFn: func(ctx context.Context, r httpClient.RequestOptions) (*http.Response, error) {
			if r.Method != http.MethodGet {
				t.Fatalf("expected method GET, got %q", r.Method)
			}
			if r.URL != "http://note-flow.local/v1/notes" {
				t.Fatalf("expected URL %q, got %q", "http://note-flow.local/v1/notes", r.URL)
			}
			if r.Headers == nil {
				t.Fatalf("expected headers, got nil")
			}
			if got := r.Headers.Get("X-User-ID"); got != userID {
				t.Fatalf("expected X-User-ID %q, got %q", userID, got)
			}
			if got := r.Headers.Get("Authorization"); got != token {
				t.Fatalf("expected Authorization %q, got %q", token, got)
			}
			if r.Query == nil || r.Query.Get("user_id") != userID {
				got := ""
				if r.Query != nil {
					got = r.Query.Get("user_id")
				}
				t.Fatalf("expected query user_id %q, got %q", userID, got)
			}

			dst, ok := r.Response.(*[]NotesResponse)
			if !ok {
				t.Fatalf("expected Response to be *[]NotesResponse, got %T", r.Response)
			}
			*dst = []NotesResponse{
				{Id: 1, Title: "t1", Content: "c1", UserId: 11, UserFullName: "FN", CreatedAt: "now", Likes: 0, Comments: nil},
			}

			return httpResponse(http.StatusOK, ""), nil
		},
	}

	h := newHandlerWithMock(t, cfg, m)

	_, c, rec := newEchoJSONContext(http.MethodGet, "/gateway/api/v1/note-flow/notes", "")
	c.Request().Header.Set("Authorization", token)
	setUserInContext(c, userID)

	if err := h.Notes(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Fatalf("expected %d, got %d (body=%q)", http.StatusOK, rec.Code, rec.Body.String())
	}

	out := decodeEchoJSONBody[[]NotesResponse](t, rec)
	if len(out) != 1 || out[0].Id != 1 || out[0].Title != "t1" {
		t.Fatalf("unexpected response: %+v", out)
	}
}

func TestNoteFlow_Notes_UpstreamErrorWithResponse_ReturnsUpstreamStatusAndError(t *testing.T) {
	cfg := &config.Config{
		NoteFlowConfig: config.NoteFlowConfig{
			BaseUrlNotes: "http://note-flow.local",
			V1Notes:      "/v1/notes",
		},
	}

	m := &mockDoer{
		t: t,
		makeRequestFn: func(ctx context.Context, r httpClient.RequestOptions) (*http.Response, error) {
			return httpResponse(http.StatusUnprocessableEntity, "validation failed"), errors.New("validation failed")
		},
	}

	h := newHandlerWithMock(t, cfg, m)

	_, c, rec := newEchoJSONContext(http.MethodGet, "/gateway/api/v1/note-flow/notes", "")
	setUserInContext(c, "u123")
	c.Request().Header.Set("Authorization", "Bearer abc")

	if err := h.Notes(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected %d, got %d (body=%q)", http.StatusUnprocessableEntity, rec.Code, rec.Body.String())
	}
	if got := decodeErrorDetail(t, rec); got != "validation failed" {
		t.Fatalf("expected detail %q, got %q", "validation failed", got)
	}
}

func TestNoteFlow_Notes_UpstreamErrorNoResponse_Returns503(t *testing.T) {
	cfg := &config.Config{
		NoteFlowConfig: config.NoteFlowConfig{
			BaseUrlNotes: "http://note-flow.local",
			V1Notes:      "/v1/notes",
		},
	}

	m := &mockDoer{
		t: t,
		makeRequestFn: func(ctx context.Context, r httpClient.RequestOptions) (*http.Response, error) {
			return nil, errors.New("dial tcp: refused")
		},
	}

	h := newHandlerWithMock(t, cfg, m)

	_, c, rec := newEchoJSONContext(http.MethodGet, "/gateway/api/v1/note-flow/notes", "")
	setUserInContext(c, "u123")

	if err := h.Notes(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected %d, got %d (body=%q)", http.StatusServiceUnavailable, rec.Code, rec.Body.String())
	}
	if got := decodeErrorDetail(t, rec); got != "Note-flow service unavailable" {
		t.Fatalf("expected detail %q, got %q", "Note-flow service unavailable", got)
	}
}

func TestNoteFlow_Note_Success_UsesPathParamAndUserQuery(t *testing.T) {
	cfg := &config.Config{
		NoteFlowConfig: config.NoteFlowConfig{
			BaseUrlNotes: "http://note-flow.local",
			V1Note:       "/v1/note",
		},
	}

	const userID = "u123"

	m := &mockDoer{
		t: t,
		makeRequestFn: func(ctx context.Context, r httpClient.RequestOptions) (*http.Response, error) {
			if r.Method != http.MethodGet {
				t.Fatalf("expected GET, got %q", r.Method)
			}
			if r.URL != "http://note-flow.local/v1/note/99" {
				t.Fatalf("expected URL %q, got %q", "http://note-flow.local/v1/note/99", r.URL)
			}
			if r.Query == nil || r.Query.Get("user_id") != userID {
				t.Fatalf("expected query user_id=%q, got %v", userID, r.Query)
			}
			dst, ok := r.Response.(*NotesResponse)
			if !ok {
				t.Fatalf("expected Response *NotesResponse, got %T", r.Response)
			}
			*dst = NotesResponse{Id: 99, Title: "t", Content: "c", UserId: 11}
			return httpResponse(http.StatusOK, ""), nil
		},
	}

	h := newHandlerWithMock(t, cfg, m)

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/gateway/api/v1/note-flow/note/99", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("note_id")
	c.SetParamValues("99")
	setUserInContext(c, userID)

	if err := h.Note(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected %d, got %d (body=%q)", http.StatusOK, rec.Code, rec.Body.String())
	}

	out := decodeEchoJSONBody[NotesResponse](t, rec)
	if out.Id != 99 {
		t.Fatalf("expected id 99, got %+v", out)
	}
}

func TestNoteFlow_NoteActive_Success_BuildsQueryWithIdAndUserId(t *testing.T) {
	cfg := &config.Config{
		NoteFlowConfig: config.NoteFlowConfig{
			BaseUrlNotes: "http://note-flow.local",
			V1NoteActive: "/v1/note/active",
		},
	}

	const userID = "u123"

	m := &mockDoer{
		t: t,
		makeRequestFn: func(ctx context.Context, r httpClient.RequestOptions) (*http.Response, error) {
			if r.Method != http.MethodPut {
				t.Fatalf("expected PUT, got %q", r.Method)
			}
			if r.URL != "http://note-flow.local/v1/note/active" {
				t.Fatalf("expected URL %q, got %q", "http://note-flow.local/v1/note/active", r.URL)
			}
			if r.Query == nil {
				t.Fatalf("expected query, got nil")
			}
			if got := r.Query.Get("user_id"); got != userID {
				t.Fatalf("expected user_id=%q, got %q", userID, got)
			}
			if got := r.Query.Get("id"); got != "777" {
				t.Fatalf("expected id=%q, got %q", "777", got)
			}
			return httpResponse(http.StatusOK, ""), nil
		},
	}

	h := newHandlerWithMock(t, cfg, m)

	e := echo.New()
	req := httptest.NewRequest(http.MethodPut, "/gateway/api/v1/note-flow/note/active?id=777", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	setUserInContext(c, userID)

	if err := h.NoteActive(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected %d, got %d (body=%q)", http.StatusOK, rec.Code, rec.Body.String())
	}
}

func TestNoteFlow_CreateNote_BindError_Returns400(t *testing.T) {
	cfg := &config.Config{
		NoteFlowConfig: config.NoteFlowConfig{
			BaseUrlNotes: "http://note-flow.local",
			V1Note:       "/v1/note",
		},
	}

	m := &mockDoer{
		t: t,
		makeRequestFn: func(ctx context.Context, r httpClient.RequestOptions) (*http.Response, error) {
			t.Fatalf("MakeRequest should not be called on bind error")
			return nil, nil
		},
	}

	h := newHandlerWithMock(t, cfg, m)

	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/gateway/api/v1/note-flow/note", bytes.NewReader([]byte("{")))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	setUserInContext(c, "u123")

	if err := h.CreateNote(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected %d, got %d (body=%q)", http.StatusBadRequest, rec.Code, rec.Body.String())
	}
	if got := decodeErrorDetail(t, rec); got != "Invalid request body" {
		t.Fatalf("expected detail %q, got %q", "Invalid request body", got)
	}
}

func TestNoteFlow_CreateNote_Success_ProxiesBodyAndReturnsResponse(t *testing.T) {
	cfg := &config.Config{
		NoteFlowConfig: config.NoteFlowConfig{
			BaseUrlNotes: "http://note-flow.local",
			V1Note:       "/v1/note",
		},
	}

	const userID = "u123"
	const token = "Bearer abc"

	m := &mockDoer{
		t: t,
		makeRequestFn: func(ctx context.Context, r httpClient.RequestOptions) (*http.Response, error) {
			if r.Method != http.MethodPost {
				t.Fatalf("expected POST, got %q", r.Method)
			}
			if r.URL != "http://note-flow.local/v1/note" {
				t.Fatalf("expected URL %q, got %q", "http://note-flow.local/v1/note", r.URL)
			}
			if r.Headers == nil || r.Headers.Get("X-User-ID") != userID {
				got := ""
				if r.Headers != nil {
					got = r.Headers.Get("X-User-ID")
				}
				t.Fatalf("expected X-User-ID=%q, got %q", userID, got)
			}
			if r.Headers.Get("Authorization") != token {
				t.Fatalf("expected Authorization=%q, got %q", token, r.Headers.Get("Authorization"))
			}
			// Handler passes &req (pointer).
			body, ok := r.Body.(*CreateNoteRequest)
			if !ok {
				t.Fatalf("expected Body *CreateNoteRequest, got %T", r.Body)
			}
			if body.Title != "t" || body.Content != "c" || body.FullName != "fn" {
				t.Fatalf("unexpected body: %+v", body)
			}

			dst, ok := r.Response.(*CreateNoteResponse)
			if !ok {
				t.Fatalf("expected Response *CreateNoteResponse, got %T", r.Response)
			}
			*dst = CreateNoteResponse{Id: 123}

			return httpResponse(http.StatusOK, ""), nil
		},
	}

	h := newHandlerWithMock(t, cfg, m)

	_, c, rec := newEchoJSONContext(http.MethodPost, "/gateway/api/v1/note-flow/note", `{"title":"t","content":"c","full_name":"fn"}`)
	c.Request().Header.Set("Authorization", token)
	setUserInContext(c, userID)

	if err := h.CreateNote(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected %d, got %d (body=%q)", http.StatusOK, rec.Code, rec.Body.String())
	}

	out := decodeEchoJSONBody[CreateNoteResponse](t, rec)
	if out.Id != 123 {
		t.Fatalf("expected id 123, got %+v", out)
	}
}

func TestNoteFlow_NoteLike_BindError_Returns400(t *testing.T) {
	cfg := &config.Config{
		NoteFlowConfig: config.NoteFlowConfig{
			BaseUrlNotes: "http://note-flow.local",
			V1NoteLike:   "/v1/note/like",
		},
	}

	m := &mockDoer{
		t: t,
		makeRequestFn: func(ctx context.Context, r httpClient.RequestOptions) (*http.Response, error) {
			t.Fatalf("MakeRequest should not be called on bind error")
			return nil, nil
		},
	}

	h := newHandlerWithMock(t, cfg, m)

	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/gateway/api/v1/note-flow/note/like", bytes.NewReader([]byte("{")))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	setUserInContext(c, "u123")

	if err := h.NoteLike(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected %d, got %d (body=%q)", http.StatusBadRequest, rec.Code, rec.Body.String())
	}
	if got := decodeErrorDetail(t, rec); got != "Invalid request body" {
		t.Fatalf("expected detail %q, got %q", "Invalid request body", got)
	}
}

func TestNoteFlow_NoteLike_Success_ReturnsLikesCountResponse(t *testing.T) {
	cfg := &config.Config{
		NoteFlowConfig: config.NoteFlowConfig{
			BaseUrlNotes: "http://note-flow.local",
			V1NoteLike:   "/v1/note/like",
		},
	}

	const userID = "u123"

	m := &mockDoer{
		t: t,
		makeRequestFn: func(ctx context.Context, r httpClient.RequestOptions) (*http.Response, error) {
			if r.Method != http.MethodPost {
				t.Fatalf("expected POST, got %q", r.Method)
			}
			if r.URL != "http://note-flow.local/v1/note/like" {
				t.Fatalf("expected URL %q, got %q", "http://note-flow.local/v1/note/like", r.URL)
			}
			body, ok := r.Body.(*LikeNoteRequest)
			if !ok {
				t.Fatalf("expected Body *LikeNoteRequest, got %T", r.Body)
			}
			if body.NoteID != 7 {
				t.Fatalf("expected note_id=7, got %+v", body)
			}

			dst, ok := r.Response.(*NoteLikesCountResponse)
			if !ok {
				t.Fatalf("expected Response *NoteLikesCountResponse, got %T", r.Response)
			}
			*dst = NoteLikesCountResponse{Likes: 10}

			return httpResponse(http.StatusCreated, ""), nil
		},
	}

	h := newHandlerWithMock(t, cfg, m)

	_, c, rec := newEchoJSONContext(http.MethodPost, "/gateway/api/v1/note-flow/note/like", `{"note_id":7}`)
	setUserInContext(c, userID)

	if err := h.NoteLike(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected %d, got %d (body=%q)", http.StatusCreated, rec.Code, rec.Body.String())
	}

	out := decodeEchoJSONBody[NoteLikesCountResponse](t, rec)
	if out.Likes != 10 {
		t.Fatalf("expected likes=10, got %+v", out)
	}
}

func TestNoteFlow_NoteComments_BindError_Returns400(t *testing.T) {
	cfg := &config.Config{
		NoteFlowConfig: config.NoteFlowConfig{
			BaseUrlNotes:  "http://note-flow.local",
			V1NoteComment: "/v1/note/comment",
		},
	}

	m := &mockDoer{
		t: t,
		makeRequestFn: func(ctx context.Context, r httpClient.RequestOptions) (*http.Response, error) {
			t.Fatalf("MakeRequest should not be called on bind error")
			return nil, nil
		},
	}

	h := newHandlerWithMock(t, cfg, m)

	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/gateway/api/v1/note-flow/note/comment", bytes.NewReader([]byte("{")))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	setUserInContext(c, "u123")

	if err := h.NoteComments(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected %d, got %d (body=%q)", http.StatusBadRequest, rec.Code, rec.Body.String())
	}
	if got := decodeErrorDetail(t, rec); got != "Invalid request body" {
		t.Fatalf("expected detail %q, got %q", "Invalid request body", got)
	}
}

func TestNoteFlow_NoteComments_Success_ReturnsResponse(t *testing.T) {
	cfg := &config.Config{
		NoteFlowConfig: config.NoteFlowConfig{
			BaseUrlNotes:  "http://note-flow.local",
			V1NoteComment: "/v1/note/comment",
		},
	}

	const userID = "u123"

	m := &mockDoer{
		t: t,
		makeRequestFn: func(ctx context.Context, r httpClient.RequestOptions) (*http.Response, error) {
			if r.Method != http.MethodPost {
				t.Fatalf("expected POST, got %q", r.Method)
			}
			if r.URL != "http://note-flow.local/v1/note/comment" {
				t.Fatalf("expected URL %q, got %q", "http://note-flow.local/v1/note/comment", r.URL)
			}
			body, ok := r.Body.(*NoteCommentsRequest)
			if !ok {
				t.Fatalf("expected Body *NoteCommentsRequest, got %T", r.Body)
			}
			if body.NoteID != 5 || body.FullName != "FN" || body.Text != "hello" {
				t.Fatalf("unexpected body: %+v", body)
			}

			dst, ok := r.Response.(*NoteCommentsResponse)
			if !ok {
				t.Fatalf("expected Response *NoteCommentsResponse, got %T", r.Response)
			}
			*dst = NoteCommentsResponse{NoteID: 5}

			return httpResponse(http.StatusOK, ""), nil
		},
	}

	h := newHandlerWithMock(t, cfg, m)

	_, c, rec := newEchoJSONContext(http.MethodPost, "/gateway/api/v1/note-flow/note/comment", `{"note_id":5,"user_full_name":"FN","text":"hello"}`)
	setUserInContext(c, userID)

	if err := h.NoteComments(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected %d, got %d (body=%q)", http.StatusOK, rec.Code, rec.Body.String())
	}

	out := decodeEchoJSONBody[NoteCommentsResponse](t, rec)
	if out.NoteID != 5 {
		t.Fatalf("expected NoteID=5, got %+v", out)
	}
}

func TestNoteFlow_BuildUserContext_AddsUserIDQueryHelper(t *testing.T) {
	uc := &userContext{id: "u123"}
	base := url.Values{}
	base.Add("a", "1")
	base.Add("a", "2")

	out := uc.withUserQuery(base)
	if out.Get("user_id") != "u123" {
		t.Fatalf("expected user_id=u123, got %q", out.Get("user_id"))
	}
	if got := out["a"]; len(got) != 2 || got[0] != "1" || got[1] != "2" {
		t.Fatalf("expected a=[1 2], got %v", got)
	}
}
