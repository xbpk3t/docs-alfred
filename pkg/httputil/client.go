// Package httputil provides HTTP utilities including a retry client
// with exponential backoff, configurable timeouts, and standard headers.
package httputil

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"time"

	retry "github.com/avast/retry-go/v4"
)

// DefaultClientTimeout is the default HTTP client timeout.
const DefaultClientTimeout = 30 * time.Second

// DefaultMaxRetries is the default number of retry attempts.
const DefaultMaxRetries = 3

// DefaultBaseDelay is the base delay for exponential backoff.
const DefaultBaseDelay = 1 * time.Second

// DefaultMaxDelay is the maximum delay for exponential backoff.
const DefaultMaxDelay = 30 * time.Second

// NewClient creates an HTTP client with the given timeout.
func NewClient(timeout time.Duration) *http.Client {
	if timeout <= 0 {
		timeout = DefaultClientTimeout
	}

	return &http.Client{Timeout: timeout}
}

// DoWithRetry performs an HTTP request with exponential backoff retry.
// Returns the response body bytes on success.
func DoWithRetry(client *http.Client, req *http.Request, maxRetries int) ([]byte, error) {
	if client == nil {
		client = NewClient(DefaultClientTimeout)
	}
	if maxRetries <= 0 {
		maxRetries = DefaultMaxRetries
	}

	var result []byte
	err := retry.Do(
		func() error {
			// Clone the request since the body may be consumed
			clonedReq := req.Clone(req.Context())
			if req.Body != nil {
				bodyBytes, readErr := io.ReadAll(req.Body)
				if readErr != nil {
					return fmt.Errorf("read request body: %w", readErr)
				}
				_ = req.Body.Close()
				clonedReq.Body = io.NopCloser(bytes.NewReader(bodyBytes))
			}

			resp, doErr := client.Do(clonedReq) //nolint:gosec // G704: URL is controlled by the caller
			if doErr != nil {
				return doErr
			}

			body, readErr := io.ReadAll(resp.Body)
			_ = resp.Body.Close()
			if readErr != nil {
				return fmt.Errorf("read response: %w", readErr)
			}

			if resp.StatusCode >= 500 {
				return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
			}

			if resp.StatusCode >= 400 {
				return retry.Unrecoverable(fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body)))
			}

			result = body

			return nil
		},
		retry.Attempts(uint(maxRetries)),
		retry.Delay(DefaultBaseDelay),
		retry.DelayType(retry.BackOffDelay),
		retry.LastErrorOnly(true),
	)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}

	return result, nil
}

// PostJSON performs an HTTP POST with JSON body and returns the response bytes.
// It uses an internal default client with retry.
func PostJSON(url string, body []byte, headers map[string]string) ([]byte, error) {
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	return DoWithRetry(NewClient(DefaultClientTimeout), req, DefaultMaxRetries)
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

// GetWithRetry performs an HTTP GET with retry.
func GetWithRetry(client *http.Client, url string, maxRetries int) ([]byte, error) {
	if client == nil {
		client = NewClient(DefaultClientTimeout)
	}

	req, err := http.NewRequest(http.MethodGet, url, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("create get request: %w", err)
	}

	return DoWithRetry(client, req, maxRetries)
}
