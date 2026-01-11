package httpClient

import (
	"context"
	"encoding/json"
	"errors"
	"gateway/internal/config"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func newTestClient(t *testing.T, sizeLimit int64) *APIClient {
	t.Helper()

	cfg := &config.Config{
		HTTPServer: config.HTTPServer{
			Timeout:   2 * time.Second,
			SizeLimit: sizeLimit,
		},
	}
	return NewAPIClient(cfg)
}

func TestAPIClient_do_ValidatesEmptyURLAndMethod(t *testing.T) {
	t.Parallel()

	c := newTestClient(t, 1<<20)

	_, err := c.do(context.Background(), RequestOptions{
		Method: "GET",
		URL:    "",
	})
	if !errors.Is(err, ErrEmptyURL) {
		t.Fatalf("expected ErrEmptyURL, got %v", err)
	}

	_, err = c.do(context.Background(), RequestOptions{
		Method: "",
		URL:    "http://example.com",
	})
	if !errors.Is(err, ErrEmptyMethod) {
		t.Fatalf("expected ErrEmptyMethod, got %v", err)
	}
}

func TestAPIClient_do_BuildsQueryAndClonesHeadersAndSetsContentType(t *testing.T) {
	t.Parallel()

	var got struct {
		Method      string
		Path        string
		RawQuery    string
		Headers     http.Header
		BodyBytes   []byte
		ContentType string
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got.Method = r.Method
		got.Path = r.URL.Path
		got.RawQuery = r.URL.RawQuery
		got.Headers = r.Header.Clone()
		got.ContentType = r.Header.Get("Content-Type")
		got.BodyBytes, _ = io.ReadAll(r.Body)

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	t.Cleanup(srv.Close)

	c := newTestClient(t, 1<<20)

	h := http.Header{}
	h.Set("X-Test", "1")

	q := url.Values{}
	q.Add("a", "1")
	q.Add("a", "2")
	q.Add("b", "x y") // ensure encoding

	body := map[string]any{"hello": "world"}

	_, err := c.MakeRequest(context.Background(), RequestOptions{
		Method:   http.MethodPost,
		URL:      srv.URL + "/path",
		Headers:  h,
		Query:    q,
		Body:     body,
		Response: &map[string]any{},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got.Method != http.MethodPost {
		t.Fatalf("expected method POST, got %q", got.Method)
	}
	if got.Path != "/path" {
		t.Fatalf("expected path /path, got %q", got.Path)
	}

	// Query: we don't assume ordering, just presence of expected pairs.
	decoded, err := url.ParseQuery(got.RawQuery)
	if err != nil {
		t.Fatalf("failed to parse query: %v (raw=%q)", err, got.RawQuery)
	}
	if strings.Join(decoded["a"], ",") != "1,2" {
		t.Fatalf("expected a=1,2 got %v", decoded["a"])
	}
	if strings.Join(decoded["b"], ",") != "x y" {
		t.Fatalf("expected b='x y' got %v", decoded["b"])
	}

	// Header cloning: request should see original value, but later mutations must not affect in-flight.
	if got.Headers.Get("X-Test") != "1" {
		t.Fatalf("expected X-Test=1, got %q", got.Headers.Get("X-Test"))
	}
	// Content-Type should be set automatically when Body != nil and no Content-Type provided.
	if got.ContentType != "application/json" {
		t.Fatalf("expected Content-Type application/json, got %q", got.ContentType)
	}

	// Body should be JSON.
	var decodedBody map[string]any
	if err := json.Unmarshal(got.BodyBytes, &decodedBody); err != nil {
		t.Fatalf("expected JSON body, unmarshal error: %v; raw=%q", err, string(got.BodyBytes))
	}
	if decodedBody["hello"] != "world" {
		t.Fatalf("expected body.hello=world, got %v", decodedBody["hello"])
	}

	// Now mutate the original header; it must not have affected the request already made.
	h.Set("X-Test", "2")
	if got.Headers.Get("X-Test") != "1" {
		t.Fatalf("expected cloned headers to keep X-Test=1, got %q", got.Headers.Get("X-Test"))
	}
}

func TestAPIClient_do_DoesNotOverrideExplicitContentType(t *testing.T) {
	t.Parallel()

	var gotContentType string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotContentType = r.Header.Get("Content-Type")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	t.Cleanup(srv.Close)

	c := newTestClient(t, 1<<20)

	h := http.Header{}
	h.Set("Content-Type", "application/custom+json")

	_, err := c.MakeRequest(context.Background(), RequestOptions{
		Method:   http.MethodPost,
		URL:      srv.URL,
		Headers:  h,
		Body:     map[string]any{"x": 1},
		Response: &map[string]any{},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if gotContentType != "application/custom+json" {
		t.Fatalf("expected custom content-type preserved, got %q", gotContentType)
	}
}

func TestAPIClient_MakeRequest_Success_DecodesJSONIntoResponse(t *testing.T) {
	t.Parallel()

	type resp struct {
		A int    `json:"a"`
		B string `json:"b"`
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"a":42,"b":"ok"}`))
	}))
	t.Cleanup(srv.Close)

	c := newTestClient(t, 1<<20)

	var out resp
	res, err := c.MakeRequest(context.Background(), RequestOptions{
		Method:   http.MethodGet,
		URL:      srv.URL,
		Response: &out,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", res.StatusCode)
	}
	if out.A != 42 || out.B != "ok" {
		t.Fatalf("unexpected decoded response: %+v", out)
	}
}

func TestAPIClient_MakeRequest_Success_NoDecodeWhenResponseNilOrContentLengthZero(t *testing.T) {
	t.Parallel()

	t.Run("ResponseNil", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"a":1}`))
		}))
		t.Cleanup(srv.Close)

		c := newTestClient(t, 1<<20)

		_, err := c.MakeRequest(context.Background(), RequestOptions{
			Method:   http.MethodGet,
			URL:      srv.URL,
			Response: nil,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("ContentLengthZero", func(t *testing.T) {
		t.Parallel()

		// net/http will set Content-Length automatically based on response writer bytes,
		// so we explicitly set it to 0 and write no body.
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Length", "0")
			w.WriteHeader(http.StatusOK)
		}))
		t.Cleanup(srv.Close)

		c := newTestClient(t, 1<<20)

		var out map[string]any
		_, err := c.MakeRequest(context.Background(), RequestOptions{
			Method:   http.MethodGet,
			URL:      srv.URL,
			Response: &out,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func TestAPIClient_MakeRequest_ErrorStatus_ReturnsBodyAsError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte("bad request details"))
	}))
	t.Cleanup(srv.Close)

	c := newTestClient(t, 1<<20)

	res, err := c.MakeRequest(context.Background(), RequestOptions{
		Method: http.MethodGet,
		URL:    srv.URL,
	})
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if err.Error() != "bad request details" {
		t.Fatalf("expected error body 'bad request details', got %q", err.Error())
	}
	if res == nil || res.StatusCode != http.StatusBadRequest {
		if res == nil {
			t.Fatalf("expected non-nil response on HTTP error")
		}
		t.Fatalf("expected 400, got %d", res.StatusCode)
	}
}

func TestAPIClient_MakeRequest_DecodeError_ReturnsWrappedError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"a":`)) // invalid JSON
	}))
	t.Cleanup(srv.Close)

	c := newTestClient(t, 1<<20)

	var out map[string]any
	_, err := c.MakeRequest(context.Background(), RequestOptions{
		Method:   http.MethodGet,
		URL:      srv.URL,
		Response: &out,
	})
	if err == nil {
		t.Fatalf("expected decode error, got nil")
	}
	if !strings.Contains(err.Error(), "failed to decode JSON response") {
		t.Fatalf("expected wrapped decode error, got %v", err)
	}
}

func TestAPIClient_MakeRequest_SizeLimit_AppliesToErrorBody(t *testing.T) {
	t.Parallel()

	// Error body larger than size limit; error should contain only the limited prefix.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte("0123456789"))
	}))
	t.Cleanup(srv.Close)

	c := newTestClient(t, 5) // limit to 5 bytes

	_, err := c.MakeRequest(context.Background(), RequestOptions{
		Method: http.MethodGet,
		URL:    srv.URL,
	})
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if err.Error() != "01234" {
		t.Fatalf("expected limited error body '01234', got %q", err.Error())
	}
}
