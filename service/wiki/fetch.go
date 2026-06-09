package wiki

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"sort"
	"strings"
	"time"
	"unicode"

	"codeberg.org/readeck/go-readability/v2"
	"github.com/PuerkitoBio/goquery"
	"github.com/google/go-github/v70/github"
	"github.com/mmcdole/gofeed"
	"github.com/xbpk3t/docs-alfred/pkg/htmlutil"
	"github.com/xbpk3t/docs-alfred/pkg/httputil"
	"github.com/xbpk3t/docs-alfred/pkg/textutil"
	"github.com/xbpk3t/docs-alfred/pkg/urlutil"
	"github.com/xbpk3t/docs-alfred/rss2nl/transcript"
	"mvdan.cc/xurls/v2"
)

// ContentFetchResult holds fetched content metadata and body.
type ContentFetchResult struct {
	Title     string `json:"title"`
	Body      string `json:"body"`
	SourceURL string `json:"sourceUrl"`
	Error     string `json:"error,omitempty"`
}

// Fetcher handles fetching content from various sources.
type Fetcher struct {
	GHClient        *http.Client
	HTTPClient      *http.Client
	GHBaseURL       string
	SubtitleCLIPath string
	SubtitleLangs   []string
	MaxBodySize     int
	OpenCLIFallback bool
	MediaEnabled    bool
}

// FetcherOption customizes content fetching behavior.
type FetcherOption func(*Fetcher)

func WithOpenCLIFallback(enabled bool) FetcherOption {
	return func(f *Fetcher) { f.OpenCLIFallback = enabled }
}

func WithMediaEnabled(enabled bool) FetcherOption {
	return func(f *Fetcher) { f.MediaEnabled = enabled }
}

func WithSubtitleCLIPath(path string) FetcherOption {
	return func(f *Fetcher) { f.SubtitleCLIPath = path }
}

func WithSubtitleLangs(langs []string) FetcherOption {
	return func(f *Fetcher) { f.SubtitleLangs = append([]string(nil), langs...) }
}

// NewFetcher creates a new Fetcher with default settings.
func NewFetcher(opts ...FetcherOption) *Fetcher {
	f := &Fetcher{
		GHClient:        httputil.NewClient(30 * time.Second),
		HTTPClient:      httputil.NewClient(30 * time.Second),
		GHBaseURL:       "https://api.github.com",
		MaxBodySize:     5000,
		OpenCLIFallback: true,
		MediaEnabled:    true,
		SubtitleCLIPath: "yt-dlp",
	}
	for _, opt := range opts {
		opt(f)
	}
	if f.MaxBodySize <= 0 {
		f.MaxBodySize = 5000
	}

	return f
}

// FetchContent fetches content based on the URL pattern.
// Supports GitHub repos, YouTube, Bilibili, and generic HTTP pages.
func (f *Fetcher) FetchContent(ctx context.Context, urlStr, contentType string) *ContentFetchResult {
	slog.Info("FetchContent", "url", urlStr, "type", contentType)

	u := strings.ToLower(urlStr)
	if contentType == "" {
		contentType = DetectContentType(u)
	}
	if _, ok := urlutil.GitHubOwnerRepo(urlStr); ok {
		return f.fetchGitHubRepo(ctx, urlStr)
	}

	switch {
	case isVideoURL(u):
		if !f.MediaEnabled {
			return extractFailure(urlStr, "media extraction disabled")
		}

		return f.fetchVideoTranscript(ctx, urlStr)
	case contentType == ContentAudio || isPodcastLikeURL(u) || isDirectAudioURL(u):
		if !f.MediaEnabled {
			return extractFailure(urlStr, "media extraction disabled")
		}

		return f.fetchPodcastTranscript(ctx, urlStr)
	default:
		return f.fetchHTTPPage(ctx, urlStr)
	}
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

type ytdlpMetadata struct {
	Subtitles         map[string][]ytdlpSubtitle `json:"subtitles"`
	AutomaticCaptions map[string][]ytdlpSubtitle `json:"automatic_captions"`
	Title             string                     `json:"title"`
	Description       string                     `json:"description"`
	Language          string                     `json:"language"`
	WebpageURL        string                     `json:"webpage_url"`
}

type ytdlpSubtitle struct {
	URL  string `json:"url"`
	Ext  string `json:"ext"`
	Name string `json:"name"`
}

type subtitlePick struct {
	Language string
	Source   string
	Item     ytdlpSubtitle
}

func (f *Fetcher) fetchVideoTranscript(ctx context.Context, rawURL string) *ContentFetchResult {
	meta, err := f.loadVideoMetadata(ctx, rawURL)
	if err != nil {
		return extractFailure(rawURL, err.Error())
	}

	pick, ok := pickVideoSubtitle(meta, f.videoSubtitleLangs(rawURL, meta))
	if !ok {
		return extractFailure(rawURL, "no subtitle or automatic caption available")
	}

	data, err := httputil.GetBytes(ctx, pick.Item.URL, httputil.RequestOptions{Timeout: httputil.DefaultClientTimeout})
	if err != nil {
		return extractFailure(rawURL, "fetch subtitle: "+err.Error())
	}
	contentType := transcript.DetectContentType(pick.Item.URL, subtitleDeclaredType(pick.Item.Ext), data)
	body := transcript.NormalizeContent(string(data), contentType)
	if strings.TrimSpace(body) == "" {
		return extractFailure(rawURL, "subtitle normalized to empty content")
	}

	title := strings.TrimSpace(meta.Title)
	if title == "" {
		title = rawURL
	}

	return mediaContentResult(title, rawURL, fmt.Sprintf("subtitle:%s:%s", pick.Source, pick.Language), body, f.mediaMaxBodySize())
}

func (f *Fetcher) loadVideoMetadata(ctx context.Context, rawURL string) (*ytdlpMetadata, error) {
	cliPath := strings.TrimSpace(f.SubtitleCLIPath)
	if cliPath == "" {
		cliPath = "yt-dlp"
	}
	if !commandAvailable(cliPath) {
		return nil, fmt.Errorf("subtitle CLI not found: %s", cliPath)
	}

	cmd := exec.CommandContext(ctx, cliPath, "--ignore-config", "--dump-json", "--skip-download", "--no-warnings", rawURL)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("yt-dlp metadata: %w (stderr: %s)", err, strings.TrimSpace(stderr.String()))
	}
	if strings.TrimSpace(stdout.String()) == "" {
		return nil, errors.New("yt-dlp returned empty metadata")
	}

	var meta ytdlpMetadata
	if err := json.Unmarshal(stdout.Bytes(), &meta); err != nil {
		return nil, fmt.Errorf("parse yt-dlp metadata: %w", err)
	}
	if meta.WebpageURL == "" {
		meta.WebpageURL = rawURL
	}

	return &meta, nil
}

func (f *Fetcher) videoSubtitleLangs(rawURL string, meta *ytdlpMetadata) []string {
	if len(f.SubtitleLangs) > 0 {
		return f.SubtitleLangs
	}
	if isBilibiliURL(strings.ToLower(rawURL)) || metadataLooksChinese(meta) {
		return []string{"zh-Hans", "zh-CN", "zh", "zh-Hant", "zh-TW", "cmn", "zho", "chi", "en"}
	}

	return []string{"en", "en-US", "en-GB", "zh-Hans", "zh-CN", "zh"}
}

func pickVideoSubtitle(meta *ytdlpMetadata, langs []string) (subtitlePick, bool) {
	if meta == nil {
		return subtitlePick{}, false
	}
	if lang, item, ok := pickSubtitleFromMap(meta.Subtitles, langs); ok {
		return subtitlePick{Language: lang, Source: "manual", Item: item}, true
	}
	if lang, item, ok := pickSubtitleFromMap(meta.AutomaticCaptions, langs); ok {
		return subtitlePick{Language: lang, Source: "auto", Item: item}, true
	}

	return subtitlePick{}, false
}

func pickSubtitleFromMap(subtitles map[string][]ytdlpSubtitle, langs []string) (string, ytdlpSubtitle, bool) {
	if len(subtitles) == 0 {
		return "", ytdlpSubtitle{}, false
	}
	for _, want := range langs {
		for _, have := range sortedSubtitleLangs(subtitles) {
			if !langMatches(have, want) {
				continue
			}
			for _, item := range subtitles[have] {
				if strings.TrimSpace(item.URL) != "" {
					return have, item, true
				}
			}
		}
	}
	for _, have := range sortedSubtitleLangs(subtitles) {
		for _, item := range subtitles[have] {
			if strings.TrimSpace(item.URL) != "" {
				return have, item, true
			}
		}
	}

	return "", ytdlpSubtitle{}, false
}

func sortedSubtitleLangs(subtitles map[string][]ytdlpSubtitle) []string {
	langs := make([]string, 0, len(subtitles))
	for lang := range subtitles {
		langs = append(langs, lang)
	}
	sort.Strings(langs)

	return langs
}

func langMatches(have, want string) bool {
	have = normalizeLang(have)
	want = normalizeLang(want)
	if have == "" || want == "" {
		return false
	}

	return have == want || strings.HasPrefix(have, want+"-") || strings.HasPrefix(want, have+"-")
}

func normalizeLang(lang string) string {
	lang = strings.TrimSpace(strings.ToLower(lang))
	lang = strings.ReplaceAll(lang, "_", "-")

	return lang
}

func subtitleDeclaredType(ext string) string {
	switch strings.TrimPrefix(strings.ToLower(strings.TrimSpace(ext)), ".") {
	case "vtt", "webvtt":
		return "text/vtt"
	case "srt":
		return "text/srt"
	case "json":
		return "application/json"
	case "html", "htm":
		return "text/html"
	case "txt":
		return "text/plain"
	default:
		return ""
	}
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

func isMissingTranscriptReason(reason string) bool {
	lower := strings.ToLower(reason)

	return strings.Contains(lower, "rss item has no podcast:transcript tag") ||
		strings.Contains(lower, "description/content has no transcript link") ||
		strings.Contains(lower, "no description or content to search") ||
		strings.Contains(lower, "all providers failed to produce transcript")
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

func commandAvailable(path string) bool {
	path = strings.TrimSpace(path)
	if path == "" {
		return false
	}
	if strings.Contains(path, string(os.PathSeparator)) {
		info, err := os.Stat(path)

		return err == nil && !info.IsDir()
	}
	_, err := exec.LookPath(path)

	return err == nil
}

// fetchHTTPPage fetches a web page via plain HTTP GET.
// Falls back to opencli (browser-based) when the server blocks the request
// (e.g. zhihu ZSE anti-bot protection).
func (f *Fetcher) fetchHTTPPage(ctx context.Context, rawURL string) *ContentFetchResult {
	timeout := httputil.DefaultClientTimeout
	if f.HTTPClient != nil && f.HTTPClient.Timeout > 0 {
		timeout = f.HTTPClient.Timeout
	}
	data, err := httputil.GetBytes(ctx, rawURL, httputil.RequestOptions{Timeout: timeout})
	if err == nil {
		return f.handleHTTPPageData(ctx, rawURL, data)
	}

	slog.Warn("HTTP fetch failed", "url", rawURL, "error", err)

	if f.OpenCLIFallback {
		// opencli fallback with a timeout so it can't hang
		// (e.g. if Chrome isn't running with the debug port).
		openCtx, cancel := context.WithTimeout(ctx, 45*time.Second)
		defer cancel()

		result := f.fetchWithOpenCLI(openCtx, rawURL)
		if result.Error == "" {
			quality := assessContentQuality(result.Title, result.Body, result.SourceURL)
			if !quality.OK {
				return extractFailure(rawURL, "low quality opencli content: "+quality.Reason)
			}

			return result
		}

		slog.Warn("opencli fallback also failed", "url", rawURL, "error", result.Error)
	}

	// Distinguish between HTTP-level errors (anti-bot, 4xx -> resolve failure)
	// and network-level errors (DNS, timeout -> fetch failure).
	errorStr := err.Error()
	if isHTTPBlockError(err) {
		errorStr = "resolve: " + errorStr
	}

	return &ContentFetchResult{SourceURL: rawURL, Error: errorStr}
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

func (f *Fetcher) ensureContentQuality(ctx context.Context, result *ContentFetchResult) *ContentFetchResult {
	if result == nil || result.Error != "" {
		return result
	}
	quality := assessContentQuality(result.Title, result.Body, result.SourceURL)
	if quality.OK {
		return result
	}
	if !f.OpenCLIFallback {
		return extractFailure(result.SourceURL, "low quality HTTP content: "+quality.Reason)
	}

	slog.Warn("HTTP extraction low quality, trying opencli fallback",
		"url", result.SourceURL, "reason", quality.Reason)
	openCtx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()

	fallback := f.fetchWithOpenCLI(openCtx, result.SourceURL)
	if fallback.Error != "" {
		return extractFailure(result.SourceURL, "low quality HTTP content: "+quality.Reason+"; opencli: "+fallback.Error)
	}
	fallbackQuality := assessContentQuality(fallback.Title, fallback.Body, fallback.SourceURL)
	if !fallbackQuality.OK {
		return extractFailure(result.SourceURL, "low quality opencli content: "+fallbackQuality.Reason)
	}

	return fallback
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

func socialShellLike(lower, body string) bool {
	return strings.Contains(lower, "log in") || strings.Contains(lower, "sign up") ||
		strings.Contains(lower, "javascript") || len([]rune(strings.TrimSpace(body))) < 500
}

func isLinkHeavy(body string) bool {
	links := strings.Count(body, "](") + len(xurls.Strict().FindAllString(body, -1))
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
		return &ContentFetchResult{SourceURL: rawURL, Error: reason}
	}

	return &ContentFetchResult{SourceURL: rawURL, Error: "extract: " + reason}
}

func cleanExtractReason(reason string) string {
	return strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(reason), "extract:"))
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

func isBilibiliURL(lowerURL string) bool {
	return strings.Contains(lowerURL, "bilibili.com") || strings.Contains(lowerURL, "b23.tv")
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
	path := lowerURL
	if err == nil {
		path = strings.ToLower(parsed.Path)
	}
	for _, suffix := range []string{".mp3", ".m4a", ".aac", ".wav", ".flac", ".ogg", ".opus"} {
		if strings.HasSuffix(path, suffix) {
			return true
		}
	}

	return false
}

func metadataLooksChinese(meta *ytdlpMetadata) bool {
	if meta == nil {
		return false
	}
	language := normalizeLang(meta.Language)
	if strings.HasPrefix(language, "zh") || language == "cmn" || language == "zho" || language == "chi" {
		return true
	}

	return containsHan(meta.Title + "\n" + meta.Description)
}

func containsHan(s string) bool {
	for _, r := range s {
		if unicode.Is(unicode.Han, r) {
			return true
		}
	}

	return false
}

// fetchWithOpenCLI uses the opencli browser tool to extract page content.
// opencli drives a real Chrome window to handle JS-rendered pages.
func (f *Fetcher) fetchWithOpenCLI(ctx context.Context, rawURL string) *ContentFetchResult {
	// opencli web read writes to a file by default; --stdout pipes content.
	cmd := exec.CommandContext(ctx, "opencli", "web", "read",
		"--url", rawURL,
		"--stdout",
	)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return &ContentFetchResult{SourceURL: rawURL,
			Error: fmt.Sprintf("opencli: %v (stderr: %s)", err, strings.TrimSpace(stderr.String()))}
	}

	body := stdout.String()
	if body == "" {
		return &ContentFetchResult{SourceURL: rawURL, Error: "opencli returned empty content"}
	}

	body = textutil.TruncateUTF8(body, f.MaxBodySize*3)

	// opencli web read returns Markdown; first heading line is the title.
	title := rawURL
	for line := range strings.SplitSeq(body, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") {
			title = strings.TrimLeft(trimmed, "# ")

			break
		}
	}

	slog.Info("opencli fetch succeeded", "url", rawURL, "bodyLen", len(body))

	return &ContentFetchResult{Title: title, Body: body, SourceURL: rawURL}
}

// extractTitle extracts the <title> from HTML content using goquery.
func extractTitle(htmlContent string) string {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(htmlContent))
	if err != nil {
		return ""
	}

	return strings.TrimSpace(doc.Find("title").First().Text())
}

func getGHToken() string {
	if token := os.Getenv("GITHUB_TOKEN"); token != "" {
		return token
	}

	return os.Getenv("GH_TOKEN")
}
