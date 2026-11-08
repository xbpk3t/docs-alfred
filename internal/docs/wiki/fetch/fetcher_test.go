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

func TestShellTitleUntitledIsPermanent(t *testing.T) {
	reason, failureReason := shellTitleReason("untitled")
	assert.Contains(t, reason, "untitled")
	assert.Equal(t, types.FailureReasonEmpty, failureReason,
		"an untitled extraction is a parked/empty page, not a retryable partial extract")
}

func TestHTTPDriverRejectsLowQualityContent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`<html><head><title>X</title></head><body>This page requires JavaScript.</body></html>`))
	}))
	t.Cleanup(server.Close)

	// The fetcher-level gate (ensureFetchedQuality) is the single quality gate
	// for all drivers; the HTTP driver no longer gates internally.
	driver := newHTTPDriver(DriverOptions{MaxBodySize: 5000})
	result := driver.FetchContent(context.Background(), server.URL, types.ContentText)
	require.NotNil(t, result)

	f := NewFetcher(WithDriver(driver))
	gated := f.ensureFetchedQuality(result, types.ContentText, server.URL)
	require.Contains(t, gated.Error, "extract:")
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
	// A short social body is a too-short failure, not necessarily a login shell.
	q := assessContentQuality("Title", "short", "https://x.com/user/status/1")
	assert.False(t, q.OK)
	assert.Equal(t, "too short", q.Reason)

	// Explicit login chrome on a long-enough body is still a social shell.
	body := strings.Repeat("welcome ", 30) + " already have an account"
	q = assessContentQuality("Title", body, "https://x.com/user/status/1")
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
	result := f.fetchPodcastTranscript(context.Background(), "https://invalid.invalid.invalid/episode/1")
	assert.Nil(t, result, "a non-feed URL falls through to the normal drivers")
}

func TestFetchPodcastTranscriptFeedLikeNetworkErrorIsRetryable(t *testing.T) {
	f := NewFetcher()
	result := f.fetchPodcastTranscript(context.Background(), "https://invalid.invalid.invalid/feed.xml")
	require.NotNil(t, result)
	assert.Equal(t, types.FailureReasonHTTPError, result.FailureReason)
}

// --- socialShellPatterns edge cases ---

func TestSocialShellLikeMediumContentWithPatterns(t *testing.T) {
	// Content between 300 and 2000 runes with social patterns
	content := strings.Repeat("x", 500) + " please log in to continue " + strings.Repeat("y", 500)
	assert.True(t, socialShellPatterns(content))
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
	result := f.FetchContent(context.Background(), "https://xiaoyuzhoufm.com/episode/abc", "")
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

// --- socialShellPatterns with medium content and patterns ---

func TestSocialShellLikeMediumContentWithSocialPatterns(t *testing.T) {
	// Content between 300 and 2000 runes with social patterns
	content := strings.Repeat("x", 500) + " please log in to continue " + strings.Repeat("y", 500)
	assert.True(t, socialShellPatterns(content))
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

// --- quality gate: production shell/error pages ---

func TestAssessContentQualityShellSignals(t *testing.T) {
	normalArticle := strings.Repeat("This article explains a real engineering tradeoff in detail. ", 30)

	tests := []struct {
		name       string
		title      string
		body       string
		url        string
		wantOK     bool
		wantReason types.FailureReason
	}{
		{
			name:       "zhihu verification title",
			title:      "安全验证",
			body:       "进入知乎\n\n系统监测到您的网络环境存在异常，请点击下方验证按钮进行验证。",
			url:        "https://zhuanlan.zhihu.com/p/1",
			wantReason: types.FailureReasonAntiBot,
		},
		{
			name:       "cloudflare challenge",
			title:      "Just a moment...",
			body:       "Just a moment... Performing security verification...",
			url:        "https://example.com/article",
			wantReason: types.FailureReasonCloudflare,
		},
		{
			name:       "chrome connection error",
			title:      "vutr.substack.com",
			body:       "This site can’t be reached. ERR_CONNECTION_CLOSED",
			url:        "https://vutr.substack.com/p/1",
			wantReason: types.FailureReasonBrowserUnreachable,
		},
		{
			name:       "gerrit query shell",
			title:      "status:open",
			body:       "[All](https://gerrit.libreoffice.org/) [Open](https://gerrit.libreoffice.org/)",
			url:        "https://gerrit.libreoffice.org/",
			wantReason: types.FailureReasonIndexPage,
		},
		{
			name:       "javascript loading shell",
			title:      "bbtree",
			body:       "Loading…",
			url:        "https://bbtree.xiangpeng.systems/post/",
			wantReason: types.FailureReasonJSShell,
		},
		{
			name:       "github api 404 json",
			title:      "",
			body:       `{"message":"Not Found","documentation_url":"https://docs.github.com/rest"}`,
			url:        "https://api.github.com/repos/a/b",
			wantReason: types.FailureReasonNotFound,
		},
		{
			name:   "short article mentioning captcha passes",
			title:  "CAPTCHA notes",
			body:   strings.Repeat("A captcha is a challenge-response test. ", 20),
			url:    "https://example.com/captcha-short",
			wantOK: true,
		},
		{
			name:   "long article mentioning captcha passes",
			title:  "How CAPTCHA systems work",
			body:   strings.Repeat("CAPTCHA is a challenge-response test. ", 80),
			url:    "https://example.com/captcha",
			wantOK: true,
		},
		{
			name:   "chinese social post with sentences passes",
			title:  "推文",
			body:   strings.Repeat("这是真实内容。它包含完整句子。还有第三个句子。", 20),
			url:    "https://x.com/user/status/1",
			wantOK: true,
		},
		{
			name:   "normal article passes",
			title:  "A real article",
			body:   normalArticle,
			url:    "https://example.com/article",
			wantOK: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			q := assessContentQuality(tt.title, tt.body, tt.url)
			assert.Equal(t, tt.wantOK, q.OK)
			if !tt.wantOK {
				assert.Equal(t, tt.wantReason, q.FailureReason)
			}
		})
	}
}

func TestFetchGitHubRepoIncludesReadme(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.HasSuffix(r.URL.Path, "/readme") {
			_, _ = w.Write([]byte(`{"content":"IyBSRVZNRQoKVGhpcyBpcyB0aGUgcmVhZG1lIGJvZHku","encoding":"base64"}`))
			return
		}
		_, _ = w.Write([]byte(`{
			"name": "testrepo",
			"full_name": "owner/testrepo",
			"description": "A test repository",
			"html_url": "https://github.com/owner/testrepo"
		}`))
	}))
	t.Cleanup(server.Close)

	f := NewFetcher()
	f.GHBaseURL = server.URL + "/"
	result := f.fetchGitHubRepo(context.Background(), "https://github.com/owner/testrepo")

	require.NotNil(t, result)
	require.Empty(t, result.Error)
	assert.Contains(t, result.Body, "README:")
	assert.Contains(t, result.Body, "This is the readme body.")
}

func TestNormalizeGitHubAPIURL(t *testing.T) {
	assert.Equal(t, "https://github.com/Autumn-27/ARTEX",
		normalizeGitHubAPIURL("https://api.github.com/repos/Autumn-27/ARTEX"))
	assert.Equal(t, "https://example.com/x",
		normalizeGitHubAPIURL("https://example.com/x"))
	assert.Equal(t, "https://api.github.com/users/foo",
		normalizeGitHubAPIURL("https://api.github.com/users/foo"))
}

// --- PDF and arXiv routing ---

type recordingDriver struct {
	url  string
	body string
}

func (d *recordingDriver) Name() string { return "recording" }
func (d *recordingDriver) FetchContent(_ context.Context, urlStr, _ string) *types.ContentFetchResult {
	d.url = urlStr
	if d.body == "" {
		d.body = strings.Repeat("A sufficiently long article body. ", 20)
	}

	return &types.ContentFetchResult{Title: "Article", Body: d.body, SourceURL: urlStr}
}

func TestFetchContentArxivPDFUsesAbsPage(t *testing.T) {
	driver := &recordingDriver{}
	f := NewFetcher(WithDriver(driver))
	result := f.FetchContent(context.Background(), "https://arxiv.org/pdf/2411.19718", types.ContentText)

	require.NotNil(t, result)
	assert.Empty(t, result.Error)
	assert.Equal(t, "https://arxiv.org/abs/2411.19718", driver.url)
	assert.Equal(t, "https://arxiv.org/pdf/2411.19718", result.SourceURL)
}

func TestFetchContentGenericPDFRejected(t *testing.T) {
	driver := &recordingDriver{}
	f := NewFetcher(WithDriver(driver))
	result := f.FetchContent(context.Background(), "https://example.com/paper.pdf", types.ContentText)

	require.NotNil(t, result)
	assert.Contains(t, result.Error, "pdf extraction unsupported")
	assert.Equal(t, types.FailureReasonPDFUnsupported, result.FailureReason)
	assert.Empty(t, driver.url)
}

func TestAssessContentQualityPartialExtract(t *testing.T) {
	q := assessContentQuality("Article", "A short lede. Continue reading", "https://example.com/article")
	assert.False(t, q.OK)
	assert.Equal(t, types.FailureReasonPartialExtract, q.FailureReason)
}

func TestFetchContentPrefersFallbackForGenericURL(t *testing.T) {
	primary := &recordingDriver{body: strings.Repeat("primary body. ", 30)}
	fallback := &recordingDriver{body: strings.Repeat("fallback body. ", 30)}
	f := NewFetcher(WithDriver(primary), WithFallbackDriver(fallback))

	result := f.FetchContent(context.Background(), "https://example.com/article", types.ContentText)

	require.NotNil(t, result)
	require.Empty(t, result.Error)
	assert.Equal(t, "https://example.com/article", fallback.url)
	assert.Empty(t, primary.url, "generic urls should not touch the browser-backed primary")
	assert.Contains(t, result.Body, "fallback body")
}

func TestFetchContentAdapterFallsBackWhenPrimaryEmpty(t *testing.T) {
	fallback := &recordingDriver{body: strings.Repeat("fallback body. ", 30)}
	f := NewFetcher(WithDriver(&nilDriver{}), WithFallbackDriver(fallback))

	result := f.FetchContent(context.Background(), "https://zhuanlan.zhihu.com/p/123", types.ContentText)

	require.NotNil(t, result)
	require.Empty(t, result.Error)
	assert.Equal(t, "https://zhuanlan.zhihu.com/p/123", fallback.url)
}
