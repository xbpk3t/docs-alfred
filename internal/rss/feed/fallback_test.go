package rss

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mmcdole/gofeed"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewConfigFallback(t *testing.T) {
	t.Run("enabled with custom relay", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "rss.yml")
		require.NoError(t, os.WriteFile(path, []byte(`newsletter:
  schedule: daily
feed:
  fallback:
    enabled: true
    relay: https://relay.test/fetch?u={url}
`), 0o600))

		cfg, err := NewConfig(path)
		require.NoError(t, err)
		assert.True(t, cfg.FeedConfig.Fallback.Enabled)
		assert.Equal(t, "https://relay.test/fetch?u={url}", cfg.FeedConfig.Fallback.Relay)
	})

	t.Run("invalid relay without placeholder", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "rss.yml")
		require.NoError(t, os.WriteFile(path, []byte(`newsletter:
  schedule: daily
feed:
  fallback:
    enabled: true
    relay: https://relay.test/fetch
`), 0o600))

		_, err := NewConfig(path)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "must contain {url}")
	})
}

func TestFallbackRelayURL(t *testing.T) {
	t.Run("default relay", func(t *testing.T) {
		got, ok := fallbackRelayURL(FeedFallbackConfig{}, "https://example.com/feed.xml")
		require.True(t, ok)
		assert.Contains(t, got, "feed2json.org")
		assert.Contains(t, got, "https%3A%2F%2Fexample.com%2Ffeed.xml")
	})

	t.Run("custom relay", func(t *testing.T) {
		got, ok := fallbackRelayURL(
			FeedFallbackConfig{Relay: "https://relay.test/fetch?u={url}"},
			"https://example.com/feed.xml",
		)
		require.True(t, ok)
		assert.Equal(t, "https://relay.test/fetch?u=https%3A%2F%2Fexample.com%2Ffeed.xml", got)
	})

	t.Run("missing placeholder", func(t *testing.T) {
		_, ok := fallbackRelayURL(
			FeedFallbackConfig{Relay: "https://relay.test/fetch"},
			"https://example.com/feed.xml",
		)
		assert.False(t, ok)
	})
}

func TestShouldFallbackFailureKind(t *testing.T) {
	tests := []struct {
		name       string
		kind       FeedFailureKind
		transient  bool
		statusCode int
		want       bool
	}{
		{name: "timeout", kind: FeedFailureKindTimeout, want: true},
		{name: "dns", kind: FeedFailureKindDNS, want: true},
		{name: "tls", kind: FeedFailureKindTLS, want: true},
		{name: "network", kind: FeedFailureKindNetwork, want: true},
		{name: "http 403", kind: FeedFailureKindHTTPStatus, statusCode: 403, want: true},
		{name: "http 429", kind: FeedFailureKindHTTPStatus, statusCode: 429, want: true},
		{name: "http 500", kind: FeedFailureKindHTTPStatus, statusCode: 500, want: true},
		{name: "http transient without code", kind: FeedFailureKindHTTPStatus, transient: true, want: true},
		{name: "http 401", kind: FeedFailureKindHTTPStatus, statusCode: 401},
		{name: "http 404", kind: FeedFailureKindHTTPStatus, statusCode: 404},
		{name: "http 410", kind: FeedFailureKindHTTPStatus, statusCode: 410},
		{name: "parse", kind: FeedFailureKindParse},
		{name: "context canceled", kind: FeedFailureKindContextCancelled},
		{name: "invalid url", kind: FeedFailureKindInvalidURL},
		{name: "unknown", kind: FeedFailureKindUnknown},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, shouldFallbackFailureKind(tt.kind, tt.transient, tt.statusCode))
		})
	}
}

func TestShouldFallbackFeedError(t *testing.T) {
	assert.True(t, shouldFallbackFeedError(context.Background(), &FeedError{
		Kind: FeedFailureKindTimeout,
	}))
	assert.False(t, shouldFallbackFeedError(context.Background(), &FeedError{
		Kind: FeedFailureKindHTTPStatus,
		Err:  gofeed.HTTPError{StatusCode: 404},
	}))

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	assert.False(t, shouldFallbackFeedError(ctx, &FeedError{
		Kind: FeedFailureKindTimeout,
	}))
}

func TestFetchURLWithRetryFallbackOnTimeout(t *testing.T) {
	direct := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(1500 * time.Millisecond)
		_, _ = w.Write([]byte(`<?xml version="1.0"?><rss version="2.0"><channel><title>slow</title></channel></rss>`))
	}))
	defer direct.Close()

	relay := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.NotEmpty(t, r.URL.Query().Get("url"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
  "version": "https://jsonfeed.org/version/1",
  "title": "Relayed Feed",
  "items": [
    {"id": "1", "url": "https://example.com/1", "title": "Item 1", "content_html": "<p>hi</p>", "date_published": "2026-11-24T00:00:00Z"}
  ]
}`))
	}))
	defer relay.Close()

	cfg := &Config{FeedConfig: FeedConfig{
		Timeout:  1,
		MaxTries: 1,
		Fallback: FeedFallbackConfig{
			Enabled: true,
			Relay:   relay.URL + "/convert?url={url}",
		},
	}}

	feed, feedErr, via := fetchURLWithRetry(
		context.Background(),
		direct.URL+"/feed.xml",
		cfg,
		newHostFetchController(cfg.FeedConfig),
	)
	require.Nil(t, feedErr)
	require.NotNil(t, feed)
	assert.Equal(t, "Relayed Feed", feed.Title)
	assert.Equal(t, FetchedViaRelay, via)
}

func TestFetchURLWithRetryDoesNotFallbackOn404(t *testing.T) {
	direct := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "not found", http.StatusNotFound)
	}))
	defer direct.Close()

	relayCalls := 0
	relay := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		relayCalls++
	}))
	defer relay.Close()

	cfg := &Config{FeedConfig: FeedConfig{
		Timeout:  1,
		MaxTries: 1,
		Fallback: FeedFallbackConfig{
			Enabled: true,
			Relay:   relay.URL + "/convert?url={url}",
		},
	}}

	feed, feedErr, via := fetchURLWithRetry(
		context.Background(),
		direct.URL+"/feed.xml",
		cfg,
		newHostFetchController(cfg.FeedConfig),
	)
	assert.Nil(t, feed)
	require.NotNil(t, feedErr)
	assert.Equal(t, FeedFailureKindHTTPStatus, feedErr.Kind)
	assert.Zero(t, relayCalls)
	assert.Empty(t, via)
}

func TestFetchURLWithRetryFallbackDisabled(t *testing.T) {
	direct := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer direct.Close()

	relayCalls := 0
	relay := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		relayCalls++
	}))
	defer relay.Close()

	cfg := &Config{FeedConfig: FeedConfig{
		Timeout:  1,
		MaxTries: 1,
		Fallback: FeedFallbackConfig{
			Enabled: false,
			Relay:   relay.URL + "/convert?url={url}",
		},
	}}

	feed, feedErr, via := fetchURLWithRetry(
		context.Background(),
		direct.URL+"/feed.xml",
		cfg,
		newHostFetchController(cfg.FeedConfig),
	)
	assert.Nil(t, feed)
	require.NotNil(t, feedErr)
	assert.Zero(t, relayCalls)
	assert.Empty(t, via)
}
