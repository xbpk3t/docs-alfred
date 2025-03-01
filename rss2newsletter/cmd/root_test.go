package cmd

import (
	"testing"
	"time"

	"github.com/gorilla/feeds"
	"github.com/stretchr/testify/assert"
	"github.com/xbpk3t/docs-alfred/pkg/rss"
)

func TestGetItemTitle(t *testing.T) {
	tests := []struct {
		item          *feeds.Item
		name          string
		expectedTitle string
		hideAuthor    bool
	}{
		{
			name:       "with author not hidden",
			hideAuthor: false,
			item: &feeds.Item{
				Title: "Test Title",
				Author: &feeds.Author{
					Name: "Test Author",
				},
			},
			expectedTitle: "[Test Author] Test Title",
		},
		{
			name:       "with author hidden",
			hideAuthor: true,
			item: &feeds.Item{
				Title: "Test Title",
				Author: &feeds.Author{
					Name: "Test Author",
				},
			},
			expectedTitle: "Test Title",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := NewNewsletterService(&rss.Config{
				NewsletterConfig: rss.NewsletterConfig{
					IsHideAuthorInTitle: tt.hideAuthor,
				},
			})

			title := service.getItemTitle(tt.item)
			assert.Equal(t, tt.expectedTitle, title)
		})
	}
}

func TestConvertToRssFeed(t *testing.T) {
	service := NewNewsletterService(&rss.Config{})
	now := time.Now()

	feed := &feeds.Feed{
		Items: []*feeds.Item{
			{
				Title: "Test Title",
				Link:  &feeds.Link{Href: "http://example.com"},
				Author: &feeds.Author{
					Name: "Test Author",
				},
				Created: now,
			},
		},
	}

	result := service.convertToRssFeed("test-type", feed)

	assert.Equal(t, "test-type", result.Category)
	assert.Len(t, result.Items, 1)
	assert.Equal(t, "[Test Author] Test Title", result.Items[0].Title)
	assert.Equal(t, "http://example.com", result.Items[0].Link)
}
