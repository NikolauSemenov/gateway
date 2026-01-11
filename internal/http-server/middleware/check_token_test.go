package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"gateway/external/httpClient"
	"gateway/internal/config"
	"gateway/internal/http-server/common"

	"github.com/labstack/echo/v4"
)

func newTestAPIClient(sizeLimit int64) *httpClient.APIClient {
	cfg := &config.Config{
		HTTPServer: config.HTTPServer{
			Timeout:   2 * time.Second,
			SizeLimit: sizeLimit,
		},
	}
	return httpClient.NewAPIClient(cfg)
}

func TestJWTAuthMiddleware_MissingToken_Returns401(t *testing.T) {
	e := echo.New()

	authSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("unexpected call to auth service: %s %s", r.Method, r.URL.String())
	}))
	t.Cleanup(authSrv.Close)

	var client Doer = newTestAPIClient(1 << 20)

	h := JWTAuthMiddleware(client, authSrv.URL)(func(c echo.Context) error {
		return c.NoContent(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/note-flow/notes", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := h(c); err != nil {
		t.Fatalf("unexpected handler error: %v", err)
	}

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d (body=%q)", http.StatusUnauthorized, rec.Code, rec.Body.String())
	}

	var m common.ErrorResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &m); err != nil {
		t.Fatalf("expected JSON error body, unmarshal err: %v; raw=%q", err, rec.Body.String())
	}
	if m.Detail != "Missing token" {
		t.Fatalf("expected detail %q, got %q", "Missing token", m.Detail)
	}
}

func TestJWTAuthMiddleware_VerifyCalledWithAuthorizationHeader_InjectsUserID(t *testing.T) {
	e := echo.New()

	const token = "Bearer abc"
	var got struct {
		method string
		path   string
		auth   string
	}

	authSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got.method = r.Method
		got.path = r.URL.Path
		got.auth = r.Header.Get("Authorization")

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"user_id":"u123"}`))
	}))
	t.Cleanup(authSrv.Close)

	var client Doer = newTestAPIClient(1 << 20)

	nextCalled := false
	h := JWTAuthMiddleware(client, authSrv.URL)(func(c echo.Context) error {
		nextCalled = true
		v, _ := c.Get(UserIDContextKey).(string)
		if v != "u123" {
			t.Fatalf("expected context %q to be %q, got %q", UserIDContextKey, "u123", v)
		}
		return c.NoContent(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/note-flow/notes", nil)
	req.Header.Set("Authorization", token)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := h(c); err != nil {
		t.Fatalf("unexpected handler error: %v", err)
	}

	if !nextCalled {
		t.Fatalf("expected next handler to be called")
	}

	if got.method != http.MethodGet {
		t.Fatalf("expected outbound method GET, got %q", got.method)
	}
	if got.path != "/v1/verify" {
		t.Fatalf("expected outbound path %q, got %q", "/v1/verify", got.path)
	}
	if got.auth != token {
		t.Fatalf("expected outbound Authorization %q, got %q", token, got.auth)
	}

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d (body=%q)", http.StatusOK, rec.Code, rec.Body.String())
	}
}

func TestJWTAuthMiddleware_VerifyReturnsHTTPError_PropagatesStatusAndMessage(t *testing.T) {
	e := echo.New()

	authSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte("unauthorized"))
	}))
	t.Cleanup(authSrv.Close)

	var client Doer = newTestAPIClient(1 << 20)

	h := JWTAuthMiddleware(client, authSrv.URL)(func(c echo.Context) error {
		t.Fatalf("next should not be called on verify error")
		return nil
	})

	req := httptest.NewRequest(http.MethodGet, "/note-flow/notes", nil)
	req.Header.Set("Authorization", "Bearer bad")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := h(c); err != nil {
		t.Fatalf("unexpected handler error: %v", err)
	}

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d (body=%q)", http.StatusUnauthorized, rec.Code, rec.Body.String())
	}
	var m common.ErrorResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &m); err != nil {
		t.Fatalf("expected JSON error body, unmarshal err: %v; raw=%q", err, rec.Body.String())
	}
	if m.Detail != "unauthorized" {
		t.Fatalf("expected detail %q, got %q", "unauthorized", m.Detail)
	}
}

func TestJWTAuthMiddleware_AuthServerUnavailable_Returns503(t *testing.T) {
	e := echo.New()

	// Create a server and close it immediately to force connection failure.
	authSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	authSrv.Close()

	// Force request to be made quickly; if transport hangs, tests may take long.
	var client Doer = newTestAPIClient(1 << 20)

	h := JWTAuthMiddleware(client, authSrv.URL)(func(c echo.Context) error {
		t.Fatalf("next should not be called when auth server is unavailable")
		return nil
	})

	req := httptest.NewRequest(http.MethodGet, "/note-flow/notes", nil)
	req = req.WithContext(context.Background())
	req.Header.Set("Authorization", "Bearer any")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := h(c); err != nil {
		t.Fatalf("unexpected handler error: %v", err)
	}

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected status %d, got %d (body=%q)", http.StatusServiceUnavailable, rec.Code, rec.Body.String())
	}
	var m common.ErrorResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &m); err != nil {
		t.Fatalf("expected JSON error body, unmarshal err: %v; raw=%q", err, rec.Body.String())
	}
	if m.Detail != "Auth service unavailable" {
		t.Fatalf("expected detail %q, got %q", "Auth service unavailable", m.Detail)
	}
}

func TestJWTAuthMiddleware_EmptyUserID_Returns401InvalidUser(t *testing.T) {
	e := echo.New()

	authSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"user_id":""}`))
	}))
	t.Cleanup(authSrv.Close)

	var client Doer = newTestAPIClient(1 << 20)

	h := JWTAuthMiddleware(client, authSrv.URL)(func(c echo.Context) error {
		t.Fatalf("next should not be called when user_id is empty")
		return nil
	})

	req := httptest.NewRequest(http.MethodGet, "/note-flow/notes", nil)
	req.Header.Set("Authorization", "Bearer any")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := h(c); err != nil {
		t.Fatalf("unexpected handler error: %v", err)
	}

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d (body=%q)", http.StatusUnauthorized, rec.Code, rec.Body.String())
	}

	var m common.ErrorResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &m); err != nil {
		t.Fatalf("expected JSON error body, unmarshal err: %v; raw=%q", err, rec.Body.String())
	}
	if m.Detail != "Invalid user" {
		t.Fatalf("expected detail %q, got %q", "Invalid user", m.Detail)
	}
}
