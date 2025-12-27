package httpClient

import (
	"errors"
	"net/http"
	"net/url"
)

var (
	ErrEmptyURL    = errors.New("empty url")
	ErrEmptyMethod = errors.New("empty method")
)

type RequestOptions struct {
	Method   string
	URL      string
	Headers  http.Header
	Query    url.Values
	Body     any
	Response any
}
