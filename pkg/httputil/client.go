// Package httputil provides HTTP utilities including a retry client
// with exponential backoff, configurable timeouts, and standard headers.
package httputil

import (
	"bytes"
	"fmt"
	"io"
	"math"
	"net/http"
	"time"
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

	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			delay := exponentialBackoff(attempt, DefaultBaseDelay, DefaultMaxDelay)
			time.Sleep(delay)
		}

		resp, err := client.Do(req) // #nosec G704
		if err != nil {
			lastErr = fmt.Errorf("request attempt %d: %w", attempt+1, err)

			continue
		}

		body, err := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if err != nil {
			lastErr = fmt.Errorf("read response attempt %d: %w", attempt+1, err)

			continue
		}

		if resp.StatusCode >= 500 {
			lastErr = fmt.Errorf("HTTP %d (attempt %d): %s", resp.StatusCode, attempt+1, string(body))

			continue
		}

		if resp.StatusCode >= 400 {
			return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
		}

		return body, nil
	}

	return nil, fmt.Errorf("request failed after %d attempts: %w", maxRetries+1, lastErr)
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

// exponentialBackoff calculates delay with jitter-free exponential backoff.
func exponentialBackoff(attempt int, baseDelay, maxDelay time.Duration) time.Duration {
	delay := float64(baseDelay) * math.Pow(2, float64(attempt-1))
	if delay > float64(maxDelay) {
		delay = float64(maxDelay)
	}

	return time.Duration(delay)
}
