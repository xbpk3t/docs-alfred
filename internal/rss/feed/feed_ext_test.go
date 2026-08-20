package rss

import (
	"errors"
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
	// endDate simulates a runner clock in UTC (the self-hosted runner is not
	// guaranteed to be Asia/Shanghai): 2026-08-15 22:00 UTC == 2026-08-16 06:00
	// Asia/Shanghai. Day boundaries must still resolve in Asia/Shanghai.
	endDate := time.Date(2026, 8, 15, 22, 0, 0, 0, time.UTC)

	tests := []struct {
		name     string
		created  time.Time
		schedule string
		want     bool
	}{
		{
			name:     "daily: yesterday evening Shanghai",
			created:  time.Date(2026, 8, 15, 10, 0, 0, 0, time.UTC), // 18:00 +08 Aug 15
			schedule: Daily,
			want:     true,
		},
		{
			name:     "daily: yesterday early morning Shanghai (still previous UTC day)",
			created:  time.Date(2026, 8, 14, 17, 0, 0, 0, time.UTC), // 01:00 +08 Aug 15
			schedule: Daily,
			want:     true,
		},
		{
			name:     "daily: two calendar days ago excluded",
			created:  time.Date(2026, 8, 14, 0, 0, 0, 0, time.UTC), // 08:00 +08 Aug 14
			schedule: Daily,
			want:     false,
		},
		{
			name:     "daily: today excluded (strict yesterday)",
			created:  time.Date(2026, 8, 15, 23, 0, 0, 0, time.UTC), // 07:00 +08 Aug 16
			schedule: Daily,
			want:     false,
		},
		{
			name:     "weekly: within previous 7 calendar days",
			created:  time.Date(2026, 8, 10, 0, 0, 0, 0, time.UTC), // 08:00 +08 Aug 10
			schedule: Weekly,
			want:     true,
		},
		{
			name:     "weekly: outside window",
			created:  time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC),
			schedule: Weekly,
			want:     false,
		},
		{
			name:     "invalid schedule",
			created:  time.Date(2026, 8, 15, 10, 0, 0, 0, time.UTC),
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
