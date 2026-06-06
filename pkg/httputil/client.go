// Package httputil provides HTTP utilities including a retry client
// with exponential backoff, configurable timeouts, and standard headers.
package httputil

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/go-resty/resty/v2"
)

// DefaultClientTimeout is the default HTTP client timeout.
const DefaultClientTimeout = 30 * time.Second

// DefaultMaxRetries is the default number of retry attempts.
const DefaultMaxRetries = 3

// DefaultBaseDelay is the base delay for exponential backoff.
const DefaultBaseDelay = 1 * time.Second

// DefaultMaxDelay is the maximum delay for exponential backoff.
const DefaultMaxDelay = 30 * time.Second

// RequestOptions configures helper HTTP requests.
type RequestOptions struct {
	Headers    map[string]string
	Timeout    time.Duration
	MaxRetries int
}

// NewRestyClient creates a resty client with retry and backoff configured.
func NewRestyClient(timeout time.Duration, maxRetries int) *resty.Client {
	if timeout <= 0 {
		timeout = DefaultClientTimeout
	}
	if maxRetries <= 0 {
		maxRetries = DefaultMaxRetries
	}

	return resty.New().
		SetTimeout(timeout).
		SetRetryCount(maxRetries).
		SetRetryWaitTime(DefaultBaseDelay).
		SetRetryMaxWaitTime(DefaultMaxDelay).
		AddRetryCondition(func(r *resty.Response, err error) bool {
			if err != nil {
				return true
			}
			// Retry on 5xx
			return r.StatusCode() >= 500
		})
}

// GetBytes performs an HTTP GET and returns response bytes.
func GetBytes(ctx context.Context, url string, opts RequestOptions) ([]byte, error) {
	rc := NewRestyClient(opts.Timeout, opts.MaxRetries)
	req := rc.R().SetContext(ctx)
	for k, v := range opts.Headers {
		req.SetHeader(k, v)
	}

	resp, err := req.Get(url)
	if err != nil {
		return nil, fmt.Errorf("get %s: %w", url, err)
	}
	if resp.StatusCode() >= 400 {
		return nil, fmt.Errorf("GET %s: HTTP %d: %s", url, resp.StatusCode(), string(resp.Body()))
	}

	return resp.Body(), nil
}

// PostJSONWithResult posts a JSON payload, optionally decoding a 2xx JSON response into result.
func PostJSONWithResult(ctx context.Context, url string, payload, result any, opts RequestOptions) ([]byte, error) {
	rc := NewRestyClient(opts.Timeout, opts.MaxRetries)
	req := rc.R().
		SetContext(ctx).
		SetHeader("Content-Type", "application/json").
		SetBody(payload)

	for k, v := range opts.Headers {
		req.SetHeader(k, v)
	}
	if result != nil {
		req.SetResult(result)
	}

	resp, err := req.Post(url)
	if err != nil {
		return nil, fmt.Errorf("post %s: %w", url, err)
	}
	if resp.StatusCode() >= 400 {
		return nil, fmt.Errorf("POST %s: HTTP %d: %s", url, resp.StatusCode(), string(resp.Body()))
	}

	return resp.Body(), nil
}

// NewClient creates an HTTP client with the given timeout.
// Retained for callers that need a plain *http.Client without retry.
func NewClient(timeout time.Duration) *http.Client {
	if timeout <= 0 {
		timeout = DefaultClientTimeout
	}

	return &http.Client{Timeout: timeout}
}

// Get performs an HTTP GET and returns the response bytes.
func Get(client *http.Client, url string) ([]byte, error) {
	if client == nil {
		client = NewClient(DefaultClientTimeout)
	}

	req, err := http.NewRequest(http.MethodGet, url, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("create get request: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("get %s: %w", url, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)

		return nil, fmt.Errorf("GET %s: HTTP %d: %s", url, resp.StatusCode, string(respBody))
	}

	return io.ReadAll(resp.Body)
}
