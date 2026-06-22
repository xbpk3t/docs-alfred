package wiki

import (
	"context"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/cloudflare/ahocorasick"
	"github.com/go-resty/resty/v2"
	"github.com/google/go-github/v70/github"
	"github.com/mmcdole/gofeed"
	"github.com/samber/lo"
	"github.com/xbpk3t/docs-alfred/internal/rss/transcript"
	"github.com/xbpk3t/docs-alfred/pkg/httputil"
	"github.com/xbpk3t/docs-alfred/pkg/textutil"
	"github.com/xbpk3t/docs-alfred/pkg/urlutil"
)

// ContentFetchResult holds fetched content metadata and body.
type ContentFetchResult struct {
	Title       string      `json:"title"`
	Body        string      `json:"body"`
	SourceURL   string      `json:"sourceUrl"`
	Error       string      `json:"error,omitempty"`
	FailureKind FailureKind `json:"-"`
}

// Fetcher handles fetching content from various sources.
type Fetcher struct {
	driver       ContentDriver
	GHClient     *resty.Client
	GHBaseURL    string
	MaxBodySize  int
	MediaEnabled bool
}

// FetcherOption customizes content fetching behavior.
type FetcherOption func(*Fetcher)

// WithDriver sets the content driver for the fetcher.
func WithDriver(driver ContentDriver) FetcherOption {
	return func(f *Fetcher) { f.driver = driver }
}

// WithMediaEnabled enables or media content extraction.
func WithMediaEnabled(enabled bool) FetcherOption {
	return func(f *Fetcher) { f.MediaEnabled = enabled }
}

// NewFetcher creates a new Fetcher with default settings.
func NewFetcher(opts ...FetcherOption) *Fetcher {
	f := &Fetcher{
		GHClient:     httputil.NewRestyClient(30*time.Second, 0),
		GHBaseURL:    "https://api.github.com",
		MaxBodySize:  5000,
		MediaEnabled: true,
	}
	for _, opt := range opts {
		opt(f)
	}
	if f.driver == nil {
		f.driver = newOpenCLIDriver(DriverOptions{
			MaxBodySize:  f.MaxBodySize,
			MediaEnabled: f.MediaEnabled,
		})
	}
	if f.MaxBodySize <= 0 {
		f.MaxBodySize = 5000
	}

	return f
}

// FetchContent fetches content based on the URL pattern.
// GitHub repos and podcasts are handled directly; all other URLs
// are delegated to the configured ContentDriver.
func (f *Fetcher) FetchContent(ctx context.Context, urlStr, contentType string) *ContentFetchResult {
	slog.Info("FetchContent", "url", urlStr, "type", contentType)

	u := strings.ToLower(urlStr)
	if contentType == "" {
		contentType = DetectContentType(u)
	}
	if _, ok := urlutil.GitHubOwnerRepo(urlStr); ok {
		return f.fetchGitHubRepo(ctx, urlStr)
	}

	// Podcast/audio bypasses the driver — direct to podcast transcript pipeline.
	if contentType == ContentAudio || isPodcastLikeURL(u) || isDirectAudioURL(u) {
		if !f.MediaEnabled {
			return extractFailure(urlStr, "media extraction disabled")
		}

		return f.fetchPodcastTranscript(ctx, urlStr)
	}

	// Delegate to driver.
	result := f.driver.FetchContent(ctx, urlStr, contentType)
	if result != nil && result.Error == "" {
		return result
	}

	if result != nil {
		return result
	}

	return extractFailure(urlStr, "driver returned empty result")
}

// --- GitHub repo fetching ---

func (f *Fetcher) fetchGitHubRepo(ctx context.Context, rawURL string) *ContentFetchResult {
	repoRef, ok := urlutil.GitHubOwnerRepo(rawURL)
	if !ok {
		return &ContentFetchResult{SourceURL: rawURL, Error: "not a valid GitHub repo URL"}
	}
	owner, repoName := repoRef.Owner, repoRef.Name

	ghClient, err := f.githubClient()
	if err != nil {
		return &ContentFetchResult{SourceURL: rawURL, Error: err.Error()}
	}
	repoData, _, err := ghClient.Repositories.Get(ctx, owner, repoName)
	if err != nil {
		return &ContentFetchResult{SourceURL: rawURL, Error: err.Error()}
	}

	licenseName := noneVal
	if repoData.GetLicense() != nil && repoData.GetLicense().GetSPDXID() != "" {
		licenseName = repoData.GetLicense().GetSPDXID()
	}

	body := fmt.Sprintf(`Repository: %s/%s
		Stars: %d
		Language: %s
		License: %s
		Topics: %s
		Description: %s
		URL: %s`,
		owner, repoName,
		repoData.GetStargazersCount(),
		repoData.GetLanguage(),
		licenseName,
		strings.Join(repoData.Topics, ", "),
		repoData.GetDescription(),
		repoData.GetHTMLURL(),
	)

	title := fmt.Sprintf("%s/%s", owner, repoName)
	if repoData.GetDescription() != "" {
		title = fmt.Sprintf("%s/%s — %s", owner, repoName, repoData.GetDescription())
	}

	return &ContentFetchResult{Title: title, Body: body, SourceURL: rawURL}
}

func (f *Fetcher) githubClient() (*github.Client, error) {
	client := github.NewClient(f.GHClient.GetClient())
	client.UserAgent = "rss2nl-wiki"

	baseURL := strings.TrimSpace(f.GHBaseURL)
	if baseURL != "" && baseURL != "https://api.github.com" && baseURL != "https://api.github.com/" {
		if !strings.HasSuffix(baseURL, "/") {
			baseURL += "/"
		}
		parsed, err := url.Parse(baseURL)
		if err != nil {
			return nil, fmt.Errorf("parse GitHub base URL: %w", err)
		}
		client.BaseURL = parsed
	}

	if token := getGHToken(); token != "" {
		client = client.WithAuthToken(token)
	}

	return client, nil
}

// --- Podcast/audio fetching ---

func (f *Fetcher) fetchPodcastTranscript(ctx context.Context, rawURL string) *ContentFetchResult {
	lowerURL := strings.ToLower(rawURL)
	if isDirectAudioURL(lowerURL) {
		return extractFailure(rawURL, "rss transcript unavailable: direct audio URL has no RSS metadata")
	}

	feed, err := gofeed.NewParser().ParseURLWithContext(rawURL, ctx)
	if err != nil {
		return extractFailure(rawURL, "rss transcript unavailable: feed parse: "+err.Error())
	}
	if feed == nil || len(feed.Items) == 0 {
		return extractFailure(rawURL, "rss transcript unavailable: feed has no items")
	}

	var failures []string
	for _, item := range podcastFeedCandidates(feed, rawURL) {
		ep := transcript.EpisodeRefFromFeedItem(item, feed.Title, rawURL)
		result := f.fetchTranscriptFromEpisode(ctx, &ep, rawURL)
		if result.Error == "" {
			return result
		}
		if reason := cleanExtractReason(result.Error); !isMissingTranscriptReason(reason) {
			failures = append(failures, reason)
		}
	}

	reason := strings.Join(lo.Filter(failures, func(s string, _ int) bool { return s != "" }), "; ")
	if reason == "" {
		reason = "no RSS item transcript found"
	}

	return extractFailure(rawURL, "rss transcript unavailable: "+reason)
}

func podcastFeedCandidates(feed *gofeed.Feed, rawURL string) []*gofeed.Item {
	if feed == nil || len(feed.Items) == 0 {
		return nil
	}

	collector := newFeedCandidateCollector(feed)
	collector.addExactURLMatches(rawURL)
	collector.addTranscriptLikeItems()
	collector.addRemainingItems()

	return collector.candidates
}

type feedCandidateCollector struct {
	feed       *gofeed.Feed
	seen       map[*gofeed.Item]bool
	candidates []*gofeed.Item
}

func newFeedCandidateCollector(feed *gofeed.Feed) *feedCandidateCollector {
	return &feedCandidateCollector{
		feed:       feed,
		seen:       make(map[*gofeed.Item]bool, len(feed.Items)),
		candidates: make([]*gofeed.Item, 0, len(feed.Items)),
	}
}

func (c *feedCandidateCollector) add(item *gofeed.Item) {
	if item == nil || c.seen[item] {
		return
	}
	c.seen[item] = true
	c.candidates = append(c.candidates, item)
}

func (c *feedCandidateCollector) addExactURLMatches(rawURL string) {
	trimmedURL := strings.TrimSpace(rawURL)
	for _, item := range c.feed.Items {
		if item != nil && (urlutil.Equal(item.Link, rawURL) || strings.EqualFold(strings.TrimSpace(item.GUID), trimmedURL)) {
			c.add(item)
		}
	}
}

func (c *feedCandidateCollector) addTranscriptLikeItems() {
	for _, item := range c.feed.Items {
		if item == nil {
			continue
		}
		ep := transcript.EpisodeRefFromFeedItem(item, c.feed.Title, c.feed.FeedLink)
		if hasTranscriptSignal(&ep) {
			c.add(item)
		}
	}
}

func (c *feedCandidateCollector) addRemainingItems() {
	for _, item := range c.feed.Items {
		c.add(item)
	}
}

func hasTranscriptSignal(ep *transcript.EpisodeRef) bool {
	return len(ep.TranscriptLinks) > 0 || strings.Contains(strings.ToLower(ep.Description+"\n"+ep.Content), "transcript")
}

func (f *Fetcher) fetchTranscriptFromEpisode(
	ctx context.Context,
	ep *transcript.EpisodeRef,
	rawURL string,
) *ContentFetchResult {
	providers := []transcript.Provider{
		transcript.NewRssTranscriptProvider(),
		transcript.NewDescriptionLinkProvider(),
	}

	result, source, err := transcript.NewPipeline(providers...).Fetch(ctx, ep)
	if err != nil {
		return extractFailure(rawURL, "transcript unavailable: "+err.Error())
	}
	content := strings.TrimSpace(result.Content)
	if len([]rune(content)) < 40 {
		return extractFailure(rawURL, "transcript too short")
	}

	title := strings.TrimSpace(ep.Title)
	if title == "" {
		title = strings.TrimSpace(result.EpisodeTitle)
	}
	if title == "" {
		title = rawURL
	}

	return mediaContentResult(title, rawURL, source, content, f.mediaMaxBodySize())
}

func (f *Fetcher) mediaMaxBodySize() int {
	bodyLimit := f.MaxBodySize * 4
	if bodyLimit <= 0 {
		return 20_000
	}

	return bodyLimit
}

func mediaContentResult(title, rawURL, source, content string, maxBodySize int) *ContentFetchResult {
	content = strings.TrimSpace(content)
	content = textutil.TruncateUTF8(content, maxBodySize)
	body := fmt.Sprintf("Title: %s\nURL: %s\nTranscript source: %s\n\n%s", title, rawURL, source, content)

	return &ContentFetchResult{Title: title, Body: body, SourceURL: rawURL}
}

// --- Content quality assessment ---

type contentQuality struct {
	Reason string
	OK     bool
}

func assessContentQuality(title, body, rawURL string) contentQuality {
	trimmed := strings.TrimSpace(body)
	if trimmed == "" {
		return contentQuality{Reason: "empty body"}
	}
	lower := strings.ToLower(title + "\n" + trimmed)
	if cfReason := cloudflareChallengeReason(lower); cfReason != "" {
		return contentQuality{Reason: cfReason}
	}
	if matches := lowQualityMatcher.Match([]byte(lower)); len(matches) > 0 {
		return contentQuality{Reason: "matched error/login shell"}
	}

	domain := urlutil.Domain(rawURL)
	if isSocialShellDomain(domain) && socialShellLike(lower, trimmed) {
		return contentQuality{Reason: "social/login shell"}
	}
	if len([]rune(trimmed)) < 120 {
		return contentQuality{Reason: "too short"}
	}
	if isLinkHeavy(trimmed) {
		return contentQuality{Reason: "link-heavy navigation shell"}
	}

	return contentQuality{OK: true}
}

func cloudflareChallengeReason(lower string) string {
	if strings.Contains(lower, "just a moment") &&
		strings.Contains(lower, "checking your browser") {
		return "cloudflare anti-bot: challenge page"
	}
	if strings.Contains(lower, "just a moment") &&
		(strings.Contains(lower, "cf-browser-verification") ||
			strings.Contains(lower, "cloudflare") ||
			strings.Contains(lower, "ray id:")) {
		return "cloudflare anti-bot: challenge page"
	}

	return ""
}

var lowQualityPatternsList = []string{
	"this page requires javascript",
	"javascript is not available",
	"enable javascript",
	"please enable js",
	"please log in",
	"log in to continue",
	"sign in to continue",
	"sign up for",
	"access denied",
	"forbidden",
	"captcha",
	"checking your browser",
	"just a moment",
	"400 bad request",
	"404 not found",
	"page not found",
	"something went wrong",
	"video content requires manual review",
}

var lowQualityMatcher = ahocorasick.NewStringMatcher(lowQualityPatternsList)

func isSocialShellDomain(domain string) bool {
	switch strings.ToLower(domain) {
	case "x.com", "twitter.com", "mobile.twitter.com", "t.co", "instagram.com", "www.instagram.com":
		return true
	default:
		return false
	}
}

const (
	socialShortContentThreshold = 300
	socialContentThreshold      = 2000
	socialMinSentences          = 3
)

func socialShellLike(lower, body string) bool {
	trimmed := strings.TrimSpace(body)
	runeLen := len([]rune(trimmed))
	if runeLen < socialShortContentThreshold {
		return true
	}
	if socialShellPatterns(lower) {
		return true
	}
	if runeLen < socialContentThreshold && !hasRealSentences(lower) {
		return true
	}

	return false
}

var socialShellMatcher = ahocorasick.NewStringMatcher([]string{
	"log in",
	"sign up",
	"javascript",
	"enable js",
	"keyboard shortcuts",
	"keyboard shortcut",
	"to continue, please",
	"already have an account",
	"don't have an account",
})

func socialShellPatterns(lower string) bool {
	return len(socialShellMatcher.Match([]byte(lower))) > 0
}

func hasRealSentences(lower string) bool {
	count := 0
	for i := 0; i < len(lower); i++ {
		switch lower[i] {
		case '.', '!', '?':
			count++
			if count >= socialMinSentences {
				return true
			}
		}
	}

	return false
}

func isLinkHeavy(body string) bool {
	links := strings.Count(body, "](") + urlutil.CountURLs(body)
	if links < 20 {
		return false
	}
	textLen := len([]rune(body))
	if textLen == 0 {
		return true
	}

	return links*80 > textLen
}

// --- Failure helpers ---

func extractFailure(rawURL, reason string) *ContentFetchResult {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		reason = "content extraction failed"
	}
	if strings.HasPrefix(reason, "extract:") {
		return &ContentFetchResult{SourceURL: rawURL, Error: reason, FailureKind: FailureExtract}
	}

	return &ContentFetchResult{SourceURL: rawURL, Error: "extract: " + reason, FailureKind: FailureExtract}
}

func cleanExtractReason(reason string) string {
	return strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(reason), "extract:"))
}

func isMissingTranscriptReason(reason string) bool {
	lower := strings.ToLower(reason)

	return strings.Contains(lower, "rss item has no podcast:transcript tag") ||
		strings.Contains(lower, "description/content has no transcript link") ||
		strings.Contains(lower, "no description or content to search") ||
		strings.Contains(lower, "all providers failed to produce transcript")
}

// --- URL classification helpers ---

func isVideoURL(lowerURL string) bool {
	lowerURL = strings.ToLower(strings.TrimSpace(lowerURL))
	parsed, err := url.Parse(lowerURL)
	if err != nil {
		return hasKnownVideoURLText(lowerURL)
	}

	return isParsedVideoURL(parsed)
}

func hasKnownVideoURLText(lowerURL string) bool {
	return strings.Contains(lowerURL, "youtu.be/") ||
		strings.Contains(lowerURL, "b23.tv/") ||
		strings.Contains(lowerURL, "bilibili.com/video/")
}

func isParsedVideoURL(parsed *url.URL) bool {
	host := strings.TrimPrefix(parsed.Hostname(), "www.")
	path := strings.TrimRight(parsed.EscapedPath(), "/")
	switch host {
	case "youtu.be":
		return strings.Trim(path, "/") != ""
	case "youtube.com", "m.youtube.com", "music.youtube.com":
		return (path == "/watch" && strings.TrimSpace(parsed.Query().Get("v")) != "") ||
			strings.HasPrefix(path, "/shorts/") ||
			strings.HasPrefix(path, "/embed/")
	case "bilibili.com", "m.bilibili.com":
		return strings.HasPrefix(path, "/video/")
	case "b23.tv":
		return strings.Trim(path, "/") != ""
	default:
		return false
	}
}

func isPodcastLikeURL(lowerURL string) bool {
	return strings.Contains(lowerURL, "xiaoyuzhou") || strings.Contains(lowerURL, "podcast") ||
		strings.Contains(lowerURL, "libsyn.com") || strings.Contains(lowerURL, "anchor.fm") || isRSSFeedLike(lowerURL)
}

func isRSSFeedLike(lowerURL string) bool {
	path := lowerURL
	if parsed, err := url.Parse(lowerURL); err == nil {
		path = strings.ToLower(parsed.Path)
	}
	path = strings.TrimRight(path, "/")

	return strings.HasSuffix(path, ".xml") || strings.HasSuffix(path, ".rss") || strings.HasSuffix(path, ".atom") ||
		strings.HasSuffix(path, "/feed") || strings.Contains(path, "/feed/") || strings.Contains(path, "/rss")
}

func isDirectAudioURL(lowerURL string) bool {
	urlPath := lowerURL
	if parsed, err := url.Parse(lowerURL); err == nil {
		urlPath = strings.ToLower(parsed.Path)
	}
	for _, suffix := range []string{".mp3", ".m4a", ".aac", ".wav", ".flac", ".ogg", ".opus"} {
		if strings.HasSuffix(urlPath, suffix) {
			return true
		}
	}

	return false
}

func getGHToken() string {
	if token := os.Getenv("GITHUB_TOKEN"); token != "" {
		return token
	}

	return os.Getenv("GH_TOKEN")
}
