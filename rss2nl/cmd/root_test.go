package cmd

import (
	"testing"
	"time"

	"github.com/mmcdole/gofeed"
	gofeedext "github.com/mmcdole/gofeed/extensions"
	"github.com/stretchr/testify/assert"
	"github.com/xbpk3t/docs-alfred/service/rss"
)

func TestGetItemTitle(t *testing.T) {
	tests := []struct {
		item          *gofeed.Item
		name          string
		expectedTitle string
		hideAuthor    bool
	}{
		{
			name:       "with author not hidden",
			hideAuthor: false,
			item: &gofeed.Item{
				Title: "Test Title",
				Author: &gofeed.Person{
					Name: "Test Author",
				},
			},
			expectedTitle: "[Test Author] Test Title",
		},
		{
			name:       "with author hidden",
			hideAuthor: true,
			item: &gofeed.Item{
				Title: "Test Title",
				Author: &gofeed.Person{
					Name: "Test Author",
				},
			},
			expectedTitle: "Test Title",
		},
		{
			name:       "no author",
			hideAuthor: false,
			item: &gofeed.Item{
				Title: "Test Title",
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
			}, "")

			title := service.getItemTitle(tt.item)
			assert.Equal(t, tt.expectedTitle, title)
		})
	}
}

func TestItemIdentity(t *testing.T) {
	hash1 := itemIdentity("https://example.com/feed", &gofeed.Item{GUID: "123", Link: "https://example.com/post1", Title: "Title"})
	hash2 := itemIdentity("https://example.com/feed", &gofeed.Item{GUID: "123", Link: "https://example.com/post1", Title: "Title"})
	hash3 := itemIdentity("https://example.com/feed", &gofeed.Item{GUID: "456", Link: "https://example.com/post2", Title: "Title"})

	assert.Equal(t, hash1, hash2, "same inputs should produce same hash")
	assert.NotEqual(t, hash1, hash3, "different inputs should produce different hash")
	assert.Len(t, hash1, 64, "sha256 hex should be 64 chars")
}

func TestExtractTranscriptRefs(t *testing.T) {
	item := &gofeed.Item{
		Extensions: gofeedext.Extensions{
			"podcast": map[string][]gofeedext.Extension{
				"transcript": {
					{Attrs: map[string]string{"url": "https://example.com/transcript.txt", "type": "text/plain"}},
					{Attrs: map[string]string{"url": "https://example.com/transcript.vtt", "type": "text/vtt"}},
				},
			},
		},
	}

	refs := extractTranscriptRefs(item)
	assert.Len(t, refs, 2)
	assert.Equal(t, "text/plain", refs[0].Type)
	assert.Equal(t, "https://example.com/transcript.txt", refs[0].URL)
}

func TestMergeFeedItems_DedupByLink(t *testing.T) {
	service := NewNewsletterService(&rss.Config{
		FeedConfig: rss.FeedConfig{FeedLimit: 30},
		NewsletterConfig: rss.NewsletterConfig{
			Schedule: "daily",
		},
	}, "")

	now := time.Now()
	feeds := []*gofeed.Feed{
		{
			Title: "Test Feed",
			Items: []*gofeed.Item{
				{Title: "Item 1", Link: "http://example.com/1", GUID: "1", PublishedParsed: &now},
				{Title: "Item 2", Link: "http://example.com/2", GUID: "2", PublishedParsed: &now},
				{Title: "Item 1 duplicate", Link: "http://example.com/1", GUID: "1", PublishedParsed: &now},
			},
		},
	}

	items, err := service.mergeFeedItems("test", feeds)
	assert.NoError(t, err)
	assert.Len(t, items, 2) // one deduplicated
	assert.Equal(t, "Item 1", items[0].Title)
}
