package rss

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateURL(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		wantErr bool
		wantNil bool
	}{
		{name: "empty URL", url: "", wantErr: true},
		{name: "valid URL", url: "https://example.com/feed.xml", wantErr: false, wantNil: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateURL(tt.url)
			if tt.wantNil {
				assert.Nil(t, err)
			} else {
				require.NotNil(t, err)
				assert.Equal(t, FeedFailureKindInvalidURL, err.Kind)
			}
		})
	}
}

func TestGetScheduleTimeRanges(t *testing.T) {
	ranges := GetScheduleTimeRanges()
	assert.Equal(t, 24, ranges[Daily])
	assert.Equal(t, 7*24, ranges[Weekly])
	assert.Len(t, ranges, 2)
}

func TestFilterFeedsWithTimeRange(t *testing.T) {
	endDate := time.Date(2026, 6, 20, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name     string
		created  time.Time
		schedule string
		want     bool
	}{
		{
			name:     "daily schedule within range",
			created:  time.Date(2026, 6, 20, 0, 0, 0, 0, time.UTC),
			schedule: Daily,
			want:     true,
		},
		{
			name:     "daily schedule out of range",
			created:  time.Date(2026, 6, 18, 0, 0, 0, 0, time.UTC),
			schedule: Daily,
			want:     false,
		},
		{
			name:     "weekly schedule within range",
			created:  time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC),
			schedule: Weekly,
			want:     true,
		},
		{
			name:     "invalid schedule",
			created:  time.Date(2026, 6, 20, 0, 0, 0, 0, time.UTC),
			schedule: "monthly",
			want:     false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FilterFeedsWithTimeRange(tt.created, endDate, tt.schedule)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestFeedError(t *testing.T) {
	fe := &FeedError{
		URL:     "https://example.com/feed",
		Message: "parse failed",
		Err:     errors.New("xml syntax error"),
	}
	assert.Contains(t, fe.Error(), "https://example.com/feed")
	assert.Contains(t, fe.Error(), "parse failed")
	assert.Contains(t, fe.Error(), "xml syntax error")
}

func TestGetMaxAttempts(t *testing.T) {
	tests := []struct {
		name     string
		maxTries int
		want     uint
	}{
		{name: "positive", maxTries: 5, want: 5},
		{name: "zero", maxTries: 0, want: 0},
		{name: "negative", maxTries: -1, want: 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{FeedConfig: FeedConfig{MaxTries: tt.maxTries}}
			assert.Equal(t, tt.want, getMaxAttempts(cfg))
		})
	}
}

func TestAiModelForWiki(t *testing.T) {
	tests := []struct {
		name string
		want string
		cfg  Config
	}{
		{
			name: "wiki model set",
			cfg:  Config{WikiConfig: WikiConfig{Ai: WikiAiConfig{Model: "wiki-model"}}},
			want: "wiki-model",
		},
		{
			name: "fallback to trns model",
			cfg:  Config{TrnsConfig: TrnsConfig{Summary: TrnsSummaryConfig{Model: "trns-model"}}},
			want: "trns-model",
		},
		{
			name: "wiki takes priority",
			cfg: Config{
				WikiConfig: WikiConfig{Ai: WikiAiConfig{Model: "wiki-model"}},
				TrnsConfig: TrnsConfig{Summary: TrnsSummaryConfig{Model: "trns-model"}},
			},
			want: "wiki-model",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.cfg.AiModelForWiki())
		})
	}
}

func TestAiBaseURLForWiki(t *testing.T) {
	tests := []struct {
		name string
		want string
		cfg  Config
	}{
		{
			name: "wiki base URL set",
			cfg:  Config{WikiConfig: WikiConfig{Ai: WikiAiConfig{BaseURL: "https://wiki.ai"}}},
			want: "https://wiki.ai",
		},
		{
			name: "fallback to trns base URL",
			cfg:  Config{TrnsConfig: TrnsConfig{Summary: TrnsSummaryConfig{BaseURL: "https://trns.ai"}}},
			want: "https://trns.ai",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.cfg.AiBaseURLForWiki())
		})
	}
}

func TestValidateForSend(t *testing.T) {
	tests := []struct {
		name    string
		wantErr string
		cfg     Config
	}{
		{
			name:    "missing token",
			cfg:     Config{NewsletterConfig: NewsletterConfig{Schedule: "daily"}},
			wantErr: "resend token is required",
		},
		{
			name: "valid config",
			cfg: Config{
				NewsletterConfig: NewsletterConfig{Schedule: "daily"},
				ResendConfig:     ResendConfig{Token: "test-token"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.ValidateForSend()
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestCreateFeedParser(t *testing.T) {
	cfg := &Config{FeedConfig: FeedConfig{Timeout: 10}}
	fp := createFeedParser(cfg)
	require.NotNil(t, fp)
	assert.Equal(t, DefaultUserAgent, fp.UserAgent)
	assert.NotNil(t, fp.Client)
}

func TestFetchURLWithRetryInvalidURL(t *testing.T) {
	cfg := &Config{FeedConfig: FeedConfig{Timeout: 5, MaxTries: 1}}
	feed, feedErr := FetchURLWithRetry(context.Background(), "", cfg)
	assert.Nil(t, feed)
	require.NotNil(t, feedErr)
	assert.Equal(t, FeedFailureKindInvalidURL, feedErr.Kind)
}

func TestFetchURLWithRetryConnectionError(t *testing.T) {
	cfg := &Config{FeedConfig: FeedConfig{Timeout: 1, MaxTries: 1}}
	feed, feedErr := FetchURLWithRetry(context.Background(), "http://127.0.0.1:1/nonexistent", cfg)
	assert.Nil(t, feed)
	require.NotNil(t, feedErr)
	assert.NotEmpty(t, feedErr.URL)
}

func TestFetchURLWithRetryContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	cfg := &Config{FeedConfig: FeedConfig{Timeout: 5, MaxTries: 2}}
	feed, feedErr := FetchURLWithRetry(ctx, "http://127.0.0.1:1/feed", cfg)
	assert.Nil(t, feed)
	require.NotNil(t, feedErr)
}

func TestFetchURLsEmpty(t *testing.T) {
	cfg := &Config{FeedConfig: FeedConfig{Timeout: 1, MaxTries: 1}}
	feeds, failures := FetchURLs(context.Background(), nil, cfg)
	assert.Empty(t, feeds)
	assert.Empty(t, failures)
}

func TestFetchURLsWithMetaEmpty(t *testing.T) {
	cfg := &Config{FeedConfig: FeedConfig{Timeout: 1, MaxTries: 1}}
	feeds, meta, failures := FetchURLsWithMeta(context.Background(), []string{}, cfg)
	assert.Empty(t, feeds)
	assert.Empty(t, meta)
	assert.Empty(t, failures)
}

func TestFetchURLWithRetrySuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/rss+xml")
		_, _ = w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0"><channel><title>Test</title>
<item><title>Item 1</title><link>https://example.com/1</link></item>
</channel></rss>`))
	}))
	t.Cleanup(server.Close)

	cfg := &Config{FeedConfig: FeedConfig{Timeout: 5, MaxTries: 1}}
	feed, feedErr := FetchURLWithRetry(context.Background(), server.URL+"/feed.xml", cfg)
	require.Nil(t, feedErr)
	require.NotNil(t, feed)
	assert.Equal(t, "Test", feed.Title)
}

func TestFetchURLsWithMetaPartialFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/rss+xml")
		_, _ = w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0"><channel><title>Test</title>
<item><title>Item</title><link>https://example.com/1</link></item>
</channel></rss>`))
	}))
	t.Cleanup(server.Close)

	cfg := &Config{FeedConfig: FeedConfig{Timeout: 1, MaxTries: 1}}
	feeds, meta, failures := FetchURLsWithMeta(context.Background(), []string{
		server.URL + "/feed.xml",
		"http://127.0.0.1:1/nonexistent",
	}, cfg)
	assert.Len(t, feeds, 1)
	assert.Len(t, meta, 2)
	assert.Len(t, failures, 1)
}

func TestNewConfigValidFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "rss.yml")
	require.NoError(t, os.WriteFile(path, []byte(`newsletter:
  schedule: daily
`), 0o600))
	cfg, err := NewConfig(path)
	require.NoError(t, err)
	assert.NotNil(t, cfg)
	assert.Equal(t, "daily", cfg.NewsletterConfig.Schedule)
}

func TestNewConfigInvalidPath(t *testing.T) {
	_, err := NewConfig("/tmp/nonexistent-config-12345.yml")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "read config")
}

func TestNewConfigInvalidYAML(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.yml")
	require.NoError(t, os.WriteFile(path, []byte("newsletter:\n  schedule: [invalid"), 0o600))
	_, err := NewConfig(path)
	require.Error(t, err)
}

func TestWrapConfigLoadErrorReadStage(t *testing.T) {
	// Test the non-LoadError path
	err := wrapConfigLoadError(errors.New("plain error"))
	assert.EqualError(t, err, "plain error")
}

func TestValidate(t *testing.T) {
	// Valid config should pass validation
	cfg := &Config{NewsletterConfig: NewsletterConfig{Schedule: "daily"}}
	err := cfg.Validate()
	assert.NoError(t, err)
}

func TestValidateForSend_MissingToken(t *testing.T) {
	cfg := &Config{NewsletterConfig: NewsletterConfig{Schedule: "daily"}}
	err := cfg.ValidateForSend()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "resend token is required")
}

func TestNewConfigValidationFails(t *testing.T) {
	path := filepath.Join(t.TempDir(), "rss-validate.yml")
	// Invalid schedule value should fail validation
	require.NoError(t, os.WriteFile(path, []byte(`newsletter:
  schedule: monthly
`), 0o600))
	_, err := NewConfig(path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "validate config")
}

func TestValidate_InvalidSchedule(t *testing.T) {
	cfg := &Config{NewsletterConfig: NewsletterConfig{Schedule: "monthly"}}
	err := cfg.Validate()
	require.Error(t, err)
}

func TestFetchURLWithRetryContextCancelledDuringRetry(t *testing.T) {
	// Create a server that always fails to trigger retries
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "server error", http.StatusInternalServerError)
	}))
	t.Cleanup(server.Close)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	cfg := &Config{FeedConfig: FeedConfig{Timeout: 5, MaxTries: 2}}
	feed, feedErr := FetchURLWithRetry(ctx, server.URL+"/feed.xml", cfg)
	assert.Nil(t, feed)
	require.NotNil(t, feedErr)
}
