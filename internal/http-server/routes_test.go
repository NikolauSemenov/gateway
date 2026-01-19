package http_server

import (
	"context"
	"gateway/external/httpClient"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
)

type handlersAuthStub struct{}

func (handlersAuthStub) SendEmailCode(c echo.Context) error   { return c.NoContent(http.StatusTeapot) }
func (handlersAuthStub) Register(c echo.Context) error        { return c.NoContent(http.StatusTeapot) }
func (handlersAuthStub) Login(c echo.Context) error           { return c.NoContent(http.StatusTeapot) }
func (handlersAuthStub) VerifyEmailCode(c echo.Context) error { return c.NoContent(http.StatusTeapot) }
func (handlersAuthStub) Logout(c echo.Context) error          { return c.NoContent(http.StatusTeapot) }

type handlersNoteFlowStub struct{}

func (handlersNoteFlowStub) Notes(c echo.Context) error        { return c.NoContent(http.StatusTeapot) }
func (handlersNoteFlowStub) Note(c echo.Context) error         { return c.NoContent(http.StatusTeapot) }
func (handlersNoteFlowStub) NoteActive(c echo.Context) error   { return c.NoContent(http.StatusTeapot) }
func (handlersNoteFlowStub) CreateNote(c echo.Context) error   { return c.NoContent(http.StatusTeapot) }
func (handlersNoteFlowStub) NoteLike(c echo.Context) error     { return c.NoContent(http.StatusTeapot) }
func (handlersNoteFlowStub) NoteComments(c echo.Context) error { return c.NoContent(http.StatusTeapot) }
func (handlersNoteFlowStub) DeleteNote(c echo.Context) error   { return c.NoContent(http.StatusTeapot) }
func (handlersNoteFlowStub) DeleteComment(c echo.Context) error {
	return c.NoContent(http.StatusTeapot)
}

func TestRegisterRoutes_RegistersHealthEndpoint(t *testing.T) {
	e := echo.New()

	var client interface {
		MakeRequest(ctx context.Context, r httpClient.RequestOptions) (*http.Response, error)
	} = nil

	RegisterRoutes(
		"/gateway/api/v1",
		e,
		handlersAuthStub{},
		handlersNoteFlowStub{},
		client,
		"http://auth.local",
		nil, // cfg
		nil, // rdb
		nil, // logger
	)

	req := httptest.NewRequest(http.MethodGet, "/gateway/api/v1/health", nil)
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d (body=%q)", http.StatusOK, rec.Code, rec.Body.String())
	}
	if rec.Body.Len() != 0 {
		t.Fatalf("expected empty body for NoContent, got %q", rec.Body.String())
	}
}

func TestRegisterRoutes_RegistersAuthAndNoteFlowRoutes(t *testing.T) {
	e := echo.New()

	var client interface {
		MakeRequest(ctx context.Context, r httpClient.RequestOptions) (*http.Response, error)
	} = nil

	RegisterRoutes(
		"/gateway/api/v1",
		e,
		handlersAuthStub{},
		handlersNoteFlowStub{},
		client,
		"http://auth.local",
		nil, // cfg
		nil, // rdb
		nil, // logger
	)

	routes := e.Routes()

	need := map[string]struct{}{
		"GET /gateway/api/v1/health":                                {},
		"POST /gateway/api/v1/auth/send-email-code":                 {},
		"PUT /gateway/api/v1/auth/verify-email-code":                {},
		"POST /gateway/api/v1/auth/register":                        {},
		"POST /gateway/api/v1/auth/login":                           {},
		"DELETE /gateway/api/v1/auth/logout":                        {},
		"GET /gateway/api/v1/note-flow/notes":                       {},
		"GET /gateway/api/v1/note-flow/note/:note_id":               {},
		"PUT /gateway/api/v1/note-flow/note/active":                 {},
		"POST /gateway/api/v1/note-flow/note":                       {},
		"POST /gateway/api/v1/note-flow/note/like":                  {},
		"POST /gateway/api/v1/note-flow/note/comment":               {},
		"DELETE /gateway/api/v1/note-flow/note/:note_id":            {},
		"DELETE /gateway/api/v1/note-flow/note/comment/:comment_id": {},
	}

	for _, r := range routes {
		key := r.Method + " " + r.Path
		delete(need, key)
	}

	if len(need) != 0 {
		missing := make([]string, 0, len(need))
		for k := range need {
			missing = append(missing, k)
		}
		t.Fatalf("missing expected routes: %v", missing)
	}
}
