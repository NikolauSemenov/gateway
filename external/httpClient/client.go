package httpClient

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"time"
)

type APIClient struct {
	http *http.Client
}

func NewAPIClient(timeout time.Duration) *APIClient {
	if timeout <= 0 {
		timeout = 5 * time.Second
	}

	return &APIClient{
		http: &http.Client{
			Timeout: timeout,
		},
	}
}

func (a *APIClient) MakeRequest(
	ctx context.Context,
	r RequestOptions,
) (*http.Response, error) {

	res, err := a.do(ctx, r)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode >= http.StatusBadRequest {
		body, _ := io.ReadAll(io.LimitReader(res.Body, 64<<10))
		// TODO: Поправить возврат ошибок, сейчас передаются как байты обернутые в строки
		return res, errors.New(string(body))
	}

	if res.ContentLength == 0 {
		return res, nil
	}

	if r.Response != nil {
		dec := json.NewDecoder(io.LimitReader(res.Body, 1<<20))
		if err = dec.Decode(r.Response); err != nil {
			return res, err
		}
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
			return nil, err
		}
		body = bytes.NewReader(b)
	}

	req, err := http.NewRequestWithContext(
		ctx,
		r.Method,
		parsedURL.String(),
		body,
	)
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
