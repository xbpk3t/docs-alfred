package rss

import (
	"testing"
	"time"

	"github.com/mmcdole/gofeed"
	"github.com/stretchr/testify/assert"
)

func TestFilterFeedsWithTimeRange(t *testing.T) {
	tests := []struct {
		name     string
		created  time.Time
		endDate  time.Time
		schedule string
		want     bool
	}{
		{
			name:     "daily schedule within range",
			created:  time.Now().Add(-12 * time.Hour),
			endDate:  time.Now(),
			schedule: Daily,
			want:     true,
		},
		{
			name:     "daily schedule out of range",
			created:  time.Now().Add(-36 * time.Hour),
			endDate:  time.Now(),
			schedule: Daily,
			want:     false,
		},
		{
			name:     "weekly schedule within range",
			created:  time.Now().Add(-24 * 6 * time.Hour),
			endDate:  time.Now(),
			schedule: Weekly,
			want:     true,
		},
		{
			name:     "invalid schedule",
			created:  time.Now(),
			endDate:  time.Now(),
			schedule: "invalid",
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FilterFeedsWithTimeRange(tt.created, tt.endDate, tt.schedule)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestMergeAllFeeds(t *testing.T) {
	tests := []struct {
		name      string
		config    *Config
		feeds     []*gofeed.Feed
		wantErr   bool
		itemCount int
	}{
		{
			name: "successful merge",
			config: &Config{
				Newsletter: NewsletterConfig{
					Schedule: Daily,
				},
				Feed: FeedConfig{
					FeedLimit: 10,
				},
			},
			feeds: []*gofeed.Feed{
				{
					Title: "Test Feed 1",
					Items: []*gofeed.Item{
						{
							Title:           "Test Item 1",
							Link:            "http://example.com/1",
							PublishedParsed: &time.Time{},
						},
					},
				},
			},
			wantErr:   false,
			itemCount: 1,
		},
		{
			name: "empty feeds",
			config: &Config{
				Newsletter: NewsletterConfig{
					Schedule: Daily,
				},
			},
			feeds:     []*gofeed.Feed{},
			wantErr:   true,
			itemCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			feed := NewFeed(tt.config)
			merged, err := feed.MergeAllFeeds("Test Feed", tt.feeds)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.itemCount, len(merged.Items))
		})
	}
}

func TestValidateURL(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		wantErr bool
	}{
		{
			name:    "valid url",
			url:     "http://example.com",
			wantErr: false,
		},
		{
			name:    "empty url",
			url:     "",
			wantErr: true,
		},
	}

	feed := NewFeed(&Config{})
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := feed.validateURL(tt.url)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
