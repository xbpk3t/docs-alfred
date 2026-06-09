package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mmcdole/gofeed"
	gofeedext "github.com/mmcdole/gofeed/extensions"
	"github.com/stretchr/testify/assert"
	"github.com/xbpk3t/docs-alfred/rss2nl/transcript"
	"github.com/xbpk3t/docs-alfred/service/rss"
)

type recordingTranscriptProvider struct {
	seen *transcript.EpisodeRef
}

func (p *recordingTranscriptProvider) Name() string {
	return "recording"
}

func (p *recordingTranscriptProvider) Fetch(_ context.Context, ep *transcript.EpisodeRef) (*transcript.TranscriptResult, error) {
	p.seen = ep

	return &transcript.TranscriptResult{Content: "recorded transcript", ContentType: "plaintext"}, nil
}

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

func TestMakeNewsletterItem_FeedTitleUsesFeedTitleWithLinkFallback(t *testing.T) {
	service := NewNewsletterService(&rss.Config{}, "")
	now := time.Now()
	item := &gofeed.Item{Title: "Episode", Link: "https://example.com/episode", PublishedParsed: &now}

	withTitle := service.makeNewsletterItem(item, &gofeed.Feed{Title: "Feed Title", Link: "https://example.com/feed"}, "podcast", "hash")
	assert.Equal(t, "Feed Title", withTitle.FeedTitle)

	withoutTitle := service.makeNewsletterItem(item, &gofeed.Feed{Link: "https://example.com/feed"}, "podcast", "hash")
	assert.Equal(t, "https://example.com/feed", withoutTitle.FeedTitle)
}

func TestProcessNewsletterTrnsItemsRespectsDefaultLimitAndReportsCounts(t *testing.T) {
	items := []NewsletterItem{
		{Title: "No media"},
		{Title: "Linked 1", Link: "https://example.com/1", EnclosureURL: "https://example.com/1.mp3"},
		{Title: "Failed", Link: "https://example.com/2", PodcastTranscripts: []PodcastTranscriptRef{{URL: "https://example.com/2.txt"}}},
		{Title: "Skipped by limit", Link: "https://example.com/3", EnclosureURL: "https://example.com/3.mp3"},
	}
	processed := 0

	report := processNewsletterTrnsItems(items, 2, func(item *NewsletterItem) (string, error) {
		processed++
		if item.Title == "Failed" {
			return "", assert.AnError
		}

		return "https://litter.catbox.moe/trns.html", nil
	})

	assert.Equal(t, NewsletterTrnsReport{Eligible: 3, Attempted: 2, Linked: 1, Failed: 1, SkippedNoMedia: 1, SkippedByLimit: 1}, report)
	assert.Equal(t, 2, processed)
	assert.Equal(t, "https://litter.catbox.moe/trns.html", items[1].TrnsURL)
	assert.Empty(t, items[3].TrnsURL)
}

func TestEffectiveTrnsLimitUsesConfigAndFlagOverride(t *testing.T) {
	cfg := &rss.Config{TrnsConfig: rss.TrnsConfig{DefaultLimit: 3}}

	assert.Equal(t, 3, effectiveTrnsLimit(cfg, &trnsFlags{}))
	assert.Equal(t, 8, effectiveTrnsLimit(cfg, &trnsFlags{limit: 8, limitSet: true}))
	assert.Equal(t, 0, effectiveTrnsLimit(cfg, &trnsFlags{limit: 0, limitSet: true}))
	assert.Equal(t, 0, effectiveTrnsLimit(&rss.Config{}, &trnsFlags{}))
}

func TestRunTrnsLimitFlagOverridesConfigDefaultLimit(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		feedURL := fmt.Sprintf("http://%s/feed.xml", r.Host)
		switch r.URL.Path {
		case "/feed.xml":
			_, _ = fmt.Fprintf(w, `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0" xmlns:podcast="https://podcastindex.org/namespace/1.0">
  <channel>
    <title>Test Podcast</title>
    <link>%s</link>
    <item><title>Ep 1</title><guid>ep-1</guid><link>https://example.com/1</link><podcast:transcript url="http://%s/t1.txt" type="text/plain" /></item>
    <item><title>Ep 2</title><guid>ep-2</guid><link>https://example.com/2</link><podcast:transcript url="http://%s/t2.txt" type="text/plain" /></item>
  </channel>
</rss>`, feedURL, r.Host, r.Host)
		case "/t1.txt":
			_, _ = w.Write([]byte("transcript one"))
		case "/t2.txt":
			_, _ = w.Write([]byte("transcript two"))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)

	cfgFile := writeTrnsTestConfig(t, server.URL+"/feed.xml", 1)

	defaultOut := t.TempDir()
	assert.NoError(t, runTrns(defaultTrnsSource, &trnsFlags{cfgFile: cfgFile, outDir: defaultOut}))
	assert.Len(t, readTrnsIndex(t, defaultOut), 1)

	flagOut := t.TempDir()
	assert.NoError(t, runTrns(defaultTrnsSource, &trnsFlags{cfgFile: cfgFile, outDir: flagOut, limit: 2, limitSet: true}))
	assert.Len(t, readTrnsIndex(t, flagOut), 2)
}

func writeTrnsTestConfig(t *testing.T, feedURL string, limit int) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "rss2nl.yml")
	content := fmt.Sprintf(`newsletter:
  schedule: daily
trns:
  enabled: true
  defaultLimit: %d
  asr:
    enabled: false
  summary:
    enabled: false
  temporaryUpload:
    enabled: false
feeds:
  - type: podcast
    urls:
      - feed: %s
`, limit, feedURL)
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	return path
}

func readTrnsIndex(t *testing.T, outDir string) []trnsIndexEntry {
	t.Helper()

	data, err := os.ReadFile(filepath.Join(outDir, "index.json"))
	if err != nil {
		t.Fatalf("read index: %v", err)
	}
	var entries []trnsIndexEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		t.Fatalf("parse index: %v", err)
	}

	return entries
}

func TestShouldProcessNewsletterTrns_PodcastOnly(t *testing.T) {
	assert.True(t, shouldProcessNewsletterTrns("podcast"))
	assert.True(t, shouldProcessNewsletterTrns(" Podcast "))
	assert.False(t, shouldProcessNewsletterTrns("coding"))
	assert.False(t, shouldProcessNewsletterTrns(""))
}

func TestToTranscriptLinks(t *testing.T) {
	links := toTranscriptLinks([]PodcastTranscriptRef{
		{URL: "https://example.com/transcript.txt", Type: "text/plain"},
		{Type: "text/vtt"},
		{URL: "https://example.com/transcript.vtt", Type: "text/vtt"},
	})

	assert.Len(t, links, 2)
	assert.Equal(t, "https://example.com/transcript.txt", links[0].URL)
	assert.Equal(t, "text/plain", links[0].Type)
	assert.Equal(t, "https://example.com/transcript.vtt", links[1].URL)
}

func TestFetchAndCacheItemTrnsPassesPodcastTranscriptRefsToPipeline(t *testing.T) {
	provider := &recordingTranscriptProvider{}
	cache := transcript.NewCache(t.TempDir())
	item := &NewsletterItem{
		Title: "Episode",
		Link:  "https://example.com/episode",
		PodcastTranscripts: []PodcastTranscriptRef{
			{URL: "https://example.com/transcript.txt", Type: "text/plain"},
		},
	}
	key := cache.Key("Feed", "hash", item.Link, item.Title)

	got, err := fetchAndCacheItemTrns(item, "Feed", key, cache, transcript.NewPipeline(provider))

	assert.NoError(t, err)
	assert.Equal(t, "recorded transcript", got.Content)
	if assert.NotNil(t, provider.seen) && assert.Len(t, provider.seen.TranscriptLinks, 1) {
		assert.Equal(t, "https://example.com/transcript.txt", provider.seen.TranscriptLinks[0].URL)
		assert.Equal(t, "text/plain", provider.seen.TranscriptLinks[0].Type)
	}
}

func TestSetupSummarizerUsesConfiguredModelAndBaseURL(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "sk-test")
	t.Setenv("OPENAI_BASE_URL", "https://env.example/v1")
	t.Setenv("LLM_MODEL", "env-model")

	summarizer := setupSummarizer(&rss.Config{TrnsConfig: rss.TrnsConfig{Summary: rss.TrnsSummaryConfig{
		Enabled:  true,
		Model:    "config-model",
		BaseURL:  "https://config.example/v1",
		Language: "zh",
	}}})

	if assert.NotNil(t, summarizer) {
		assert.Equal(t, "config-model", summarizer.Config.Model)
		assert.Equal(t, "https://config.example/v1", summarizer.Config.BaseURL)
		assert.Equal(t, "zh", summarizer.Language)
	}
}

func TestSetupSummarizerKeepsEnvBaseURLForDefaultConfigBaseURL(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "sk-test")
	t.Setenv("OPENAI_BASE_URL", "https://env.example/v1")

	summarizer := setupSummarizer(&rss.Config{TrnsConfig: rss.TrnsConfig{Summary: rss.TrnsSummaryConfig{
		Enabled: true,
		Model:   "config-model",
		BaseURL: defaultTrnsSummaryBaseURL,
	}}})

	if assert.NotNil(t, summarizer) {
		assert.Equal(t, "https://env.example/v1", summarizer.Config.BaseURL)
	}
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
