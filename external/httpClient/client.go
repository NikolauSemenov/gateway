package httpClient

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"gateway/internal/config"
	"io"
	"net"
	"net/http"
	"net/url"
	"time"
)

type APIClient struct {
	http      *http.Client
	configApp *config.Config
}

func NewAPIClient(configApp *config.Config) *APIClient {
	timeout := configApp.Timeout
	if timeout <= 0 {
		// Safe fallback: avoid "no timeout" footgun.
		timeout = 5 * time.Second
	}

	sizeLimit := configApp.SizeLimit
	if sizeLimit <= 0 {
		// Safe fallback: avoid "unlimited reads" footgun.
		sizeLimit = 64 * 1024
	}

	// Keep the adjusted values inside the client so size limits/timeouts are enforced even if config is misconfigured.
	cfg := *configApp
	cfg.Timeout = timeout
	cfg.SizeLimit = sizeLimit

	// Harden transport defaults for reliability under load (pooling, timeouts, keep-alive).
	// NOTE: http.Client.Timeout is an overall per-request deadline, but it's still useful to set lower-level timeouts too.
	dialer := &net.Dialer{
		Timeout:   3 * time.Second,
		KeepAlive: 30 * time.Second,
	}

	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			return dialer.DialContext(ctx, network, addr)
		},
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          200,
		MaxIdleConnsPerHost:   50,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   5 * time.Second,
		ResponseHeaderTimeout: 5 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}

	return &APIClient{
		http: &http.Client{
			Timeout:   timeout,
			Transport: transport,
		},
		configApp: &cfg,
	}
}

func (a *APIClient) MakeRequest(ctx context.Context, r RequestOptions) (*http.Response, error) {
	res, err := a.do(ctx, r)
	if err != nil {
		return nil, err
	}
	defer func() {
		// Drain body so underlying transport can reuse the connection.
		_, _ = io.Copy(io.Discard, res.Body)
		_ = res.Body.Close()
	}()

	// Error responses: read limited body and return it as an error message (trimmed).
	if res.StatusCode >= http.StatusBadRequest {
		body, err := io.ReadAll(io.LimitReader(res.Body, a.configApp.SizeLimit))
		if err != nil {
			return res, fmt.Errorf("failed to read error body: %w", err)
		}
		return res, errors.New(string(body))
	}

	// If caller didn't provide a response target, do not attempt to decode.
	// Also handle "unknown length" responses: ContentLength may be -1, so don't rely only on ContentLength==0.
	if r.Response == nil {
		return res, nil
	}

	// Decode JSON safely with size limit and strictness.
	dec := json.NewDecoder(io.LimitReader(res.Body, a.configApp.SizeLimit))
	dec.DisallowUnknownFields()
	if err := dec.Decode(r.Response); err != nil {
		// Treat empty body as OK (common for 204/empty 200s).
		if errors.Is(err, io.EOF) {
			return res, nil
		}
		return res, fmt.Errorf("failed to decode JSON response: %w", err)
	}

	// Ensure there's no trailing junk (e.g., multiple JSON values).
	if err := ensureNoTrailingJSON(dec); err != nil {
		return res, fmt.Errorf("failed to decode JSON response: %w", err)
	}

	return res, nil
}

func (a *APIClient) do(ctx context.Context, r RequestOptions) (*http.Response, error) {
	if r.URL == "" {
		return nil, ErrEmptyURL
	}
	if r.Method == "" {
		return nil, ErrEmptyMethod
	}

	parsedURL, err := url.Parse(r.URL)
	if err != nil {
		return nil, err
	}

	// Hardening: prevent SSRF-style surprises via non-http schemes.
	switch parsedURL.Scheme {
	case "http", "https":
		// ok
	default:
		return nil, fmt.Errorf("unsupported url scheme: %q", parsedURL.Scheme)
	}

	// Hardening: require a host.
	if parsedURL.Host == "" {
		return nil, fmt.Errorf("missing url host")
	}

	if len(r.Query) > 0 {
		q := parsedURL.Query()
		for k, vs := range r.Query {
			for _, v := range vs {
				q.Add(k, v)
			}
		}
		parsedURL.RawQuery = q.Encode()
	}

	var body io.Reader = http.NoBody
	if r.Body != nil {
		b, err := json.Marshal(r.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal body: %w", err)
		}
		body = bytes.NewReader(b)
	}

	req, err := http.NewRequestWithContext(ctx, r.Method, parsedURL.String(), body)
	if err != nil {
		return nil, err
	}

	if r.Headers != nil {
		req.Header = r.Headers.Clone()
	}

	// Default content type for JSON bodies.
	if r.Body != nil && req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}

	// Default accept header (helps some APIs return JSON consistently).
	if req.Header.Get("Accept") == "" {
		req.Header.Set("Accept", "application/json")
	}

	return a.http.Do(req)
}

// ensureNoTrailingJSON verifies the decoder is at EOF after the first JSON value.
// This prevents accepting responses like: {"ok":true}{"evil":true}
func ensureNoTrailingJSON(dec *json.Decoder) error {
	var extra any
	err := dec.Decode(&extra)
	if err == nil {
		return errors.New("unexpected trailing data after JSON value")
	}
	if errors.Is(err, io.EOF) {
		return nil
	}
	return err
}
