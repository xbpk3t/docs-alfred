package cmd

import (
	"testing"

	"github.com/mmcdole/gofeed"
	gofeedext "github.com/mmcdole/gofeed/extensions"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	rss "github.com/xbpk3t/docs-alfred/internal/rss/feed"
	"github.com/xbpk3t/docs-alfred/internal/rss/transcript"
)

// ---------------------------------------------------------------------------
// computeStats
// ---------------------------------------------------------------------------

func TestComputeStatsAllStatuses(t *testing.T) {
	entries := []trnsIndexEntry{
		{Status: statusFound},
		{Status: statusFound},
		{Status: statusCached},
		{Status: statusFailed},
		{Status: "unknown"},
	}
	found, cached, failed := computeStats(entries)
	assert.Equal(t, 2, found)
	assert.Equal(t, 1, cached)
	assert.Equal(t, 2, failed) // "unknown" falls into default
}

func TestComputeStatsEmpty(t *testing.T) {
	found, cached, failed := computeStats(nil)
	assert.Equal(t, 0, found)
	assert.Equal(t, 0, cached)
	assert.Equal(t, 0, failed)
}

// ---------------------------------------------------------------------------
// normalizeTrnsLimit
// ---------------------------------------------------------------------------

func TestNormalizeTrnsLimitVarious(t *testing.T) {
	assert.Equal(t, 0, normalizeTrnsLimit(0))
	assert.Equal(t, 0, normalizeTrnsLimit(-1))
	assert.Equal(t, 0, normalizeTrnsLimit(-100))
	assert.Equal(t, 5, normalizeTrnsLimit(5))
	assert.Equal(t, 1, normalizeTrnsLimit(1))
}

// ---------------------------------------------------------------------------
// trnsLimitReached
// ---------------------------------------------------------------------------

func TestTrnsLimitReachedVarious(t *testing.T) {
	assert.False(t, trnsLimitReached(0, 0), "limit=0 means no limit")
	assert.False(t, trnsLimitReached(0, 5))
	assert.True(t, trnsLimitReached(5, 5))
	assert.True(t, trnsLimitReached(6, 5))
	assert.False(t, trnsLimitReached(4, 5))
}

// ---------------------------------------------------------------------------
// hasTranscriptLinks
// ---------------------------------------------------------------------------

func TestHasTranscriptLinksPodcastExtension(t *testing.T) {
	item := &gofeed.Item{
		Extensions: gofeedext.Extensions{
			"podcast": map[string][]gofeedext.Extension{
				"transcript": {{Attrs: map[string]string{"url": "https://example.com/t.txt"}}},
			},
		},
	}
	assert.True(t, hasTranscriptLinks(item))
}

func TestHasTranscriptLinksAudioEnclosure(t *testing.T) {
	item := &gofeed.Item{
		Enclosures: []*gofeed.Enclosure{{Type: "audio/mpeg"}},
	}
	assert.False(t, hasTranscriptLinks(item))
}

func TestHasTranscriptLinksNone(t *testing.T) {
	item := &gofeed.Item{}
	assert.False(t, hasTranscriptLinks(item))
}

func TestHasTranscriptLinksTranscriptTag(t *testing.T) {
	item := &gofeed.Item{
		Extensions: gofeedext.Extensions{
			"podcast": map[string][]gofeedext.Extension{
				"Transcript": {{Attrs: map[string]string{"url": "t.txt"}}},
			},
		},
	}
	assert.True(t, hasTranscriptLinks(item))
}

func TestHasTranscriptLinksEnclosureWithTranscript(t *testing.T) {
	item := &gofeed.Item{
		Enclosures: []*gofeed.Enclosure{{Type: "text/vtt"}},
	}
	assert.False(t, hasTranscriptLinks(item))
}

// ---------------------------------------------------------------------------
// toEpisodeRef
// ---------------------------------------------------------------------------

func TestToEpisodeRefWithEnclosure(t *testing.T) {
	item := &gofeed.Item{
		Title:       "Episode 1",
		Link:        "https://example.com/ep1",
		GUID:        "ep1",
		Description: "desc",
		Content:     "content",
		Enclosures: []*gofeed.Enclosure{
			{URL: "https://example.com/audio.mp3"},
		},
	}
	ref := toEpisodeRef(item, "Feed Title", "https://feed.com")
	assert.Equal(t, "Episode 1", ref.Title)
	assert.Equal(t, "https://example.com/ep1", ref.URL)
	assert.Equal(t, "ep1", ref.GUID)
	assert.Equal(t, "https://example.com/audio.mp3", ref.EnclosureURL)
	assert.Equal(t, "Feed Title", ref.FeedTitle)
	assert.Equal(t, "https://feed.com", ref.FeedURL)
}

func TestToEpisodeRefWithTranscriptExtensions(t *testing.T) {
	item := &gofeed.Item{
		Title: "Episode 2",
		Link:  "https://example.com/ep2",
		Extensions: gofeedext.Extensions{
			"podcast": map[string][]gofeedext.Extension{
				"transcript": {
					{Attrs: map[string]string{"url": "https://example.com/t.txt", "type": "text/plain"}},
				},
			},
		},
	}
	ref := toEpisodeRef(item, "Feed", "https://feed.com")
	require.Len(t, ref.TranscriptLinks, 1)
	assert.Equal(t, "https://example.com/t.txt", ref.TranscriptLinks[0].URL)
}

func TestToEpisodeRefNoEnclosure(t *testing.T) {
	item := &gofeed.Item{Title: "Episode 3", Link: "https://example.com/ep3"}
	ref := toEpisodeRef(item, "Feed", "https://feed.com")
	assert.Empty(t, ref.EnclosureURL)
}

func TestToEpisodeRefTranscriptExtNoURL(t *testing.T) {
	item := &gofeed.Item{
		Title: "Episode 4",
		Extensions: gofeedext.Extensions{
			"podcast": map[string][]gofeedext.Extension{
				"transcript": {{Attrs: map[string]string{"type": "text/plain"}}},
			},
		},
	}
	ref := toEpisodeRef(item, "Feed", "https://feed.com")
	assert.Empty(t, ref.TranscriptLinks)
}

func TestToEpisodeRefNonPodcastExtension(t *testing.T) {
	item := &gofeed.Item{
		Title: "Episode 5",
		Extensions: gofeedext.Extensions{
			"other": map[string][]gofeedext.Extension{
				"transcript": {{Attrs: map[string]string{"url": "t.txt"}}},
			},
		},
	}
	ref := toEpisodeRef(item, "Feed", "https://feed.com")
	assert.Empty(t, ref.TranscriptLinks)
}

func TestToEpisodeRefNonTranscriptTag(t *testing.T) {
	item := &gofeed.Item{
		Title: "Episode 6",
		Extensions: gofeedext.Extensions{
			"podcast": map[string][]gofeedext.Extension{
				"episode": {{Attrs: map[string]string{"url": "t.txt"}}},
			},
		},
	}
	ref := toEpisodeRef(item, "Feed", "https://feed.com")
	assert.Empty(t, ref.TranscriptLinks)
}

// ---------------------------------------------------------------------------
// toTranscriptLinks
// ---------------------------------------------------------------------------

func TestToTranscriptLinksEmptyURL(t *testing.T) {
	refs := []PodcastTranscriptRef{
		{URL: "", Type: "text/plain"},
		{URL: "https://example.com/t.txt", Type: "text/plain"},
	}
	links := toTranscriptLinks(refs)
	assert.Len(t, links, 1)
}

func TestToTranscriptLinksNil(t *testing.T) {
	links := toTranscriptLinks(nil)
	assert.Empty(t, links)
}

// ---------------------------------------------------------------------------
// summarizeItemTrns
// ---------------------------------------------------------------------------

func TestSummarizeItemTrnsNilSummarizer(t *testing.T) {
	result := summarizeItemTrns(nil, "title", "content")
	assert.Empty(t, result.text)
	assert.Empty(t, result.errText)
}

// ---------------------------------------------------------------------------
// uploadItemTrns
// ---------------------------------------------------------------------------

func TestUploadItemTrnsNilUploader(t *testing.T) {
	url, err := uploadItemTrns(nil, "hash123", "<html>content</html>")
	assert.NoError(t, err)
	assert.Empty(t, url)
}

// ---------------------------------------------------------------------------
// readCachedItemTrns
// ---------------------------------------------------------------------------

func TestReadCachedItemTrnsEmptyCache(t *testing.T) {
	cache := transcript.NewCache(t.TempDir())
	result, err := readCachedItemTrns(cache, "nonexistent-key")
	assert.NoError(t, err)
	assert.False(t, result.ok)
}

// ---------------------------------------------------------------------------
// configuredSummaryBaseURL
// ---------------------------------------------------------------------------

func TestConfiguredSummaryBaseURLEmpty(t *testing.T) {
	cfg := &rss.Config{TrnsConfig: rss.TrnsConfig{Summary: rss.TrnsSummaryConfig{BaseURL: ""}}}
	assert.Equal(t, "", configuredSummaryBaseURL(cfg))
}

func TestConfiguredSummaryBaseURLDefault(t *testing.T) {
	cfg := &rss.Config{TrnsConfig: rss.TrnsConfig{Summary: rss.TrnsSummaryConfig{BaseURL: defaultTrnsSummaryBaseURL}}}
	assert.Equal(t, "", configuredSummaryBaseURL(cfg))
}

func TestConfiguredSummaryBaseURLCustom(t *testing.T) {
	cfg := &rss.Config{TrnsConfig: rss.TrnsConfig{Summary: rss.TrnsSummaryConfig{BaseURL: "https://custom.example/v1"}}}
	assert.Equal(t, "https://custom.example/v1", configuredSummaryBaseURL(cfg))
}

// ---------------------------------------------------------------------------
// setupUploader
// ---------------------------------------------------------------------------

func TestSetupUploaderDisabled(t *testing.T) {
	cfg := &rss.Config{TrnsConfig: rss.TrnsConfig{TemporaryUpload: rss.TrnsTemporaryUploadConfig{Enabled: false}}}
	flags := &trnsFlags{publish: false}
	assert.Nil(t, setupUploader(cfg, flags))
}

func TestSetupUploaderPublishFlag(t *testing.T) {
	cfg := &rss.Config{TrnsConfig: rss.TrnsConfig{TemporaryUpload: rss.TrnsTemporaryUploadConfig{Enabled: false}}}
	flags := &trnsFlags{publish: true}
	uploader := setupUploader(cfg, flags)
	assert.NotNil(t, uploader)
}

func TestSetupUploaderConfigEnabled(t *testing.T) {
	cfg := &rss.Config{TrnsConfig: rss.TrnsConfig{TemporaryUpload: rss.TrnsTemporaryUploadConfig{Enabled: true, ExpirationDuration: "12h"}}}
	flags := &trnsFlags{publish: false}
	uploader := setupUploader(cfg, flags)
	assert.NotNil(t, uploader)
}

// ---------------------------------------------------------------------------
// inspectFeedItems
// ---------------------------------------------------------------------------

func TestInspectFeedItemsEmpty(t *testing.T) {
	feed := &gofeed.Feed{}
	rssCount, audioCount, epCount := inspectFeedItems(feed, 0)
	assert.Equal(t, 0, rssCount)
	assert.Equal(t, 0, audioCount)
	assert.Equal(t, 0, epCount)
}

func TestInspectFeedItemsWithTranscripts(t *testing.T) {
	feed := &gofeed.Feed{
		Items: []*gofeed.Item{
			{
				Title: "Ep 1",
				Extensions: gofeedext.Extensions{
					"podcast": map[string][]gofeedext.Extension{
						"transcript": {{Attrs: map[string]string{"url": "t.txt"}}},
					},
				},
				Enclosures: []*gofeed.Enclosure{{Type: "audio/mpeg"}},
			},
			{
				Title:      "Ep 2",
				Enclosures: []*gofeed.Enclosure{{Type: "audio/mp3"}},
			},
		},
	}
	rssCount, audioCount, epCount := inspectFeedItems(feed, 0)
	assert.Equal(t, 1, rssCount)
	assert.Equal(t, 2, audioCount)
	assert.Equal(t, 2, epCount)
}

func TestInspectFeedItemsWithLimit(t *testing.T) {
	feed := &gofeed.Feed{
		Items: []*gofeed.Item{
			{Title: "Ep 1"},
			{Title: "Ep 2"},
			{Title: "Ep 3"},
		},
	}
	_, _, epCount := inspectFeedItems(feed, 2)
	assert.Equal(t, 2, epCount)
}

func TestInspectFeedItemsMP3Enclosure(t *testing.T) {
	feed := &gofeed.Feed{
		Items: []*gofeed.Item{
			{Title: "Ep", Enclosures: []*gofeed.Enclosure{{Type: "audio/mp3"}}},
		},
	}
	_, audioCount, _ := inspectFeedItems(feed, 0)
	assert.Equal(t, 1, audioCount)
}

// ---------------------------------------------------------------------------
// ProcessNewsletterTrns disabled
// ---------------------------------------------------------------------------

func TestProcessNewsletterTrnsDisabledConfig(t *testing.T) {
	cfg := &rss.Config{TrnsConfig: rss.TrnsConfig{Enabled: false}}
	report := ProcessNewsletterTrns(nil, cfg, t.TempDir())
	assert.Equal(t, NewsletterTrnsReport{}, report)
}

func TestProcessNewsletterTrnsNoTempUpload(t *testing.T) {
	cfg := &rss.Config{TrnsConfig: rss.TrnsConfig{Enabled: true}}
	report := ProcessNewsletterTrns(nil, cfg, t.TempDir())
	assert.Equal(t, NewsletterTrnsReport{}, report)
}

// ---------------------------------------------------------------------------
// effectiveTrnsLimit edge cases
// ---------------------------------------------------------------------------

func TestEffectiveTrnsLimitNilFlags(t *testing.T) {
	cfg := &rss.Config{TrnsConfig: rss.TrnsConfig{DefaultLimit: 5}}
	assert.Equal(t, 5, effectiveTrnsLimit(cfg, nil))
}

func TestEffectiveTrnsLimitNilCfg(t *testing.T) {
	assert.Equal(t, 0, effectiveTrnsLimit(&rss.Config{}, nil))
}

// ---------------------------------------------------------------------------
// readCachedItemTrns with cache hit
// ---------------------------------------------------------------------------

func TestReadCachedItemTrnsCacheHit(t *testing.T) {
	cache := transcript.NewCache(t.TempDir())
	entry := &transcript.CacheEntry{
		EpisodeTitle: "Episode 1",
		Source:       "test",
		ContentType:  "plaintext",
	}
	require.NoError(t, cache.Set("key1", entry, "transcript content"))

	result, err := readCachedItemTrns(cache, "key1")
	assert.NoError(t, err)
	assert.True(t, result.ok)
	assert.Equal(t, "transcript content", result.content)
	assert.Equal(t, "test", result.source)
}

// ---------------------------------------------------------------------------
// renderTrnsPage edge cases
// ---------------------------------------------------------------------------

func TestRenderTrnsPageNoSummaryNoError(t *testing.T) {
	view := &trnsPageView{
		Title:      "Simple Episode",
		FeedTitle:  "Feed",
		EpisodeURL: "https://example.com/ep",
		Status:     "found",
		Content:    "transcript content",
	}
	html := renderTrnsPage(view)
	assert.NotEmpty(t, html)
	assert.Contains(t, html, "Simple Episode")
	assert.NotContains(t, html, "AI Summary")
}

func TestRenderTrnsPageNoEpisodeURL(t *testing.T) {
	view := &trnsPageView{
		Title:     "Episode",
		FeedTitle: "Feed",
		Status:    "cached",
		Content:   "content",
	}
	html := renderTrnsPage(view)
	assert.NotEmpty(t, html)
	assert.Contains(t, html, "Episode")
}

// ---------------------------------------------------------------------------
// summarizeItemTrns with non-nil summarizer (will fail without API)
// ---------------------------------------------------------------------------

func TestSummarizeItemTrnsWithSummarizerNoAPIKey(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("LLM_AxonHub", "")

	cfg := &rss.Config{TrnsConfig: rss.TrnsConfig{Summary: rss.TrnsSummaryConfig{Enabled: true}}}
	summarizer := setupSummarizer(cfg)
	if summarizer != nil {
		// summarizer created but no API key -> GenerateSummary will fail
		result := summarizeItemTrns(summarizer, "title", "content")
		// either empty result or error text
		_ = result
	}
}

// ---------------------------------------------------------------------------
// ProcessNewsletterTrns edge cases
// ---------------------------------------------------------------------------

func TestProcessNewsletterTrnsItemsAllNonMedia(t *testing.T) {
	items := []NewsletterItem{
		{Title: "No media 1", IsMedia: false},
		{Title: "No media 2", IsMedia: false},
	}
	report := processNewsletterTrnsItems(items, 0, nil)
	assert.Equal(t, NewsletterTrnsReport{SkippedNoMedia: 2}, report)
}

func TestProcessNewsletterTrnsItemsZeroLimit(t *testing.T) {
	items := []NewsletterItem{
		{Title: "Media", IsMedia: true, EnclosureURL: "https://example.com/a.mp3"},
	}
	processed := 0
	report := processNewsletterTrnsItems(items, 0, func(item *NewsletterItem) (string, error) {
		processed++
		return "", nil
	})
	// limit=0 means no limit -> should process
	assert.Equal(t, 1, report.Attempted)
	assert.Equal(t, 1, processed)
}

func TestProcessNewsletterTrnsItemsProcessorReturnsEmptyURL(t *testing.T) {
	items := []NewsletterItem{
		{Title: "Media", IsMedia: true, EnclosureURL: "https://example.com/a.mp3"},
	}
	report := processNewsletterTrnsItems(items, 10, func(item *NewsletterItem) (string, error) {
		return "", nil // no URL
	})
	assert.Equal(t, 1, report.Attempted)
	assert.Equal(t, 0, report.Linked)
	assert.Equal(t, 0, report.Failed)
}

// ---------------------------------------------------------------------------
// toEpisodeRef edge cases
// ---------------------------------------------------------------------------

func TestToEpisodeRefTranscriptNonPodcastNamespace(t *testing.T) {
	item := &gofeed.Item{
		Title: "Episode",
		Link:  "https://example.com/ep",
		Extensions: gofeedext.Extensions{
			"atom": map[string][]gofeedext.Extension{
				"transcript": {{Attrs: map[string]string{"url": "t.txt"}}},
			},
		},
	}
	ref := toEpisodeRef(item, "Feed", "https://feed.com")
	// "atom" does not contain "podcast" or "transcript", so links are skipped
	assert.Empty(t, ref.TranscriptLinks)
}

func TestToEpisodeRefMultipleEnclosures(t *testing.T) {
	item := &gofeed.Item{
		Title: "Episode",
		Link:  "https://example.com/ep",
		Enclosures: []*gofeed.Enclosure{
			{URL: "https://example.com/audio1.mp3"},
			{URL: "https://example.com/audio2.mp3"},
		},
	}
	ref := toEpisodeRef(item, "Feed", "https://feed.com")
	assert.Equal(t, "https://example.com/audio1.mp3", ref.EnclosureURL)
}

// ---------------------------------------------------------------------------
// inspectFeedItems with mixed enclosure types
// ---------------------------------------------------------------------------

func TestInspectFeedItemsMixedEnclosures(t *testing.T) {
	feed := &gofeed.Feed{
		Items: []*gofeed.Item{
			{
				Title: "Ep 1",
				Enclosures: []*gofeed.Enclosure{
					{Type: "video/mp4"},
					{Type: "audio/mpeg"},
				},
			},
			{
				Title: "Ep 2",
				Enclosures: []*gofeed.Enclosure{
					{Type: "text/html"},
				},
			},
		},
	}
	_, audioCount, epCount := inspectFeedItems(feed, 0)
	assert.Equal(t, 1, audioCount)
	assert.Equal(t, 2, epCount)
}

func TestInspectFeedItemsMPEGEnclosure(t *testing.T) {
	feed := &gofeed.Feed{
		Items: []*gofeed.Item{
			{Title: "Ep", Enclosures: []*gofeed.Enclosure{{Type: "audio/mpeg"}}},
		},
	}
	_, audioCount, _ := inspectFeedItems(feed, 0)
	assert.Equal(t, 1, audioCount)
}

// ---------------------------------------------------------------------------
// configuredSummaryBaseURL edge cases
// ---------------------------------------------------------------------------

func TestConfiguredSummaryBaseURLWhitespace(t *testing.T) {
	cfg := &rss.Config{TrnsConfig: rss.TrnsConfig{Summary: rss.TrnsSummaryConfig{BaseURL: "  "}}}
	assert.Equal(t, "", configuredSummaryBaseURL(cfg))
}

// ---------------------------------------------------------------------------
// hasTranscriptLinks edge cases
// ---------------------------------------------------------------------------

func TestHasTranscriptLinksMixedExtensions(t *testing.T) {
	item := &gofeed.Item{
		Extensions: gofeedext.Extensions{
			"podcast": map[string][]gofeedext.Extension{
				"episode":    {{Attrs: map[string]string{"url": "t.txt"}}},
				"transcript": {{Attrs: map[string]string{"url": "t2.txt"}}},
			},
		},
	}
	assert.True(t, hasTranscriptLinks(item))
}

func TestHasTranscriptLinksMultipleNamespaces(t *testing.T) {
	item := &gofeed.Item{
		Extensions: gofeedext.Extensions{
			"podcast": map[string][]gofeedext.Extension{
				"episode": {{Attrs: map[string]string{}}},
			},
			"other": map[string][]gofeedext.Extension{
				"transcript": {{Attrs: map[string]string{"url": "t.txt"}}},
			},
		},
	}
	assert.True(t, hasTranscriptLinks(item))
}
