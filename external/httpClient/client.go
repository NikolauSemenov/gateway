package httpClient

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"gateway/internal/config"
	"io"
	"net/http"
	"net/url"
)

type APIClient struct {
	http      *http.Client
	configApp *config.Config
}

func NewAPIClient(configApp *config.Config) *APIClient {
	return &APIClient{
		http: &http.Client{
			Timeout: configApp.Timeout,
		},
		configApp: configApp,
	}
}

func (a *APIClient) MakeRequest(ctx context.Context, r RequestOptions) (*http.Response, error) {
	res, err := a.do(ctx, r)
	if err != nil {
		return nil, err
	}
	defer func() {
		_, _ = io.Copy(io.Discard, res.Body)
		_ = res.Body.Close()
	}()

	if res.StatusCode >= http.StatusBadRequest {
		body, err := io.ReadAll(io.LimitReader(res.Body, a.configApp.SizeLimit))
		if err != nil {
			return res, fmt.Errorf("failed to read error body: %w", err)
		}
		return res, errors.New(string(body))
	}
	if r.Response == nil || res.ContentLength == 0 {
		return res, nil
	}

	dec := json.NewDecoder(io.LimitReader(res.Body, a.configApp.SizeLimit))
	if err := dec.Decode(r.Response); err != nil {
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

	if r.Body != nil && req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}

	return a.http.Do(req)
}
