package wiki

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"time"

	"codeberg.org/readeck/go-readability/v2"
	"github.com/PuerkitoBio/goquery"
	"github.com/google/go-github/v70/github"
	"github.com/mmcdole/gofeed"
	"github.com/xbpk3t/docs-alfred/internal/transcript"
	"github.com/xbpk3t/docs-alfred/pkg/htmlutil"
	"github.com/xbpk3t/docs-alfred/pkg/httputil"
	"github.com/xbpk3t/docs-alfred/pkg/opencli"
	"github.com/xbpk3t/docs-alfred/pkg/textutil"
	"github.com/xbpk3t/docs-alfred/pkg/urlutil"
	"github.com/yuin/goldmark"
	gast "github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	geast "github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/text"
)

// FetchMethod represents a single content fetching approach.
type FetchMethod int

const (
	MethodAdapter FetchMethod = iota // opencli site adapter (youtube/bilibili/twitter...)
	MethodHTTP                       // HTTP GET + readability/markdown
	MethodWeb                        // opencli web read (generic browser)
)

// FetchStrategy selects the primary fetching approach.
type FetchStrategy int

const (
	FetchStrategyOpenCLI FetchStrategy = iota // opencli first (adapter or web read)
	FetchStrategyHTTP                         // HTTP GET first (current behavior)
)

// FetchPlanner decides the ordered fetch methods for a URL.
type FetchPlanner interface {
	// Plan returns an ordered list of fetch methods to try for the given URL.
	Plan(ctx context.Context, urlStr string) []FetchMethod
}

// OpenCLIPlanner implements opencli-first strategy.
type OpenCLIPlanner struct{}

func (OpenCLIPlanner) Plan(_ context.Context, urlStr string) []FetchMethod {
	if opencli.HasAdapter(urlStr) {
		return []FetchMethod{MethodAdapter}
	}

	return []FetchMethod{MethodWeb}
}

// HTTPPlanner implements HTTP-first strategy (current behavior).
type HTTPPlanner struct{}

func (HTTPPlanner) Plan(_ context.Context, urlStr string) []FetchMethod {
	if opencli.HasAdapter(urlStr) {
		return []FetchMethod{MethodHTTP, MethodAdapter}
	}

	return []FetchMethod{MethodHTTP, MethodWeb}
}

func strategyPlanner(s FetchStrategy) FetchPlanner {
	switch s {
	case FetchStrategyHTTP:
		return HTTPPlanner{}
	default:
		return OpenCLIPlanner{}
	}
}

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
	planner      FetchPlanner
	GHClient     *http.Client
	HTTPClient   *http.Client
	GHBaseURL    string
	MaxBodySize  int
	Strategy     FetchStrategy
	MediaEnabled bool
}

// FetcherOption customizes content fetching behavior.
type FetcherOption func(*Fetcher)

func WithStrategy(strategy FetchStrategy) FetcherOption {
	return func(f *Fetcher) { f.Strategy = strategy; f.planner = strategyPlanner(strategy) }
}

func WithMediaEnabled(enabled bool) FetcherOption {
	return func(f *Fetcher) { f.MediaEnabled = enabled }
}

// NewFetcher creates a new Fetcher with default settings (opencli-first).
func NewFetcher(opts ...FetcherOption) *Fetcher {
	f := &Fetcher{
		GHClient:     httputil.NewClient(30 * time.Second),
		HTTPClient:   httputil.NewClient(30 * time.Second),
		GHBaseURL:    "https://api.github.com",
		MaxBodySize:  5000,
		Strategy:     FetchStrategyOpenCLI,
		MediaEnabled: true,
	}
	for _, opt := range opts {
		opt(f)
	}
	if f.planner == nil {
		f.planner = strategyPlanner(f.Strategy)
	}
	if f.MaxBodySize <= 0 {
		f.MaxBodySize = 5000
	}

	return f
}

// FetchContent fetches content based on the URL pattern.
// Uses the configured FetchPlanner to determine the ordered fetch methods.
func (f *Fetcher) FetchContent(ctx context.Context, urlStr, contentType string) *ContentFetchResult {
	slog.Info("FetchContent", "url", urlStr, "type", contentType)

	u := strings.ToLower(urlStr)
	if contentType == "" {
		contentType = DetectContentType(u)
	}
	if _, ok := urlutil.GitHubOwnerRepo(urlStr); ok {
		return f.fetchGitHubRepo(ctx, urlStr)
	}

	// Podcast/audio bypasses the planner — direct to podcast transcript pipeline.
	if contentType == ContentAudio || isPodcastLikeURL(u) || isDirectAudioURL(u) {
		if !f.MediaEnabled {
			return extractFailure(urlStr, "media extraction disabled")
		}

		return f.fetchPodcastTranscript(ctx, urlStr)
	}

	// Planner-driven fetch: try each method in order.
	methods := f.planner.Plan(ctx, u)

	for _, m := range methods {
		if result := f.tryFetchMethod(ctx, m, urlStr, u); result != nil {
			return result
		}
	}

	return extractFailure(urlStr, "all fetch methods failed")
}

// tryFetchMethod attempts a single fetch method from the planner.
// Returns the result if successful, or nil to continue to the next method.
func (f *Fetcher) tryFetchMethod(ctx context.Context, m FetchMethod, urlStr, u string) *ContentFetchResult {
	switch m {
	case MethodAdapter:
		if isVideoURL(u) && !f.MediaEnabled {
			return extractFailure(urlStr, "media extraction disabled")
		}

		return validOrNil(f.fetchWithOpenCLI(ctx, urlStr))
	case MethodHTTP:
		return validOrNil(f.fetchHTTPPage(ctx, urlStr))
	case MethodWeb:
		r := f.runOpenCLI(ctx, "web", []string{"read", "--url", urlStr, "--stdout"})
		r.SourceURL = urlStr

		return validOrNil(r)
	}

	return nil
}

// validOrNil returns the result if its content quality is acceptable, nil otherwise.
func validOrNil(r *ContentFetchResult) *ContentFetchResult {
	if r == nil || r.Error != "" {
		return nil
	}
	q := assessContentQuality(r.Title, r.Body, r.SourceURL)
	if q.OK {
		return r
	}

	return nil
}

// fetchGitHubRepo fetches GitHub repository information via the API.
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
	client := github.NewClient(f.GHClient)
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

	reason := strings.Join(compactStrings(failures), "; ")
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

// fetchHTTPPage fetches a web page via plain HTTP GET.
// Quality checking is performed via ensureContentQuality.
// The FetchPlanner manages fallback — this function does not retry or delegate.
func (f *Fetcher) fetchHTTPPage(ctx context.Context, rawURL string) *ContentFetchResult {
	timeout := httputil.DefaultClientTimeout
	if f.HTTPClient != nil && f.HTTPClient.Timeout > 0 {
		timeout = f.HTTPClient.Timeout
	}
	data, err := httputil.GetBytes(ctx, rawURL, httputil.RequestOptions{Timeout: timeout})
	if err != nil {
		failureKind := FailureFetch
		errorStr := err.Error()
		if isHTTPBlockError(err) {
			failureKind = FailureResolve
			errorStr = "resolve: " + errorStr
		}

		return &ContentFetchResult{SourceURL: rawURL, Error: errorStr, FailureKind: failureKind}
	}

	return f.handleHTTPPageData(ctx, rawURL, data)
}

func (f *Fetcher) handleHTTPPageData(ctx context.Context, rawURL string, data []byte) *ContentFetchResult {
	slog.Info("HTTP fetch succeeded", "url", rawURL, "bodyLen", len(data))
	if result := f.extractWithReadability(data, rawURL); result != nil {
		return f.ensureContentQuality(ctx, result)
	}

	body := textutil.TruncateUTF8(markdownFallbackBody(data), f.MaxBodySize)
	title := extractTitle(string(data))
	if title == "" {
		title = rawURL
	}

	return f.ensureContentQuality(ctx, &ContentFetchResult{Title: title, Body: body, SourceURL: rawURL})
}

func (f *Fetcher) ensureContentQuality(_ context.Context, result *ContentFetchResult) *ContentFetchResult {
	if result == nil || result.Error != "" {
		return result
	}
	quality := assessContentQuality(result.Title, result.Body, result.SourceURL)
	if quality.OK {
		return result
	}

	return extractFailure(result.SourceURL, "low quality HTTP content: "+quality.Reason)
}

func markdownFallbackBody(data []byte) string {
	body, err := htmlutil.ToMarkdown(string(data))
	if err == nil && strings.TrimSpace(body) != "" {
		return body
	}

	return string(data)
}

// extractWithReadability uses go-readability to extract article content.
// Returns nil if extraction fails or produces empty content.
func (f *Fetcher) extractWithReadability(data []byte, rawURL string) *ContentFetchResult {
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return nil
	}

	article, err := readability.FromReader(bytes.NewReader(data), parsedURL)
	if err != nil || article.Node == nil {
		return nil
	}

	var buf strings.Builder
	if err := article.RenderText(&buf); err != nil {
		return nil
	}

	body := buf.String()
	if strings.TrimSpace(body) == "" {
		return nil
	}

	body = textutil.TruncateUTF8(body, f.MaxBodySize)

	title := article.Title()
	if title == "" {
		title = extractTitle(string(data))
	}
	if title == "" {
		title = rawURL
	}

	slog.Info("go-readability extraction succeeded", "url", rawURL, "bodyLen", len(body))

	return &ContentFetchResult{Title: title, Body: body, SourceURL: rawURL}
}

// isHTTPBlockError checks whether the error is from an HTTP status code
// (e.g. 403 anti-bot) rather than a network-level error.
func isHTTPBlockError(err error) bool {
	return strings.Contains(err.Error(), "HTTP ")
}

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
	for _, pattern := range lowQualityPatterns() {
		if strings.Contains(lower, pattern) {
			return contentQuality{Reason: "matched error/login shell: " + pattern}
		}
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

// cloudflareChallengeReason returns a descriptive reason if the content
// looks like a Cloudflare anti-bot challenge page, or empty string otherwise.
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

func lowQualityPatterns() []string {
	return []string{
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
}

func isSocialShellDomain(domain string) bool {
	switch strings.ToLower(domain) {
	case "x.com", "twitter.com", "mobile.twitter.com", "t.co", "instagram.com", "www.instagram.com":
		return true
	default:
		return false
	}
}

// Content quality check thresholds for social shell detection.
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

func socialShellPatterns(lower string) bool {
	return strings.Contains(lower, "log in") ||
		strings.Contains(lower, "sign up") ||
		strings.Contains(lower, "javascript") ||
		strings.Contains(lower, "enable js") ||
		strings.Contains(lower, "keyboard shortcuts") ||
		strings.Contains(lower, "keyboard shortcut") ||
		strings.Contains(lower, "to continue, please") ||
		strings.Contains(lower, "already have an account") ||
		strings.Contains(lower, "don't have an account")
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
func compactStrings(items []string) []string {
	result := make([]string, 0, len(items))
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item != "" {
			result = append(result, item)
		}
	}

	return result
}

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
	parsed, err := url.Parse(lowerURL)
	path := lowerURL
	if err == nil {
		path = strings.ToLower(parsed.Path)
	}
	path = strings.TrimRight(path, "/")

	return strings.HasSuffix(path, ".xml") || strings.HasSuffix(path, ".rss") || strings.HasSuffix(path, ".atom") ||
		strings.HasSuffix(path, "/feed") || strings.Contains(path, "/feed/") || strings.Contains(path, "/rss")
}

func isDirectAudioURL(lowerURL string) bool {
	parsed, err := url.Parse(lowerURL)
	urlPath := lowerURL
	if err == nil {
		urlPath = strings.ToLower(parsed.Path)
	}
	for _, suffix := range []string{".mp3", ".m4a", ".aac", ".wav", ".flac", ".ogg", ".opus"} {
		if strings.HasSuffix(urlPath, suffix) {
			return true
		}
	}

	return false
}

// runOpenCLI executes an arbitrary opencli subcommand and returns the result.
func (f *Fetcher) runOpenCLI(ctx context.Context, subcommand string, extraArgs []string) *ContentFetchResult {
	args := append([]string{subcommand}, extraArgs...)

	cmd := exec.CommandContext(ctx, "opencli", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return &ContentFetchResult{
			Error: fmt.Sprintf("opencli: %v (stderr: %s)", err, strings.TrimSpace(stderr.String())),
		}
	}

	body := stdout.String()
	if body == "" {
		return &ContentFetchResult{Error: "opencli returned empty content"}
	}

	body = textutil.TruncateUTF8(body, f.MaxBodySize*3)

	title := extractTitleFromMarkdown(body)

	slog.Info("opencli fetch succeeded", "subcommand", subcommand, "bodyLen", len(body))

	return &ContentFetchResult{Title: title, Body: body}
}

// fetchWithOpenCLI uses the opencli browser tool to extract page content
// for a URL, routing to the appropriate site adapter.
func (f *Fetcher) fetchWithOpenCLI(ctx context.Context, rawURL string) *ContentFetchResult {
	subcommand, extraArgs := opencli.CommandForURL(rawURL)

	result := f.runOpenCLI(ctx, subcommand, extraArgs)
	result.SourceURL = rawURL

	return result
}

// extractTitle extracts the <title> from HTML content using goquery.
func extractTitle(htmlContent string) string {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(htmlContent))
	if err != nil {
		return ""
	}

	return strings.TrimSpace(doc.Find("title").First().Text())
}

// extractTitleFromMarkdown parses the markdown body with goldmark and returns
// the title from either:
//   - the first heading (any level), or
//   - a metadata-style table (| field | value |) with a "title" field.
//
// This handles all opencli adapters — those that return heading-prefixed
// markdown (twitter, web read, etc.) and those that return metadata tables
// (bilibili video, etc.).
func extractTitleFromMarkdown(body string) string {
	md := goldmark.New(
		goldmark.WithExtensions(extension.Table),
	)
	reader := text.NewReader([]byte(body))
	doc := md.Parser().Parse(reader)

	var title string
	_ = gast.Walk(doc, func(n gast.Node, entering bool) (gast.WalkStatus, error) {
		if !entering {
			return gast.WalkContinue, nil
		}

		// First heading (any level) wins.
		if n.Kind() == gast.KindHeading {
			title = strings.TrimSpace(string(n.Text([]byte(body))))

			return gast.WalkStop, nil
		}

		// Table with | field | value | structure — look for a "title" row.
		if n.Kind() == geast.KindTable {
			if t := extractTitleFromTable(body, n); t != "" {
				title = t

				return gast.WalkStop, nil
			}
		}

		return gast.WalkContinue, nil
	})

	return title
}

// extractTitleFromTable walks a goldmark Table AST node looking for a row
// whose first cell is "title" and returns the value from the second cell.
func extractTitleFromTable(body string, table gast.Node) string {
	for row := table.FirstChild(); row != nil; row = row.NextSibling() {
		if row.Kind() != geast.KindTableRow {
			continue
		}
		cell := row.FirstChild()
		if cell == nil || cell.Kind() != geast.KindTableCell {
			continue
		}
		field := strings.TrimSpace(string(cell.Text([]byte(body))))
		if !strings.EqualFold(field, "title") {
			continue
		}
		valueCell := cell.NextSibling()
		if valueCell == nil || valueCell.Kind() != geast.KindTableCell {
			continue
		}

		return strings.TrimSpace(string(valueCell.Text([]byte(body))))
	}

	return ""
}

func getGHToken() string {
	if token := os.Getenv("GITHUB_TOKEN"); token != "" {
		return token
	}

	return os.Getenv("GH_TOKEN")
}
