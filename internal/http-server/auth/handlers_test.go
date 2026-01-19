package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"gateway/external/httpClient"
	"gateway/internal/config"
	"gateway/internal/http-server/common"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
)

type mockDoer struct {
	t *testing.T

	makeRequestFn func(ctx context.Context, r httpClient.RequestOptions) (*http.Response, error)

	calls []httpClient.RequestOptions
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
		t.Fatalf("expected at least one call, got 0")
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

func decodeEchoJSONBody(t *testing.T, rec *httptest.ResponseRecorder, dst any) {
	t.Helper()
	if rec.Body.Len() == 0 {
		t.Fatalf("expected non-empty body to decode, got empty")
	}
	if err := json.Unmarshal(rec.Body.Bytes(), dst); err != nil {
		t.Fatalf("failed to unmarshal response body: %v; raw=%q", err, rec.Body.String())
	}
}

func decodeErrorDetail(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	var out common.ErrorResponse
	decodeEchoJSONBody(t, rec, &out)
	return out.Detail
}

func TestAuth_SendEmailCode_BindError_Returns400(t *testing.T) {
	cfg := &config.Config{
		AuthConfig: config.AuthConfig{
			BaseUrl:         "http://auth",
			V1SendEmailCode: "/v1/send-email-code",
		},
	}
	m := &mockDoer{t: t, makeRequestFn: func(ctx context.Context, r httpClient.RequestOptions) (*http.Response, error) {
		t.Fatalf("MakeRequest should not be called on bind error")
		return nil, nil
	}}

	h := NewAuth(cfg, m, nil, nil)

	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/auth/send-email-code", bytes.NewReader([]byte("{")))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := h.SendEmailCode(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected %d, got %d (body=%q)", http.StatusBadRequest, rec.Code, rec.Body.String())
	}
	if got := decodeErrorDetail(t, rec); got != "Invalid request body" {
		t.Fatalf("expected detail %q, got %q", "Invalid request body", got)
	}
}

func TestAuth_SendEmailCode_Success_ProxiesAndReturnsMessage(t *testing.T) {
	cfg := &config.Config{
		AuthConfig: config.AuthConfig{
			BaseUrl:         "http://auth.local",
			V1SendEmailCode: "/v1/send-email-code",
		},
	}

	m := &mockDoer{
		t: t,
		makeRequestFn: func(ctx context.Context, r httpClient.RequestOptions) (*http.Response, error) {
			return httpResponse(http.StatusOK, ""), nil
		},
	}

	h := NewAuth(cfg, m, nil, nil)

	_, c, rec := newEchoJSONContext(http.MethodPost, "/gateway/api/v1/auth/send-email-code", `{"email":"a@b.c"}`)

	if err := h.SendEmailCode(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	call := m.lastCall(t)
	if call.Method != http.MethodPost {
		t.Fatalf("expected method POST, got %q", call.Method)
	}
	if call.URL != "http://auth.local/v1/send-email-code" {
		t.Fatalf("expected url %q, got %q", "http://auth.local/v1/send-email-code", call.URL)
	}
	if call.Response != nil {
		t.Fatalf("expected Response nil for SendEmailCode, got %T", call.Response)
	}

	bodyMap, ok := call.Body.(SendEmailCodeRequest)
	if !ok {
		// Bind fills struct by value; handler passes req (value).
		t.Fatalf("expected Body to be SendEmailCodeRequest, got %T", call.Body)
	}
	if bodyMap.Email != "a@b.c" {
		t.Fatalf("expected email %q, got %q", "a@b.c", bodyMap.Email)
	}

	if rec.Code != http.StatusOK {
		t.Fatalf("expected %d, got %d (body=%q)", http.StatusOK, rec.Code, rec.Body.String())
	}

	var out string
	decodeEchoJSONBody(t, rec, &out)
	if out != "Email verification code sent" {
		t.Fatalf("unexpected response message: %q", out)
	}
}

func TestAuth_SendEmailCode_UpstreamErrorWithResponse_ReturnsUpstreamStatusAndErrorBody(t *testing.T) {
	cfg := &config.Config{
		AuthConfig: config.AuthConfig{
			BaseUrl:         "http://auth.local",
			V1SendEmailCode: "/v1/send-email-code",
		},
	}

	m := &mockDoer{
		t: t,
		makeRequestFn: func(ctx context.Context, r httpClient.RequestOptions) (*http.Response, error) {
			return httpResponse(http.StatusUnprocessableEntity, "validation failed"), errors.New("validation failed")
		},
	}

	h := NewAuth(cfg, m, nil, nil)

	_, c, rec := newEchoJSONContext(http.MethodPost, "/gateway/api/v1/auth/send-email-code", `{"email":"a@b.c"}`)

	if err := h.SendEmailCode(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected %d, got %d (body=%q)", http.StatusUnprocessableEntity, rec.Code, rec.Body.String())
	}
	if got := decodeErrorDetail(t, rec); got != "validation failed" {
		t.Fatalf("expected detail %q, got %q", "validation failed", got)
	}
}

func TestAuth_SendEmailCode_UpstreamErrorNoResponse_Returns503(t *testing.T) {
	cfg := &config.Config{
		AuthConfig: config.AuthConfig{
			BaseUrl:         "http://auth.local",
			V1SendEmailCode: "/v1/send-email-code",
		},
	}

	m := &mockDoer{
		t: t,
		makeRequestFn: func(ctx context.Context, r httpClient.RequestOptions) (*http.Response, error) {
			return nil, errors.New("dial tcp: refused")
		},
	}

	h := NewAuth(cfg, m, nil, nil)

	_, c, rec := newEchoJSONContext(http.MethodPost, "/gateway/api/v1/auth/send-email-code", `{"email":"a@b.c"}`)

	if err := h.SendEmailCode(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected %d, got %d (body=%q)", http.StatusServiceUnavailable, rec.Code, rec.Body.String())
	}
	if got := decodeErrorDetail(t, rec); got != "Auth service unavailable" {
		t.Fatalf("expected detail %q, got %q", "Auth service unavailable", got)
	}
}

func TestAuth_VerifyEmailCode_Success_DecodesResponseAndReturnsIt(t *testing.T) {
	cfg := &config.Config{
		AuthConfig: config.AuthConfig{
			BaseUrl:       "http://auth.local",
			V1VerifyEmail: "/v1/verify-email-code",
		},
	}

	m := &mockDoer{
		t: t,
		makeRequestFn: func(ctx context.Context, r httpClient.RequestOptions) (*http.Response, error) {
			// Ensure handler asked to decode into *VerifyEmailCodeResponse.
			dst, ok := r.Response.(*VerifyEmailCodeResponse)
			if !ok {
				t.Fatalf("expected Response to be *VerifyEmailCodeResponse, got %T", r.Response)
			}
			*dst = VerifyEmailCodeResponse{
				AccessToken: "tkn",
				TokenType:   "Bearer",
				UserName:    "John",
				LastName:    "Doe",
				UserID:      123,
			}
			return httpResponse(http.StatusOK, ""), nil
		},
	}

	h := NewAuth(cfg, m, nil, nil)

	_, c, rec := newEchoJSONContext(http.MethodPut, "/gateway/api/v1/auth/verify-email-code", `{"email":"a@b.c","code":"1234"}`)

	if err := h.VerifyEmailCode(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	call := m.lastCall(t)
	if call.Method != http.MethodPut {
		t.Fatalf("expected method PUT, got %q", call.Method)
	}
	if call.URL != "http://auth.local/v1/verify-email-code" {
		t.Fatalf("expected url %q, got %q", "http://auth.local/v1/verify-email-code", call.URL)
	}

	if rec.Code != http.StatusOK {
		t.Fatalf("expected %d, got %d (body=%q)", http.StatusOK, rec.Code, rec.Body.String())
	}

	var out VerifyEmailCodeResponse
	decodeEchoJSONBody(t, rec, &out)
	if out.AccessToken != "tkn" || out.TokenType != "Bearer" || out.UserName != "John" || out.LastName != "Doe" || out.UserID != 123 {
		t.Fatalf("unexpected response: %+v", out)
	}
}

func TestAuth_Login_Success_DecodesResponseAndReturnsIt(t *testing.T) {
	cfg := &config.Config{
		AuthConfig: config.AuthConfig{
			BaseUrl:    "http://auth.local",
			V1Login:    "/v1/login",
			V1Register: "/v1/register",
		},
	}

	m := &mockDoer{
		t: t,
		makeRequestFn: func(ctx context.Context, r httpClient.RequestOptions) (*http.Response, error) {
			dst, ok := r.Response.(*LoginResponse)
			if !ok {
				t.Fatalf("expected Response to be *LoginResponse, got %T", r.Response)
			}
			*dst = LoginResponse{
				AccessToken: "tkn",
				TokenType:   "Bearer",
				UserName:    "John",
				LastName:    "Doe",
				UserID:      456,
			}
			return httpResponse(http.StatusOK, ""), nil
		},
	}

	h := NewAuth(cfg, m, nil, nil)

	_, c, rec := newEchoJSONContext(http.MethodPost, "/gateway/api/v1/auth/login", `{"login":"u","password":"p"}`)

	if err := h.Login(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	call := m.lastCall(t)
	if call.Method != http.MethodPost {
		t.Fatalf("expected method POST, got %q", call.Method)
	}
	if call.URL != "http://auth.local/v1/login" {
		t.Fatalf("expected url %q, got %q", "http://auth.local/v1/login", call.URL)
	}

	var out LoginResponse
	decodeEchoJSONBody(t, rec, &out)
	if out.AccessToken != "tkn" || out.TokenType != "Bearer" || out.UserName != "John" || out.LastName != "Doe" || out.UserID != 456 {
		t.Fatalf("unexpected response: %+v", out)
	}
}

func TestAuth_Verify_SetsAuthorizationHeaderAndReturnsStatus(t *testing.T) {
	cfg := &config.Config{
		AuthConfig: config.AuthConfig{
			BaseUrl:    "http://auth.local",
			V1Verify:   "/v1/verify",
			V1Logout:   "/v1/logout",
			V1Login:    "/v1/login",
			V1Register: "/v1/register",
		},
	}

	const token = "Bearer abc"

	m := &mockDoer{
		t: t,
		makeRequestFn: func(ctx context.Context, r httpClient.RequestOptions) (*http.Response, error) {
			if r.Method != http.MethodGet {
				t.Fatalf("expected GET, got %q", r.Method)
			}
			if r.URL != "http://auth.local/v1/verify" {
				t.Fatalf("expected url %q, got %q", "http://auth.local/v1/verify", r.URL)
			}
			if r.Headers == nil || r.Headers.Get("Authorization") != token {
				got := ""
				if r.Headers != nil {
					got = r.Headers.Get("Authorization")
				}
				t.Fatalf("expected Authorization header %q, got %q", token, got)
			}
			return httpResponse(http.StatusOK, ""), nil
		},
	}

	h := NewAuth(cfg, m, nil, nil)

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/gateway/api/v1/auth/verify", nil)
	req.Header.Set("Authorization", token)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := h.Verify(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected %d, got %d (body=%q)", http.StatusOK, rec.Code, rec.Body.String())
	}
}

func TestAuth_Register_Success_ReturnsMessage(t *testing.T) {
	cfg := &config.Config{
		AuthConfig: config.AuthConfig{
			BaseUrl:    "http://auth.local",
			V1Register: "/v1/register",
		},
	}

	m := &mockDoer{
		t: t,
		makeRequestFn: func(ctx context.Context, r httpClient.RequestOptions) (*http.Response, error) {
			return httpResponse(http.StatusCreated, ""), nil
		},
	}

	h := NewAuth(cfg, m, nil, nil)

	_, c, rec := newEchoJSONContext(http.MethodPost, "/gateway/api/v1/auth/register", `{"login":"u","user_name":"n","last_name":"l","password":"p","email":"e@e.e"}`)

	if err := h.Register(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected %d, got %d (body=%q)", http.StatusCreated, rec.Code, rec.Body.String())
	}

	var out string
	decodeEchoJSONBody(t, rec, &out)
	if out != "Registration request sent" {
		t.Fatalf("unexpected response message: %q", out)
	}
}

// Logout should call DELETE to /v1/logout.
func TestAuth_Logout_CallsLogoutEndpoint(t *testing.T) {
	cfg := &config.Config{
		AuthConfig: config.AuthConfig{
			BaseUrl:  "http://auth.local",
			V1Verify: "/v1/verify",
			V1Logout: "/v1/logout",
		},
	}

	const token = "Bearer abc"

	m := &mockDoer{
		t: t,
		makeRequestFn: func(ctx context.Context, r httpClient.RequestOptions) (*http.Response, error) {
			if r.Method != http.MethodDelete {
				t.Fatalf("expected Logout to use method %q, got %q", http.MethodDelete, r.Method)
			}
			if r.URL != "http://auth.local/v1/logout" {
				t.Fatalf("expected Logout to call %q, got %q", "http://auth.local/v1/logout", r.URL)
			}
			if r.Headers == nil || r.Headers.Get("Authorization") != token {
				got := ""
				if r.Headers != nil {
					got = r.Headers.Get("Authorization")
				}
				t.Fatalf("expected Authorization header %q, got %q", token, got)
			}
			return httpResponse(http.StatusNoContent, ""), nil
		},
	}

	h := NewAuth(cfg, m, nil, nil)

	e := echo.New()
	req := httptest.NewRequest(http.MethodDelete, "/gateway/api/v1/auth/logout", nil)
	req.Header.Set("Authorization", token)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := h.Logout(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected %d, got %d (body=%q)", http.StatusNoContent, rec.Code, rec.Body.String())
	}
}
