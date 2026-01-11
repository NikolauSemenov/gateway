package note_flow

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"gateway/internal/config"
	"gateway/internal/http-server/middleware"

	"github.com/labstack/echo/v4"
)

func newTestHandler() *NoteFlow {
	return NewNoteFlow(&config.Config{}, nil, nil)
}

func TestNoteFlow_userID_Success(t *testing.T) {
	h := newTestHandler()

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	c.Set(middleware.UserIDContextKey, "u123")

	userID, err := h.userID(c)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if userID != "u123" {
		t.Fatalf("expected userID %q, got %q", "u123", userID)
	}
}

func TestNoteFlow_userID_MissingOrWrongType_ReturnsUnauthorized(t *testing.T) {
	h := newTestHandler()

	t.Run("MissingKey", func(t *testing.T) {
		e := echo.New()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		_, err := h.userID(c)
		if err == nil {
			t.Fatalf("expected error, got nil")
		}
		he, ok := err.(*echo.HTTPError)
		if !ok {
			t.Fatalf("expected *echo.HTTPError, got %T", err)
		}
		if he.Code != http.StatusUnauthorized {
			t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, he.Code)
		}
	})

	t.Run("WrongType", func(t *testing.T) {
		e := echo.New()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		c.Set(middleware.UserIDContextKey, 123)

		_, err := h.userID(c)
		if err == nil {
			t.Fatalf("expected error, got nil")
		}
		he, ok := err.(*echo.HTTPError)
		if !ok {
			t.Fatalf("expected *echo.HTTPError, got %T", err)
		}
		if he.Code != http.StatusUnauthorized {
			t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, he.Code)
		}
	})

	t.Run("EmptyString", func(t *testing.T) {
		e := echo.New()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		c.Set(middleware.UserIDContextKey, "")

		_, err := h.userID(c)
		if err == nil {
			t.Fatalf("expected error, got nil")
		}
		he, ok := err.(*echo.HTTPError)
		if !ok {
			t.Fatalf("expected *echo.HTTPError, got %T", err)
		}
		if he.Code != http.StatusUnauthorized {
			t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, he.Code)
		}
	})
}

func TestNoteFlow_buildUserContext_BuildsHeadersAndCopiesAuthorization(t *testing.T) {
	h := newTestHandler()

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer token")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	c.Set(middleware.UserIDContextKey, "u123")

	uc, err := h.buildUserContext(c)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if uc == nil {
		t.Fatalf("expected non-nil userContext")
	}
	if uc.id != "u123" {
		t.Fatalf("expected uc.id %q, got %q", "u123", uc.id)
	}

	if got := uc.headers.Get("X-User-ID"); got != "u123" {
		t.Fatalf("expected X-User-ID %q, got %q", "u123", got)
	}
	if got := uc.headers.Get("Authorization"); got != "Bearer token" {
		t.Fatalf("expected Authorization %q, got %q", "Bearer token", got)
	}
}

func TestNoteFlow_buildUserContext_NoAuthorizationHeader_DoesNotSetAuthorization(t *testing.T) {
	h := newTestHandler()

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	c.Set(middleware.UserIDContextKey, "u123")

	uc, err := h.buildUserContext(c)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if got := uc.headers.Get("Authorization"); got != "" {
		t.Fatalf("expected Authorization to be empty, got %q", got)
	}
	if got := uc.headers.Get("X-User-ID"); got != "u123" {
		t.Fatalf("expected X-User-ID %q, got %q", "u123", got)
	}
}

func TestUserContext_withUserQuery_AddsUserIDAndPreservesExistingParams(t *testing.T) {
	uc := &userContext{id: "u123"}

	base := url.Values{}
	base.Add("a", "1")
	base.Add("a", "2")
	base.Set("user_id", "will-be-overwritten")

	out := uc.withUserQuery(base)

	if out.Get("user_id") != "u123" {
		t.Fatalf("expected user_id %q, got %q", "u123", out.Get("user_id"))
	}
	if got := out["a"]; len(got) != 2 || got[0] != "1" || got[1] != "2" {
		t.Fatalf("expected a=[1 2], got %v", got)
	}
}

func TestUserContext_withUserQuery_NilBase_CreatesValues(t *testing.T) {
	uc := &userContext{id: "u123"}

	out := uc.withUserQuery(nil)
	if out == nil {
		t.Fatalf("expected non-nil url.Values")
	}
	if out.Get("user_id") != "u123" {
		t.Fatalf("expected user_id %q, got %q", "u123", out.Get("user_id"))
	}
}
