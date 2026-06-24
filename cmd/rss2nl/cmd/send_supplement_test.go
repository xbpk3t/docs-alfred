package cmd

import (
	"os"
	"testing"
	"time"

	"github.com/mmcdole/gofeed"
	gofeedext "github.com/mmcdole/gofeed/extensions"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	rss "github.com/xbpk3t/docs-alfred/internal/rss/feed"
	"github.com/xbpk3t/docs-alfred/pkg/md"
)

// ---------------------------------------------------------------------------
// feedDisplayName
// ---------------------------------------------------------------------------

func TestFeedDisplayNameNil(t *testing.T) {
	assert.Empty(t, feedDisplayName(nil))
}

func TestFeedDisplayNameWithTitle(t *testing.T) {
	assert.Equal(t, "My Feed", feedDisplayName(&gofeed.Feed{Title: "My Feed", Link: "https://example.com"}))
}

func TestFeedDisplayNameLinkFallback(t *testing.T) {
	assert.Equal(t, "https://example.com", feedDisplayName(&gofeed.Feed{Link: "https://example.com"}))
}

// ---------------------------------------------------------------------------
// getItemCreationTime
// ---------------------------------------------------------------------------

func TestGetItemCreationTimePublished(t *testing.T) {
	now := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	item := &gofeed.Item{PublishedParsed: &now}
	assert.Equal(t, now, getItemCreationTime(item))
}

func TestGetItemCreationTimeUpdated(t *testing.T) {
	updated := time.Date(2026, 6, 2, 0, 0, 0, 0, time.UTC)
	item := &gofeed.Item{UpdatedParsed: &updated}
	got := getItemCreationTime(item)
	assert.Equal(t, updated, got)
}

func TestGetItemCreationTimeNeither(t *testing.T) {
	before := time.Now()
	item := &gofeed.Item{}
	got := getItemCreationTime(item)
	after := time.Now()
	assert.True(t, !got.Before(before) && !got.After(after))
}

// ---------------------------------------------------------------------------
// getFeedLatestTime
// ---------------------------------------------------------------------------

func TestGetFeedLatestTimeEmpty(t *testing.T) {
	feed := &gofeed.Feed{}
	assert.True(t, getFeedLatestTime(feed).IsZero())
}

func TestGetFeedLatestTimePublished(t *testing.T) {
	now := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	feed := &gofeed.Feed{Items: []*gofeed.Item{{PublishedParsed: &now}}}
	assert.Equal(t, now, getFeedLatestTime(feed))
}

func TestGetFeedLatestTimeUpdated(t *testing.T) {
	updated := time.Date(2026, 6, 2, 0, 0, 0, 0, time.UTC)
	feed := &gofeed.Feed{Items: []*gofeed.Item{{UpdatedParsed: &updated}}}
	assert.Equal(t, updated, getFeedLatestTime(feed))
}

func TestGetFeedLatestTimeNoDates(t *testing.T) {
	feed := &gofeed.Feed{Items: []*gofeed.Item{{Title: "no dates"}}}
	assert.True(t, getFeedLatestTime(feed).IsZero())
}

// ---------------------------------------------------------------------------
// calcPublishFreq
// ---------------------------------------------------------------------------

func TestCalcPublishFreqLessThanTwo(t *testing.T) {
	feed := &gofeed.Feed{Items: []*gofeed.Item{
		{PublishedParsed: timePtr(time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC))},
	}}
	assert.Equal(t, "-", calcPublishFreq(feed))
}

func TestCalcPublishFreqMultiple(t *testing.T) {
	feed := &gofeed.Feed{Items: []*gofeed.Item{
		{PublishedParsed: timePtr(time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC))},
		{PublishedParsed: timePtr(time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC))},
		{PublishedParsed: timePtr(time.Date(2026, 6, 30, 0, 0, 0, 0, time.UTC))},
	}}
	freq := calcPublishFreq(feed)
	assert.NotEqual(t, "-", freq)
	assert.Contains(t, freq, "/Month")
}

func TestCalcPublishFreqEmptyFeed(t *testing.T) {
	feed := &gofeed.Feed{}
	assert.Equal(t, "-", calcPublishFreq(feed))
}

// ---------------------------------------------------------------------------
// firstFeedURL
// ---------------------------------------------------------------------------

func TestFirstFeedURLEmpty(t *testing.T) {
	assert.Empty(t, firstFeedURL(rss.FeedsDetail{}))
}

func TestFirstFeedURLWithFeeds(t *testing.T) {
	assert.Equal(t, "https://a.com/feed", firstFeedURL(rss.FeedsDetail{
		Feeds: []rss.Feeds{{Feed: "https://a.com/feed"}, {Feed: "https://b.com/feed"}},
	}))
}

func TestFirstFeedURLSkipsEmpty(t *testing.T) {
	assert.Equal(t, "https://b.com/feed", firstFeedURL(rss.FeedsDetail{
		Feeds: []rss.Feeds{{Feed: ""}, {Feed: "https://b.com/feed"}},
	}))
}

// ---------------------------------------------------------------------------
// enrichFeedDetails
// ---------------------------------------------------------------------------

func TestEnrichFeedDetailsWithMatch(t *testing.T) {
	service := &NewsletterService{
		feedLastUpdated: map[string]string{"https://a.com/feed": "2026-06-01"},
		feedPublishFreq: map[string]string{"https://a.com/feed": "5/Month"},
	}
	feedList := []rss.FeedsDetail{
		{Type: "tech", Feeds: []rss.Feeds{{Feed: "https://a.com/feed"}}},
	}
	enriched := service.enrichFeedDetails(feedList)
	require.Len(t, enriched, 1)
	assert.Equal(t, "2026-06-01", enriched[0].Feeds[0].LastUpdated)
	assert.Equal(t, "5/Month", enriched[0].Feeds[0].PublishFreq)
}

func TestEnrichFeedDetailsNoMatch(t *testing.T) {
	service := &NewsletterService{
		feedLastUpdated: map[string]string{},
		feedPublishFreq: map[string]string{},
	}
	feedList := []rss.FeedsDetail{
		{Type: "tech", Feeds: []rss.Feeds{{Feed: "https://unknown.com/feed"}}},
	}
	enriched := service.enrichFeedDetails(feedList)
	require.Len(t, enriched, 1)
	assert.Empty(t, enriched[0].Feeds[0].LastUpdated)
}

// ---------------------------------------------------------------------------
// filterStaleFeeds
// ---------------------------------------------------------------------------

func TestFilterStaleFeedsZeroMonths(t *testing.T) {
	service := &NewsletterService{
		config: &rss.Config{DashboardConfig: rss.DashboardConfig{FeedDetail: rss.FeedDetailConfig{StaleMonths: 0}}},
	}
	feedList := []rss.FeedsDetail{{Type: "tech", Feeds: []rss.Feeds{{Feed: "a"}}}}
	result := service.filterStaleFeeds(feedList)
	assert.Len(t, result, 1)
}

func TestFilterStaleFeedsEmptyLastUpdated(t *testing.T) {
	service := &NewsletterService{
		config: &rss.Config{DashboardConfig: rss.DashboardConfig{FeedDetail: rss.FeedDetailConfig{StaleMonths: 3}}},
	}
	feedList := []rss.FeedsDetail{{Type: "tech", Feeds: []rss.Feeds{{Feed: "a", LastUpdated: ""}}}}
	result := service.filterStaleFeeds(feedList)
	assert.Len(t, result, 1) // empty LastUpdated kept
}

func TestFilterStaleFeedsStaleEntry(t *testing.T) {
	service := &NewsletterService{
		config: &rss.Config{DashboardConfig: rss.DashboardConfig{FeedDetail: rss.FeedDetailConfig{StaleMonths: 3}}},
	}
	feedList := []rss.FeedsDetail{{Type: "tech", Feeds: []rss.Feeds{{Feed: "a", LastUpdated: "2020-01-01"}}}}
	result := service.filterStaleFeeds(feedList)
	assert.Len(t, result, 1) // stale entry kept
}

func TestFilterStaleFeedsRecentEntry(t *testing.T) {
	service := &NewsletterService{
		config: &rss.Config{DashboardConfig: rss.DashboardConfig{FeedDetail: rss.FeedDetailConfig{StaleMonths: 3}}},
	}
	feedList := []rss.FeedsDetail{{Type: "tech", Feeds: []rss.Feeds{{Feed: "a", LastUpdated: "2026-06-20"}}}}
	result := service.filterStaleFeeds(feedList)
	assert.Empty(t, result) // recent entry filtered out (not stale)
}

// ---------------------------------------------------------------------------
// generateEmailSubject
// ---------------------------------------------------------------------------

func TestGenerateEmailSubject(t *testing.T) {
	service := NewNewsletterService(&rss.Config{}, "")
	subject := service.generateEmailSubject(NewsletterTpl)
	assert.Contains(t, subject, "Newsletter")
	assert.Contains(t, subject, "第")
	assert.Contains(t, subject, "周")
}

func TestGenerateEmailSubjectDashboard(t *testing.T) {
	service := NewNewsletterService(&rss.Config{}, "")
	subject := service.generateEmailSubject(DashboardTpl)
	assert.Contains(t, subject, "Dashboard")
}

// ---------------------------------------------------------------------------
// addFailedFeedsSection
// ---------------------------------------------------------------------------

func TestAddFailedFeedsSectionDisabled(t *testing.T) {
	doc := md.NewDocument()
	data := &TemplateData{DashboardConfig: rss.DashboardConfig{IsShowFetchFailedFeeds: false}}
	addFailedFeedsSection(doc, data)
	html, err := doc.ToHTML()
	require.NoError(t, err)
	assert.NotContains(t, html, "Fetch Failed")
}

func TestAddFailedFeedsSectionWithFailedFeedsList(t *testing.T) {
	doc := md.NewDocument()
	data := &TemplateData{
		DashboardConfig: rss.DashboardConfig{IsShowFetchFailedFeeds: true},
		DashboardData: struct {
			FailedFeeds   []*rss.FeedError
			FailureReport *rss.FeedFailureReport
			FeedDetails   []rss.FeedsDetail
		}{
			FailedFeeds: []*rss.FeedError{
				{URL: "https://broken.com/feed", Message: "timeout"},
			},
		},
	}
	addFailedFeedsSection(doc, data)
	html, err := doc.ToHTML()
	require.NoError(t, err)
	assert.Contains(t, html, "Fetch Failed Feeds")
}

func TestAddFailedFeedsSectionWithNilFailureReportAndNoFailedFeeds(t *testing.T) {
	doc := md.NewDocument()
	data := &TemplateData{
		DashboardConfig: rss.DashboardConfig{IsShowFetchFailedFeeds: true},
		DashboardData: struct {
			FailedFeeds   []*rss.FeedError
			FailureReport *rss.FeedFailureReport
			FeedDetails   []rss.FeedsDetail
		}{
			FailedFeeds: nil,
		},
	}
	addFailedFeedsSection(doc, data)
	html, err := doc.ToHTML()
	require.NoError(t, err)
	assert.NotContains(t, html, "Fetch Failed Feeds")
}

func TestAddFailedFeedsSectionWithError(t *testing.T) {
	doc := md.NewDocument()
	data := &TemplateData{
		DashboardConfig: rss.DashboardConfig{IsShowFetchFailedFeeds: true},
		DashboardData: struct {
			FailedFeeds   []*rss.FeedError
			FailureReport *rss.FeedFailureReport
			FeedDetails   []rss.FeedsDetail
		}{
			FailedFeeds: []*rss.FeedError{
				{URL: "https://broken.com/feed", Err: assert.AnError},
			},
		},
	}
	addFailedFeedsSection(doc, data)
	html, err := doc.ToHTML()
	require.NoError(t, err)
	assert.Contains(t, html, "Fetch Failed Feeds")
}

// ---------------------------------------------------------------------------
// addFeedDetailsSection
// ---------------------------------------------------------------------------

func TestAddFeedDetailsSectionDisabled(t *testing.T) {
	doc := md.NewDocument()
	data := &TemplateData{DashboardConfig: rss.DashboardConfig{FeedDetail: rss.FeedDetailConfig{Enabled: false}}}
	addFeedDetailsSection(doc, data)
	html, err := doc.ToHTML()
	require.NoError(t, err)
	assert.NotContains(t, html, "Feed")
}

func TestAddFeedDetailsSectionEnabled(t *testing.T) {
	doc := md.NewDocument()
	data := &TemplateData{
		DashboardConfig: rss.DashboardConfig{FeedDetail: rss.FeedDetailConfig{Enabled: true}},
		DashboardData: struct {
			FailedFeeds   []*rss.FeedError
			FailureReport *rss.FeedFailureReport
			FeedDetails   []rss.FeedsDetail
		}{
			FeedDetails: []rss.FeedsDetail{
				{Type: "tech", Feeds: []rss.Feeds{
					{Feed: "https://a.com/feed", LastUpdated: "2026-06-01", PublishFreq: "5/Month"},
					{URL: "https://b.com", LastUpdated: "", PublishFreq: ""},
				}},
			},
		},
	}
	addFeedDetailsSection(doc, data)
	html, err := doc.ToHTML()
	require.NoError(t, err)
	assert.Contains(t, html, "tech")
}

func TestAddFeedDetailsSectionEmpty(t *testing.T) {
	doc := md.NewDocument()
	data := &TemplateData{
		DashboardConfig: rss.DashboardConfig{FeedDetail: rss.FeedDetailConfig{Enabled: true}},
		DashboardData: struct {
			FailedFeeds   []*rss.FeedError
			FailureReport *rss.FeedFailureReport
			FeedDetails   []rss.FeedsDetail
		}{
			FeedDetails: []rss.FeedsDetail{},
		},
	}
	addFeedDetailsSection(doc, data)
}

// ---------------------------------------------------------------------------
// addFeedCategorySection
// ---------------------------------------------------------------------------

func TestAddFeedCategorySectionWithItems(t *testing.T) {
	doc := md.NewDocument()
	data := &TemplateData{
		Feeds: []NewsletterCategory{
			{
				Category: "tech",
				Items: []NewsletterItem{
					{Title: "Post 1", Link: "https://example.com/1", PubDate: "2026-06-01"},
					{Title: "Post 2", PubDate: ""},
				},
			},
		},
	}
	addFeedCategorySection(doc, data)
	html, err := doc.ToHTML()
	require.NoError(t, err)
	assert.Contains(t, html, "tech")
	assert.Contains(t, html, "Post 1")
}

func TestAddFeedCategorySectionEmpty(t *testing.T) {
	doc := md.NewDocument()
	data := &TemplateData{
		Feeds: []NewsletterCategory{
			{Category: "empty", Items: nil},
		},
	}
	addFeedCategorySection(doc, data)
}

// ---------------------------------------------------------------------------
// NewNewsletterService
// ---------------------------------------------------------------------------

func TestNewNewsletterService(t *testing.T) {
	cfg := &rss.Config{}
	s := NewNewsletterService(cfg, "/tmp/trns")
	assert.NotNil(t, s)
	assert.Equal(t, cfg, s.config)
	assert.Equal(t, "/tmp/trns", s.trnsOut)
	assert.NotNil(t, s.failedFeeds)
	assert.Empty(t, s.failedFeeds)
}

// ---------------------------------------------------------------------------
// extractTranscriptRefs edge cases
// ---------------------------------------------------------------------------

func TestExtractTranscriptRefsNoPodcastNS(t *testing.T) {
	item := &gofeed.Item{
		Extensions: gofeedext.Extensions{
			"other": map[string][]gofeedext.Extension{
				"tag": {{Attrs: map[string]string{"url": "https://example.com/t.txt"}}}},
		},
	}
	refs := extractTranscriptRefs(item)
	assert.Nil(t, refs)
}

func TestExtractTranscriptRefsEmptyURL(t *testing.T) {
	item := &gofeed.Item{
		Extensions: gofeedext.Extensions{
			"podcast": map[string][]gofeedext.Extension{
				"transcript": {{Attrs: map[string]string{}}},
			},
		},
	}
	refs := extractTranscriptRefs(item)
	assert.Nil(t, refs)
}

// ---------------------------------------------------------------------------
// itemIdentity edge cases
// ---------------------------------------------------------------------------

func TestItemIdentityUsesLink(t *testing.T) {
	hash := itemIdentity("feed", &gofeed.Item{Link: "https://example.com/post"})
	assert.Len(t, hash, 64)
}

func TestItemIdentityUsesTitle(t *testing.T) {
	hash := itemIdentity("feed", &gofeed.Item{Title: "My Title"})
	assert.Len(t, hash, 64)
}

// ---------------------------------------------------------------------------
// mergeFeedItems edge cases
// ---------------------------------------------------------------------------

func TestMergeFeedItemsNilFeed(t *testing.T) {
	service := NewNewsletterService(&rss.Config{
		FeedConfig:       rss.FeedConfig{FeedLimit: 10},
		NewsletterConfig: rss.NewsletterConfig{Schedule: "daily"},
	}, "")
	fetchMeta := []rss.FetchResult{
		{Feed: nil, URL: "https://example.com"},
	}
	items, err := service.mergeFeedItems("test", fetchMeta, nil)
	assert.NoError(t, err)
	assert.Empty(t, items)
}

func TestMergeFeedItemsFeedLimit(t *testing.T) {
	service := NewNewsletterService(&rss.Config{
		FeedConfig:       rss.FeedConfig{FeedLimit: 1},
		NewsletterConfig: rss.NewsletterConfig{Schedule: "daily"},
	}, "")
	now := time.Now()
	fetchMeta := []rss.FetchResult{
		{
			Feed: &gofeed.Feed{
				Items: []*gofeed.Item{
					{Title: "Item 1", Link: "https://a.com/1", GUID: "1", PublishedParsed: &now},
					{Title: "Item 2", Link: "https://a.com/2", GUID: "2", PublishedParsed: &now},
				},
			},
			URL: "https://a.com/feed",
		},
	}
	items, err := service.mergeFeedItems("test", fetchMeta, map[string]rss.Feeds{
		"https://a.com/feed": {Feed: "https://a.com/feed"},
	})
	assert.NoError(t, err)
	assert.Len(t, items, 1)
}

// ---------------------------------------------------------------------------
// makeNewsletterItem edge cases
// ---------------------------------------------------------------------------

func TestMakeNewsletterItemWithEnclosure(t *testing.T) {
	service := NewNewsletterService(&rss.Config{}, "")
	now := time.Now()
	item := &gofeed.Item{
		Title:           "Episode",
		Link:            "https://example.com/ep",
		PublishedParsed: &now,
		Content:         "full content",
		Enclosures: []*gofeed.Enclosure{
			{URL: "https://example.com/audio.mp3", Type: "audio/mpeg"},
		},
	}
	ni := service.makeNewsletterItem(item, &gofeed.Feed{Title: "Feed"}, "podcast", "hash")
	assert.Equal(t, "https://example.com/audio.mp3", ni.EnclosureURL)
	assert.Equal(t, "audio/mpeg", ni.EnclosureType)
	assert.Equal(t, "full content", ni.Content)
}

func TestMakeNewsletterItemDescriptionFallback(t *testing.T) {
	service := NewNewsletterService(&rss.Config{}, "")
	now := time.Now()
	item := &gofeed.Item{
		Title:           "Episode",
		Link:            "https://example.com/ep",
		PublishedParsed: &now,
		Description:     "desc only",
	}
	ni := service.makeNewsletterItem(item, &gofeed.Feed{Title: "Feed"}, "podcast", "hash")
	assert.Equal(t, "desc only", ni.Content)
}

// ---------------------------------------------------------------------------
// renderItemTitle edge case
// ---------------------------------------------------------------------------

func TestGetItemTitleAuthorEmptyName(t *testing.T) {
	service := NewNewsletterService(&rss.Config{NewsletterConfig: rss.NewsletterConfig{IsHideAuthorInTitle: false}}, "")
	item := &gofeed.Item{Title: "Test", Author: &gofeed.Person{Name: ""}}
	assert.Equal(t, "Test", service.getItemTitle(item))
}

// ---------------------------------------------------------------------------
// loadConfig
// ---------------------------------------------------------------------------

func TestLoadConfigInvalidFile(t *testing.T) {
	_, err := loadConfig("/nonexistent/rss2nl.yml")
	assert.Error(t, err)
}

// ---------------------------------------------------------------------------
// newTrnsCmd structure
// ---------------------------------------------------------------------------

func TestNewTrnsCmdFlags(t *testing.T) {
	cmd := newTrnsCmd()
	assert.Contains(t, cmd.Use, "trns")
	assert.NotNil(t, cmd.Flags().Lookup("out"))
	assert.NotNil(t, cmd.Flags().Lookup("limit"))
	assert.NotNil(t, cmd.Flags().Lookup("refresh"))
	assert.NotNil(t, cmd.PersistentFlags().Lookup("config"))
	assert.NotNil(t, cmd.Flags().Lookup("publish"))
}

func TestNewSendCmdFlags(t *testing.T) {
	cmd := newSendCmd()
	assert.Equal(t, "send", cmd.Use)
	assert.NotNil(t, cmd.Flags().Lookup("config"))
	assert.NotNil(t, cmd.Flags().Lookup("trns-out"))
	assert.NotNil(t, cmd.Flags().Lookup("check"))
}

func TestNewHuntCmdFlags(t *testing.T) {
	cmd := newHuntCmd()
	assert.Equal(t, "hunt", cmd.Use)
	assert.NotNil(t, cmd.Flags().Lookup("config"))
	assert.NotNil(t, cmd.Flags().Lookup("state"))
	assert.NotNil(t, cmd.Flags().Lookup("category"))
	assert.NotNil(t, cmd.Flags().Lookup("providers"))
	assert.NotNil(t, cmd.Flags().Lookup("max"))
	assert.NotNil(t, cmd.Flags().Lookup("per-category"))
	assert.NotNil(t, cmd.Flags().Lookup("provider-max"))
	assert.NotNil(t, cmd.Flags().Lookup("seed-limit"))
	assert.NotNil(t, cmd.Flags().Lookup("report-md"))
	assert.NotNil(t, cmd.Flags().Lookup("report-html"))
	assert.NotNil(t, cmd.Flags().Lookup("report-json"))
	assert.NotNil(t, cmd.Flags().Lookup("blocked-domain"))
	assert.NotNil(t, cmd.Flags().Lookup("new-only"))
	assert.NotNil(t, cmd.Flags().Lookup("dry-run"))
	assert.NotNil(t, cmd.Flags().Lookup("send-mail"))
}

// ---------------------------------------------------------------------------
// huntRunConfig.ApiKey
// ---------------------------------------------------------------------------

func TestHuntRunConfigApiKey(t *testing.T) {
	hc := &huntRunConfig{
		apiKeys: map[huntProvider]string{providerExa: "exa-key", providerTavily: "tavily-key"},
	}
	assert.Equal(t, "exa-key", hc.ApiKey(providerExa))
	assert.Equal(t, "tavily-key", hc.ApiKey(providerTavily))
	assert.Empty(t, hc.ApiKey("unknown"))
}

// ---------------------------------------------------------------------------
// normalizeURL and isBlocked and extractDomain
// ---------------------------------------------------------------------------

func TestNormalizeURLBasic(t *testing.T) {
	result := normalizeURL("https://Example.COM/path")
	assert.NotEmpty(t, result)
}

func TestExtractDomainBasic(t *testing.T) {
	assert.NotEmpty(t, extractDomain("https://example.com/path"))
}

func TestIsBlockedBasic(t *testing.T) {
	blocked := map[string]bool{"facebook.com": true}
	assert.True(t, isBlocked("facebook.com", blocked))
	assert.False(t, isBlocked("example.com", blocked))
}

// ---------------------------------------------------------------------------
// loadHuntState / saveHuntState
// ---------------------------------------------------------------------------

func TestLoadHuntStateNonexistent(t *testing.T) {
	state := loadHuntState("/nonexistent/path/state.json")
	assert.NotNil(t, state)
	assert.NotNil(t, state.Seen)
}

func TestSaveAndLoadHuntState(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/state.json"
	state := &huntState{
		Seen:  map[string]huntSeenRecord{"https://a.com": {Count: 1}},
		Muted: map[string]huntMutedRecord{"https://b.com": {Reason: "spam"}},
	}
	saveHuntState(path, state)

	loaded := loadHuntState(path)
	assert.Len(t, loaded.Seen, 1)
	assert.Len(t, loaded.Muted, 1)
}

// ---------------------------------------------------------------------------
// writeHuntReports
// ---------------------------------------------------------------------------

func TestWriteHuntReports(t *testing.T) {
	dir := t.TempDir()
	report := &huntReport{
		GeneratedAt: "2026-06-23T00:00:00Z",
		Stats:       huntStats{AcceptedCandidates: 1},
		Candidates: []huntCandidate{
			{Title: "Blog", NormalizedURL: "https://blog.example.com", Category: "tech", Domain: "blog.example.com"},
		},
	}
	writeHuntReports(report, dir+"/report.md", dir+"/report.html", dir+"/report.json")
}

// ---------------------------------------------------------------------------
// renderHuntMarkdown
// ---------------------------------------------------------------------------

func TestRenderHuntMarkdownBasic(t *testing.T) {
	report := &huntReport{
		GeneratedAt: "2026-06-23T00:00:00Z",
		DryRun:      true,
		Stats:       huntStats{AcceptedCandidates: 1, CategoriesScanned: 1},
		Candidates: []huntCandidate{
			{Title: "Blog", NormalizedURL: "https://blog.example.com", Category: "tech", Domain: "blog.example.com"},
		},
	}
	result := renderHuntMarkdown(report)
	assert.Contains(t, result, "Source Discovery")
	assert.Contains(t, result, "dry-run")
}

// ---------------------------------------------------------------------------
// renderHuntHTML with fallback
// ---------------------------------------------------------------------------

func TestRenderHuntHTMLFallback(t *testing.T) {
	report := &huntReport{
		GeneratedAt: "2026-06-23T00:00:00Z",
		Stats:       huntStats{},
	}
	html := renderHuntHTML(report)
	assert.NotEmpty(t, html)
}

// ---------------------------------------------------------------------------
// newsletterItemHasTrnsInput
// ---------------------------------------------------------------------------

func TestNewsletterItemHasTrnsInputNil(t *testing.T) {
	assert.False(t, newsletterItemHasTrnsInput(nil))
}

func TestNewsletterItemHasTrnsInputNotMedia(t *testing.T) {
	item := &NewsletterItem{IsMedia: false, EnclosureURL: "https://example.com/a.mp3"}
	assert.False(t, newsletterItemHasTrnsInput(item))
}

func TestNewsletterItemHasTrnsInputWithEnclosure(t *testing.T) {
	item := &NewsletterItem{IsMedia: true, EnclosureURL: "https://example.com/a.mp3"}
	assert.True(t, newsletterItemHasTrnsInput(item))
}

func TestNewsletterItemHasTrnsInputWithTranscripts(t *testing.T) {
	item := &NewsletterItem{IsMedia: true, PodcastTranscripts: []PodcastTranscriptRef{{URL: "t.txt"}}}
	assert.True(t, newsletterItemHasTrnsInput(item))
}

func TestNewsletterItemHasTrnsInputNoInput(t *testing.T) {
	item := &NewsletterItem{IsMedia: true}
	assert.False(t, newsletterItemHasTrnsInput(item))
}

// helper.
func timePtr(t time.Time) *time.Time { return &t }

// ---------------------------------------------------------------------------
// renderNewsletterHTML
// ---------------------------------------------------------------------------

func TestRenderNewsletterHTMLBasic(t *testing.T) {
	service := NewNewsletterService(&rss.Config{
		DashboardConfig: rss.DashboardConfig{},
	}, "")
	data := &TemplateData{
		Title: "Test Newsletter",
		Feeds: []NewsletterCategory{
			{
				Category: "tech",
				Items: []NewsletterItem{
					{Title: "Post 1", Link: "https://example.com/1", PubDate: "2026-06-01"},
				},
			},
		},
	}
	html, err := service.renderNewsletterHTML(data)
	require.NoError(t, err)
	assert.Contains(t, html, "Test Newsletter")
	assert.Contains(t, html, "Post 1")
}

func TestRenderNewsletterHTMLWithSourceHuntURL(t *testing.T) {
	service := NewNewsletterService(&rss.Config{}, "")
	data := &TemplateData{
		Title:         "Newsletter",
		SourceHuntURL: "https://hunt.example.com/report",
	}
	html, err := service.renderNewsletterHTML(data)
	require.NoError(t, err)
	assert.Contains(t, html, "Source Discovery")
}

func TestRenderNewsletterHTMLWithMediaItems(t *testing.T) {
	service := NewNewsletterService(&rss.Config{}, "")
	data := &TemplateData{
		Title: "Newsletter",
		Feeds: []NewsletterCategory{
			{
				Category: "podcast",
				Items: []NewsletterItem{
					{Title: "Episode 1", Link: "https://example.com/ep1", IsMedia: true, TrnsURL: "https://litter.example.com/trns.html"},
				},
			},
		},
	}
	html, err := service.renderNewsletterHTML(data)
	require.NoError(t, err)
	assert.Contains(t, html, "Episode 1")
	assert.Contains(t, html, "trns")
}

// ---------------------------------------------------------------------------
// RenderNewsletter
// ---------------------------------------------------------------------------

func TestRenderNewsletterBasic(t *testing.T) {
	service := NewNewsletterService(&rss.Config{
		DashboardConfig: rss.DashboardConfig{},
	}, "")
	categories := []NewsletterCategory{
		{
			Category: "tech",
			Items:    []NewsletterItem{{Title: "Post 1", Link: "https://example.com/1"}},
		},
	}
	contents, err := service.RenderNewsletter(categories, nil, nil, "")
	require.NoError(t, err)
	require.Len(t, contents, 1)
	assert.Contains(t, contents[0].Subject, "Newsletter")
	assert.Contains(t, contents[0].Content, "Post 1")
}

func TestRenderNewsletterEmpty(t *testing.T) {
	service := NewNewsletterService(&rss.Config{
		DashboardConfig: rss.DashboardConfig{},
	}, "")
	contents, err := service.RenderNewsletter(nil, nil, nil, "")
	require.NoError(t, err)
	require.Len(t, contents, 1)
	assert.NotEmpty(t, contents[0].Subject)
}

func TestRenderNewsletterWithFailedFeeds(t *testing.T) {
	service := NewNewsletterService(&rss.Config{
		DashboardConfig: rss.DashboardConfig{
			IsShowFetchFailedFeeds: true,
		},
	}, "")
	failedFeeds := []*rss.FeedError{
		{URL: "https://broken.com/feed", Message: "timeout"},
	}
	contents, err := service.RenderNewsletter(nil, nil, failedFeeds, "https://hunt.example.com")
	require.NoError(t, err)
	require.Len(t, contents, 1)
	assert.Contains(t, contents[0].Content, "Fetch Failed")
}

// ---------------------------------------------------------------------------
// handleOutput
// ---------------------------------------------------------------------------

func TestHandleOutputDebug(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	require.NoError(t, os.Chdir(dir))
	t.Cleanup(func() { os.Chdir(origDir) })

	service := NewNewsletterService(&rss.Config{
		EnvConfig: rss.EnvConfig{Debug: true},
	}, "")
	err := service.handleOutput([]EmailContent{
		{Subject: "test", Content: "<h1>test</h1>"},
	})
	require.NoError(t, err)

	data, err := os.ReadFile("newsletter_1.html")
	require.NoError(t, err)
	assert.Contains(t, string(data), "<h1>test</h1>")
}

// ---------------------------------------------------------------------------
// addFailedFeedsSection with FailureReport
// ---------------------------------------------------------------------------

func TestAddFailedFeedsSectionWithFailureReport(t *testing.T) {
	doc := md.NewDocument()
	report := &rss.FeedFailureReport{
		TotalFailures: 3,
		Groups: []rss.FeedFailureGroup{
			{
				Host:      "example.com",
				KindLabel: "DNS error",
				Count:     2,
				Status:    "failed",
				ExampleURLs: []string{
					"https://example.com/feed1",
					"https://example.com/feed2",
				},
				RemainingCount: 1,
			},
		},
	}
	data := &TemplateData{
		DashboardConfig: rss.DashboardConfig{IsShowFetchFailedFeeds: true},
		DashboardData: struct {
			FailedFeeds   []*rss.FeedError
			FailureReport *rss.FeedFailureReport
			FeedDetails   []rss.FeedsDetail
		}{
			FailureReport: report,
		},
	}
	addFailedFeedsSection(doc, data)
	html, err := doc.ToHTML()
	require.NoError(t, err)
	assert.Contains(t, html, "Fetch Failed Feeds")
	assert.Contains(t, html, "example.com")
}

// ---------------------------------------------------------------------------
// addFeedCategorySection with trns URL
// ---------------------------------------------------------------------------

func TestAddFeedCategorySectionWithTrnsURL(t *testing.T) {
	doc := md.NewDocument()
	data := &TemplateData{
		Feeds: []NewsletterCategory{
			{
				Category: "podcast",
				Items: []NewsletterItem{
					{Title: "Episode 1", Link: "https://example.com/1", PubDate: "2026-06-01", TrnsURL: "https://litter.example.com/trns.html"},
				},
			},
		},
	}
	addFeedCategorySection(doc, data)
	html, err := doc.ToHTML()
	require.NoError(t, err)
	assert.Contains(t, html, "trns")
	assert.Contains(t, html, "Episode 1")
}

func TestAddFeedCategorySectionWithMediaItem(t *testing.T) {
	doc := md.NewDocument()
	data := &TemplateData{
		Feeds: []NewsletterCategory{
			{
				Category: "podcast",
				Items: []NewsletterItem{
					{Title: "Episode 1", Link: "https://example.com/1", IsMedia: true},
				},
			},
		},
	}
	addFeedCategorySection(doc, data)
	html, err := doc.ToHTML()
	require.NoError(t, err)
	assert.Contains(t, html, "Episode 1")
}

// ---------------------------------------------------------------------------
// addFeedDetailsSection with URL fallback
// ---------------------------------------------------------------------------

func TestAddFeedDetailsSectionWithURLFallback(t *testing.T) {
	doc := md.NewDocument()
	data := &TemplateData{
		DashboardConfig: rss.DashboardConfig{FeedDetail: rss.FeedDetailConfig{Enabled: true}},
		DashboardData: struct {
			FailedFeeds   []*rss.FeedError
			FailureReport *rss.FeedFailureReport
			FeedDetails   []rss.FeedsDetail
		}{
			FeedDetails: []rss.FeedsDetail{
				{Type: "tech", Feeds: []rss.Feeds{
					{URL: "https://b.com", LastUpdated: "", PublishFreq: ""},
				}},
			},
		},
	}
	addFeedDetailsSection(doc, data)
	html, err := doc.ToHTML()
	require.NoError(t, err)
	assert.Contains(t, html, "https://b.com")
}
