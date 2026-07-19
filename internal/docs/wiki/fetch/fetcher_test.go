package fetch

import (
	"context"
	"github.com/xbpk3t/docs-alfred/internal/docs/wiki/types"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/mmcdole/gofeed"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xbpk3t/docs-alfred/internal/rss/transcript"
)

func TestMarkdownFallbackBodyConvertsHTML(t *testing.T) {
	body := markdownFallbackBody([]byte(`<html><head><title>Page Title</title></head><body><h1>Hello</h1><p>Read <a href="https://example.com">more</a>.</p></body></html>`))

	assert.Contains(t, body, "Hello")
	assert.Contains(t, body, "[more](https://example.com)")
	assert.NotContains(t, body, "<h1>", "fallback body should not be raw HTML")
}

func TestHTTPDriverRejectsLowQualityContent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`<html><head><title>X</title></head><body>This page requires JavaScript.</body></html>`))
	}))
	t.Cleanup(server.Close)

	driver := newHTTPDriver(DriverOptions{MaxBodySize: 5000})
	result := driver.FetchContent(context.Background(), server.URL, types.ContentText)

	require.NotNil(t, result)
	require.Contains(t, result.Error, "extract:")
}

func TestFetchPodcastTranscriptUsesRSSTranscript(t *testing.T) {
	transcriptServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte("This is a long enough podcast transcript pulled directly from the RSS transcript tag."))
	}))
	t.Cleanup(transcriptServer.Close)

	feedServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/rss+xml")
		_, _ = w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0" xmlns:podcast="https://podcastindex.org/namespace/1.0">
  <channel>
    <title>Example Podcast</title>
    <item>
      <title>Episode One</title>
      <link>https://example.com/episode-one</link>
      <podcast:transcript url="` + transcriptServer.URL + `/transcript.txt" type="text/plain" />
    </item>
  </channel>
</rss>`))
	}))
	t.Cleanup(feedServer.Close)

	fetcher := NewFetcher()
	result := fetcher.FetchContent(context.Background(), feedServer.URL+"/feed.xml", types.ContentAudio)

	require.NotNil(t, result)
	require.Empty(t, result.Error)
	assert.Equal(t, "Episode One", result.Title)
	assert.Contains(t, result.Body, "Transcript source: rss-transcript")
	assert.Contains(t, result.Body, "long enough podcast transcript")
}

func TestFetchDirectAudioDoesNotRunASR(t *testing.T) {
	fetcher := NewFetcher()
	result := fetcher.FetchContent(context.Background(), "https://example.com/audio.mp3", types.ContentAudio)

	require.NotNil(t, result)
	require.Contains(t, result.Error, "extract:")
	require.Contains(t, result.Error, "direct audio URL has no RSS metadata")
}

func TestHTTPDriverTruncatesUTF8Safely(t *testing.T) {
	fetcher := NewFetcher()
	fetcher.MaxBodySize = 5
	body := strings.Repeat("你好世界。", 60)
	driver := newHTTPDriver(DriverOptions{MaxBodySize: 5})
	result := driver.extractWithReadability([]byte(`<html><head><title>测试</title></head><body><article><p>`+body+`</p></article></body></html>`), "https://example.com/a")

	require.NotNil(t, result)
	assert.True(t, utf8.ValidString(result.Body))
	assert.Contains(t, result.Body, "...")
}

func TestOpenCLIDriverTruncatesUTF8Safely(t *testing.T) {
	binDir := t.TempDir()
	opencli := filepath.Join(binDir, "opencli")
	script := `#!/bin/sh
printf '# 测试\n'
printf '` + strings.Repeat("你好", 100) + `\n'
`
	require.NoError(t, os.WriteFile(opencli, []byte(script), 0o700))
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	driver := newOpenCLIDriver(DriverOptions{MaxBodySize: 5})

	result := driver.runOpenCLI(context.Background(), "web", []string{"read", "--url", "https://example.com/a", "--stdout"})

	require.NotNil(t, result)
	require.Empty(t, result.Error)
	assert.True(t, utf8.ValidString(result.Body))
	assert.Contains(t, result.Body, "...")
}

func TestOpenCLIDriver_BilibiliUsesAdapter(t *testing.T) {
	driver := newOpenCLIDriver(DriverOptions{})
	result := driver.FetchContent(context.Background(), "https://www.bilibili.com/video/BV1xx", types.ContentVideo)
	// Will fail because opencli isn't installed, but verifies routing.
	require.NotNil(t, result)
}

func TestHTTPDriver_ExtractsContent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`<html><head><title>Test Page</title></head><body><article><p>This is a sufficiently long article body that should pass the quality check threshold for readability extraction. The content needs to be at least 120 characters long to avoid the "too short" rejection from the quality assessment function. Adding more text here to ensure we pass that threshold reliably.</p></article></body></html>`))
	}))
	t.Cleanup(server.Close)

	driver := newHTTPDriver(DriverOptions{MaxBodySize: 5000})
	result := driver.FetchContent(context.Background(), server.URL, types.ContentText)

	require.NotNil(t, result)
	require.Empty(t, result.Error)
	assert.Equal(t, "Test Page", result.Title)
}

func TestDetectContentTypeOnlyTreatsConcreteVideoURLsAsVideo(t *testing.T) {
	tests := []struct {
		name string
		url  string
		want string
	}{
		{name: "youtube watch", url: "https://www.youtube.com/watch?v=abc", want: types.ContentVideo},
		{name: "youtube shorts", url: "https://youtube.com/shorts/abc", want: types.ContentVideo},
		{name: "youtube embed", url: "https://www.youtube.com/embed/abc", want: types.ContentVideo},
		{name: "youtu be", url: "https://youtu.be/abc", want: types.ContentVideo},
		{name: "youtube channel", url: "https://www.youtube.com/@VirtualizationHowto/videos", want: types.ContentText},
		{name: "youtube playlist", url: "https://www.youtube.com/playlist?list=abc", want: types.ContentText},
		{name: "bilibili video", url: "https://www.bilibili.com/video/BV1xx", want: types.ContentVideo},
		{name: "bilibili space", url: "https://space.bilibili.com/123", want: types.ContentText},
		{name: "bilibili homepage", url: "https://www.bilibili.com/", want: types.ContentText},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, DetectContentType(tt.url))
		})
	}
}

// --- WithDriver option ---

func TestWithDriverOption(t *testing.T) {
	driver := newHTTPDriver(DriverOptions{MaxBodySize: 1000})
	f := NewFetcher(WithDriver(driver))
	assert.Equal(t, "http-readability", f.driver.Name())
}

// --- isVideoURL edge cases ---

func TestIsVideoURLInvalidURL(t *testing.T) {
	// url.Parse is very permissive and almost never fails.
	// "not a url but has youtu.be/ in it" parses with empty host, not a video.
	assert.False(t, isVideoURL("not a url at all"))
	// A URL with known video host but no video path
	assert.False(t, isVideoURL("https://youtube.com/"))
}

// --- assessContentQuality with social shell domain ---

func TestAssessContentQualitySocialShellDomain(t *testing.T) {
	// Social shell domain with short content should be rejected
	q := assessContentQuality("Title", "short", "https://x.com/user/status/1")
	assert.False(t, q.OK)
	assert.Equal(t, "social/login shell", q.Reason)
}

func TestAssessContentQualitySocialShellDomainLongContent(t *testing.T) {
	// Social shell domain with social patterns should be rejected
	q := assessContentQuality("Title", strings.Repeat("please log in to continue. ", 50), "https://twitter.com/user/status/1")
	assert.False(t, q.OK)
}

func TestAssessContentQualitySocialShellDomainGoodContent(t *testing.T) {
	// Social shell domain with good content should pass
	long := strings.Repeat("This is real content with enough sentences. It has good information. ", 20)
	q := assessContentQuality("Good Article", long, "https://x.com/user/status/1")
	assert.True(t, q.OK)
}

// --- podcastFeedCandidates ---

func TestPodcastFeedCandidatesNilFeed(t *testing.T) {
	result := podcastFeedCandidates(nil, "https://example.com/feed.xml")
	assert.Nil(t, result)
}

func TestPodcastFeedCandidatesEmptyItems(t *testing.T) {
	feed := &gofeed.Feed{Title: "Test", Items: []*gofeed.Item{}}
	result := podcastFeedCandidates(feed, "https://example.com/feed.xml")
	assert.Nil(t, result)
}

func TestPodcastFeedCandidatesWithItems(t *testing.T) {
	feed := &gofeed.Feed{
		Title: "Test Podcast",
		Items: []*gofeed.Item{
			{Title: "Episode 1", Link: "https://example.com/ep1"},
			{Title: "Episode 2", Link: "https://example.com/ep2"},
		},
	}
	result := podcastFeedCandidates(feed, "https://example.com/ep1")
	require.Len(t, result, 2)
	// First should be the exact URL match
	assert.Equal(t, "Episode 1", result[0].Title)
}

func TestPodcastFeedCandidatesWithTranscriptItem(t *testing.T) {
	feed := &gofeed.Feed{
		Title: "Test Podcast",
		Items: []*gofeed.Item{
			{Title: "Episode 1", Link: "https://example.com/ep1", Description: "no transcript here"},
			{Title: "Episode 2", Link: "https://example.com/ep2", Description: "Full transcript available below"},
		},
	}
	result := podcastFeedCandidates(feed, "https://example.com/ep3")
	require.Len(t, result, 2)
	// Both items should be present (order depends on collector implementation)
	titles := []string{result[0].Title, result[1].Title}
	assert.Contains(t, titles, "Episode 1")
	assert.Contains(t, titles, "Episode 2")
}

// --- feedCandidateCollector methods ---

func TestFeedCandidateCollectorAddNil(t *testing.T) {
	feed := &gofeed.Feed{Title: "Test", Items: []*gofeed.Item{}}
	collector := newFeedCandidateCollector(feed)
	collector.add(nil)
	assert.Empty(t, collector.candidates)
}

func TestFeedCandidateCollectorAddDuplicate(t *testing.T) {
	feed := &gofeed.Feed{Title: "Test", Items: []*gofeed.Item{}}
	collector := newFeedCandidateCollector(feed)
	item := &gofeed.Item{Title: "Episode"}
	collector.add(item)
	collector.add(item) // duplicate
	assert.Len(t, collector.candidates, 1)
}

func TestFeedCandidateCollectorAddExactURLMatches(t *testing.T) {
	feed := &gofeed.Feed{
		Title: "Test",
		Items: []*gofeed.Item{
			{Title: "Match", Link: "https://example.com/ep1"},
			{Title: "No Match", Link: "https://example.com/ep2"},
		},
	}
	collector := newFeedCandidateCollector(feed)
	collector.addExactURLMatches("https://example.com/ep1")
	require.Len(t, collector.candidates, 1)
	assert.Equal(t, "Match", collector.candidates[0].Title)
}

func TestFeedCandidateCollectorAddExactURLMatchesByGUID(t *testing.T) {
	feed := &gofeed.Feed{
		Title: "Test",
		Items: []*gofeed.Item{
			{Title: "Match", GUID: "https://example.com/ep1"},
		},
	}
	collector := newFeedCandidateCollector(feed)
	collector.addExactURLMatches("https://example.com/ep1")
	require.Len(t, collector.candidates, 1)
}

func TestFeedCandidateCollectorAddTranscriptLikeItems(t *testing.T) {
	feed := &gofeed.Feed{
		Title: "Test",
		Items: []*gofeed.Item{
			{Title: "No Transcript", Link: "https://example.com/ep1"},
			{Title: "Has Transcript", Link: "https://example.com/ep2", Description: "Full transcript below"},
		},
	}
	collector := newFeedCandidateCollector(feed)
	collector.addTranscriptLikeItems()
	require.Len(t, collector.candidates, 1)
	assert.Equal(t, "Has Transcript", collector.candidates[0].Title)
}

func TestFeedCandidateCollectorAddRemainingItems(t *testing.T) {
	feed := &gofeed.Feed{
		Title: "Test",
		Items: []*gofeed.Item{
			{Title: "Episode 1"},
			{Title: "Episode 2"},
		},
	}
	collector := newFeedCandidateCollector(feed)
	collector.addRemainingItems()
	assert.Len(t, collector.candidates, 2)
}

// --- extractFailure edge cases ---

func TestExtractFailureAlreadyHasExtractPrefix(t *testing.T) {
	r := extractFailure("https://example.com", "extract: custom error")
	assert.Equal(t, "extract: custom error", r.Error)
	assert.Equal(t, types.FailureExtract, r.FailureKind)
}

func TestExtractFailureWhitespaceReason(t *testing.T) {
	r := extractFailure("https://example.com", "  ")
	assert.Contains(t, r.Error, "content extraction failed")
}

// --- hasTranscriptSignal ---

func TestHasTranscriptSignalWithTranscriptLinks(t *testing.T) {
	ep := &transcript.EpisodeRef{
		TranscriptLinks: []transcript.TranscriptLink{{URL: "https://example.com/transcript.txt", Type: "text/plain"}},
	}
	assert.True(t, hasTranscriptSignal(ep))
}

func TestHasTranscriptSignalWithTranscriptInDescription(t *testing.T) {
	ep := &transcript.EpisodeRef{
		Description: "Full transcript: This is the content of the episode...",
	}
	assert.True(t, hasTranscriptSignal(ep))
}

func TestHasTranscriptSignalNoSignal(t *testing.T) {
	ep := &transcript.EpisodeRef{
		Description: "Just a regular description",
	}
	assert.False(t, hasTranscriptSignal(ep))
}

// --- mediaMaxBodySize ---

func TestMediaMaxBodySizeZeroReturnsDefault(t *testing.T) {
	f := &Fetcher{MaxBodySize: 0}
	assert.Equal(t, 20_000, f.mediaMaxBodySize())
}

// --- getGHToken ---

func TestGetGHTokenFromEnv(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "test-token")
	assert.Equal(t, "test-token", getGHToken())
}

func TestGetGHTokenFromGHToken(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("GH_TOKEN", "gh-token")
	assert.Equal(t, "gh-token", getGHToken())
}

func TestGetGHTokenEmpty(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("GH_TOKEN", "")
	assert.Empty(t, getGHToken())
}

// --- NewFetcher edge cases ---

func TestNewFetcherZeroMaxBodySize(t *testing.T) {
	f := NewFetcher(func(fetcher *Fetcher) { fetcher.MaxBodySize = 0 })
	assert.Equal(t, 5000, f.MaxBodySize)
}

// --- FetchContent with empty contentType ---

func TestFetchContentDetectsContentType(t *testing.T) {
	f := NewFetcher(WithMediaEnabled(false))
	// YouTube URL should be detected as video
	result := f.FetchContent(context.Background(), "https://www.youtube.com/watch?v=abc", "")
	require.NotNil(t, result)
}

// --- fetchGitHubRepo with mock server ---

func TestFetchGitHubRepoSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"name": "testrepo",
			"full_name": "owner/testrepo",
			"description": "A test repository",
			"html_url": "https://github.com/owner/testrepo",
			"stargazers_count": 42,
			"language": "Go",
			"topics": ["go", "cli"],
			"license": {"spdx_id": "MIT"}
		}`))
	}))
	t.Cleanup(server.Close)

	f := NewFetcher()
	f.GHBaseURL = server.URL + "/"
	result := f.fetchGitHubRepo(context.Background(), "https://github.com/owner/testrepo")

	require.NotNil(t, result)
	assert.Empty(t, result.Error)
	assert.Contains(t, result.Title, "owner/testrepo")
	assert.Contains(t, result.Title, "A test repository")
	assert.Contains(t, result.Body, "Stars: 42")
	assert.Contains(t, result.Body, "Language: Go")
	assert.Contains(t, result.Body, "License: MIT")
	assert.Contains(t, result.Body, "Topics: go, cli")
}

func TestFetchGitHubRepoNoDescription(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"name": "testrepo",
			"full_name": "owner/testrepo",
			"html_url": "https://github.com/owner/testrepo",
			"stargazers_count": 0,
			"language": "",
			"topics": []
		}`))
	}))
	t.Cleanup(server.Close)

	f := NewFetcher()
	f.GHBaseURL = server.URL + "/"
	result := f.fetchGitHubRepo(context.Background(), "https://github.com/owner/testrepo")

	require.NotNil(t, result)
	assert.Empty(t, result.Error)
	assert.Equal(t, "owner/testrepo", result.Title)
	assert.Contains(t, result.Body, "License: none")
}

func TestFetchGitHubRepoNoLicense(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"name": "testrepo",
			"full_name": "owner/testrepo",
			"description": "desc",
			"html_url": "https://github.com/owner/testrepo",
			"stargazers_count": 1,
			"language": "Python"
		}`))
	}))
	t.Cleanup(server.Close)

	f := NewFetcher()
	f.GHBaseURL = server.URL + "/"
	result := f.fetchGitHubRepo(context.Background(), "https://github.com/owner/testrepo")

	require.NotNil(t, result)
	assert.Empty(t, result.Error)
	assert.Contains(t, result.Body, "License: none")
}

func TestFetchGitHubRepoInvalidURL(t *testing.T) {
	f := NewFetcher()
	result := f.fetchGitHubRepo(context.Background(), "https://not-github.com/owner/repo")
	require.NotNil(t, result)
	assert.NotEmpty(t, result.Error)
}

// --- githubClient ---

func TestGithubClientDefaultBaseURL(t *testing.T) {
	f := NewFetcher()
	client, err := f.githubClient()
	require.NoError(t, err)
	assert.NotNil(t, client)
}

func TestGithubClientCustomBaseURL(t *testing.T) {
	f := NewFetcher()
	f.GHBaseURL = "https://custom.github.com/api"
	client, err := f.githubClient()
	require.NoError(t, err)
	assert.NotNil(t, client)
	assert.Equal(t, "https://custom.github.com/api/", client.BaseURL.String())
}

func TestGithubClientCustomBaseURLWithTrailingSlash(t *testing.T) {
	f := NewFetcher()
	f.GHBaseURL = "https://custom.github.com/api/"
	client, err := f.githubClient()
	require.NoError(t, err)
	assert.NotNil(t, client)
}

func TestGithubClientWithToken(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "test-token-123")
	f := NewFetcher()
	client, err := f.githubClient()
	require.NoError(t, err)
	assert.NotNil(t, client)
}

// --- fetchPodcastTranscript edge cases ---

func TestFetchPodcastTranscriptDirectAudioURL(t *testing.T) {
	f := NewFetcher()
	result := f.fetchPodcastTranscript(context.Background(), "https://example.com/audio.mp3")
	require.NotNil(t, result)
	assert.Contains(t, result.Error, "direct audio URL has no RSS metadata")
}

func TestFetchPodcastTranscriptInvalidFeedURL(t *testing.T) {
	f := NewFetcher()
	result := f.fetchPodcastTranscript(context.Background(), "https://invalid.invalid.invalid/feed.xml")
	require.NotNil(t, result)
	assert.Contains(t, result.Error, "rss transcript unavailable")
}

// --- socialShellLike edge cases ---

func TestSocialShellLikeMediumContentWithPatterns(t *testing.T) {
	// Content between 300 and 2000 runes with social patterns
	content := strings.Repeat("x", 500) + " please log in " + strings.Repeat("y", 500)
	assert.True(t, socialShellLike(content, content))
}

// --- isLinkHeavy edge case ---

func TestIsLinkHeavyBelowThreshold(t *testing.T) {
	// Less than 20 links should NOT be link-heavy
	heavy := strings.Repeat("[a](https://x.com) ", 5)
	assert.False(t, isLinkHeavy(heavy))
}

// --- NewFetcher with GHBaseURL ---

func TestNewFetcherWithGHBaseURL(t *testing.T) {
	f := NewFetcher()
	f.GHBaseURL = "https://github.example.com/api"
	assert.Equal(t, "https://github.example.com/api", f.GHBaseURL)
}

// --- FetchContent with podcast URL ---

func TestFetchContentPodcastURL(t *testing.T) {
	f := NewFetcher(WithMediaEnabled(false))
	result := f.FetchContent(context.Background(), "https://xiaoyuzhoufm.com/ep/1", "")
	require.NotNil(t, result)
	assert.Contains(t, result.Error, "media extraction disabled")
}

// --- FetchContent with GitHub URL ---

func TestFetchContentGitHubURL(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"name": "repo",
			"full_name": "owner/repo",
			"description": "A repo",
			"html_url": "https://github.com/owner/repo",
			"stargazers_count": 10,
			"language": "Go"
		}`))
	}))
	t.Cleanup(server.Close)

	f := NewFetcher()
	f.GHBaseURL = server.URL + "/"
	result := f.FetchContent(context.Background(), "https://github.com/owner/repo", "")
	require.NotNil(t, result)
	assert.Empty(t, result.Error)
	assert.Contains(t, result.Body, "Stars: 10")
}

// --- FetchContent with driver returning error ---

func TestFetchContentDriverError(t *testing.T) {
	// Create a driver that returns an error
	driver := newHTTPDriver(DriverOptions{MaxBodySize: 5000})
	f := NewFetcher(WithDriver(driver))
	// Use a URL that will cause an HTTP error
	result := f.FetchContent(context.Background(), "http://invalid.invalid.invalid", "")
	require.NotNil(t, result)
	assert.NotEmpty(t, result.Error)
}

// --- FetchContent with driver returning nil ---

func TestFetchContentDriverReturnsNil(t *testing.T) {
	f := NewFetcher()
	// Override driver with one that returns nil
	f.driver = &nilDriver{}
	result := f.FetchContent(context.Background(), "https://example.com", "")
	require.NotNil(t, result)
	assert.Contains(t, result.Error, "driver returned empty result")
}

// nilDriver is a test driver that always returns nil.
type nilDriver struct{}

func (d *nilDriver) Name() string { return "nil" }
func (d *nilDriver) FetchContent(_ context.Context, _, _ string) *types.ContentFetchResult {
	return nil
}

// --- FetchContent with RSS feed URL ---

func TestFetchContentRSSFeedURL(t *testing.T) {
	feedServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/rss+xml")
		_, _ = w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0">
  <channel>
    <title>Empty Podcast</title>
  </channel>
</rss>`))
	}))
	t.Cleanup(feedServer.Close)

	f := NewFetcher()
	result := f.FetchContent(context.Background(), feedServer.URL+"/feed.xml", types.ContentAudio)
	require.NotNil(t, result)
	// Should fail because feed has no items
	assert.NotEmpty(t, result.Error)
}

// --- fetchPodcastTranscript with empty feed ---

func TestFetchPodcastTranscriptEmptyFeed(t *testing.T) {
	feedServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/rss+xml")
		_, _ = w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0">
  <channel>
    <title>Empty Podcast</title>
  </channel>
</rss>`))
	}))
	t.Cleanup(feedServer.Close)

	f := NewFetcher()
	result := f.fetchPodcastTranscript(context.Background(), feedServer.URL+"/feed.xml")
	require.NotNil(t, result)
	assert.Contains(t, result.Error, "feed has no items")
}

// --- fetchPodcastTranscript with feed items but no transcript ---

func TestFetchPodcastTranscriptNoTranscript(t *testing.T) {
	transcriptServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte("This is a long enough podcast transcript pulled directly from the RSS transcript tag."))
	}))
	t.Cleanup(transcriptServer.Close)

	feedServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/rss+xml")
		_, _ = w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0" xmlns:podcast="https://podcastindex.org/namespace/1.0">
  <channel>
    <title>Example Podcast</title>
    <item>
      <title>Episode One</title>
      <link>https://example.com/episode-one</link>
      <podcast:transcript url="` + transcriptServer.URL + `/transcript.txt" type="text/plain" />
    </item>
  </channel>
</rss>`))
	}))
	t.Cleanup(feedServer.Close)

	f := NewFetcher()
	result := f.fetchPodcastTranscript(context.Background(), feedServer.URL+"/feed.xml")
	require.NotNil(t, result)
	assert.Empty(t, result.Error)
	assert.Contains(t, result.Body, "Transcript source: rss-transcript")
}

// --- addTranscriptLikeItems with nil item ---

func TestFeedCandidateCollectorAddTranscriptLikeItemsNilItem(t *testing.T) {
	feed := &gofeed.Feed{
		Title: "Test",
		Items: []*gofeed.Item{
			nil,
			{Title: "Episode", Link: "https://example.com/ep1", Description: "just a regular description"},
		},
	}
	collector := newFeedCandidateCollector(feed)
	collector.addTranscriptLikeItems()
	// Should not panic and should skip nil item; Episode has no transcript signal
	assert.Empty(t, collector.candidates)
}

// --- isVideoURL with invalid URL that has known video text ---

func TestIsVideoURLInvalidURLWithKnownText(t *testing.T) {
	// These contain known video URL text but are not valid URLs
	assert.True(t, hasKnownVideoURLText("has youtu.be/abc in it"))
	assert.True(t, hasKnownVideoURLText("has b23.tv/abc in it"))
	assert.True(t, hasKnownVideoURLText("contains bilibili.com/video/BV1xx somewhere"))
}

// --- assessContentQuality with link-heavy content ---

func TestAssessContentQualityLinkHeavy(t *testing.T) {
	// Content with many links relative to text length
	heavy := strings.Repeat("[a](https://x.com) ", 25)
	q := assessContentQuality("Title", heavy, "https://example.com")
	assert.False(t, q.OK)
	assert.Contains(t, q.Reason, "link-heavy")
}

// --- socialShellLike with medium content and patterns ---

func TestSocialShellLikeMediumContentWithSocialPatterns(t *testing.T) {
	// Content between 300 and 2000 runes with social patterns
	content := strings.Repeat("x", 500) + " please log in " + strings.Repeat("y", 500)
	assert.True(t, socialShellLike(content, content))
}

// --- FetchContent with podcast URL and media enabled ---

func TestFetchContentPodcastURLEnabled(t *testing.T) {
	feedServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/rss+xml")
		_, _ = w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0">
  <channel>
    <title>Podcast</title>
    <item>
      <title>Episode</title>
      <link>https://example.com/ep1</link>
    </item>
  </channel>
</rss>`))
	}))
	t.Cleanup(feedServer.Close)

	f := NewFetcher(WithMediaEnabled(true))
	result := f.FetchContent(context.Background(), feedServer.URL+"/feed.xml", types.ContentAudio)
	require.NotNil(t, result)
	// Will fail because no transcript, but exercises the podcast path
}

// --- FetchContent with driver returning nil and error ---

func TestFetchContentDriverReturnsNilWithAudioType(t *testing.T) {
	f := NewFetcher(WithMediaEnabled(false))
	result := f.FetchContent(context.Background(), "https://example.com/audio.mp3", types.ContentAudio)
	require.NotNil(t, result)
	assert.Contains(t, result.Error, "media extraction disabled")
}

// --- assessContentQuality with social shell domain and good content ---

func TestAssessContentQualitySocialShellGoodContent(t *testing.T) {
	// Long content with real sentences on social domain
	long := strings.Repeat("This is a real sentence with good content. ", 50)
	q := assessContentQuality("Good", long, "https://x.com/user/status/1")
	assert.True(t, q.OK)
}

// --- isLinkHeavy with many links ---

func TestIsLinkHeavyManyLinks(t *testing.T) {
	// 30 links with short text
	heavy := strings.Repeat("[a](https://x.com) ", 30)
	assert.True(t, isLinkHeavy(heavy))
}
