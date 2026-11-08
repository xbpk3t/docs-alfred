package fetch

import (
	"context"
	"fmt"
	"github.com/xbpk3t/docs-alfred/internal/docs/wiki/types"
	"log/slog"
	"net/url"
	"os"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/cloudflare/ahocorasick"
	"github.com/go-resty/resty/v2"
	"github.com/google/go-github/v70/github"
	"github.com/mmcdole/gofeed"
	"github.com/samber/lo"
	"github.com/xbpk3t/docs-alfred/internal/rss/transcript"
	"github.com/xbpk3t/docs-alfred/pkg/httputil"
	"github.com/xbpk3t/docs-alfred/pkg/opencli"
	"github.com/xbpk3t/docs-alfred/pkg/textutil"
	"github.com/xbpk3t/docs-alfred/pkg/urlutil"
	"golang.org/x/sync/errgroup"
)

const noneVal = "none"

// Fetcher handles fetching content from various sources.
type Fetcher struct {
	driver        ContentDriver
	mediaDriver   ContentDriver
	fallback      ContentDriver
	mediaFallback ContentDriver
	GHClient      *resty.Client
	GHBaseURL     string
	MaxBodySize   int
	MediaEnabled  bool
}

// FetcherOption customizes content fetching behavior.
type FetcherOption func(*Fetcher)

// WithDriver sets the text/default content driver for the fetcher.
func WithDriver(driver ContentDriver) FetcherOption {
	return func(f *Fetcher) { f.driver = driver }
}

// WithMediaDriver overrides the driver used for video/audio URLs.
func WithMediaDriver(driver ContentDriver) FetcherOption {
	return func(f *Fetcher) { f.mediaDriver = driver }
}

// WithMediaFallbackDriver sets a media-only fallback (for example opencli
// Bilibili summary when the yt-dlp media driver has no transcript).
func WithMediaFallbackDriver(driver ContentDriver) FetcherOption {
	return func(f *Fetcher) { f.mediaFallback = driver }
}

// WithFallbackDriver sets a secondary driver. Generic (non-adapter) URLs try
// the fallback first, and adapter URLs fall back to it only when the primary
// driver returns nothing usable. This keeps browser-dependent opencli calls
// reserved for sites that actually need a browser.
func WithFallbackDriver(driver ContentDriver) FetcherOption {
	return func(f *Fetcher) { f.fallback = driver }
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
	if f.mediaDriver == nil {
		f.mediaDriver = f.driver
	}
	if f.MaxBodySize <= 0 {
		f.MaxBodySize = 5000
	}

	return f
}

// FetchContent fetches content based on the URL pattern.
// GitHub repos and podcasts are handled directly; all other URLs
// are delegated to the configured ContentDriver.
func (f *Fetcher) FetchContent(ctx context.Context, urlStr, contentType string) *types.ContentFetchResult {
	slog.Info("FetchContent", "url", urlStr, "type", contentType)

	if err := urlutil.ValidateURL(urlStr); err != nil {
		return extractFailureWithReason(urlStr, err.Error(), types.FailureReasonHTTPError)
	}

	urlStr = normalizeGitHubAPIURL(urlStr)

	u := strings.ToLower(urlStr)
	if contentType == "" {
		contentType = DetectContentType(u)
	}
	if _, ok := urlutil.GitHubOwnerRepo(urlStr); ok {
		return f.fetchGitHubRepo(ctx, urlStr)
	}

	// PDF is not consumable by the browser readers. arXiv has a stable HTML
	// abstract page, so use that instead of archiving an empty "untitled" PDF.
	if id := arxivPDFID(urlStr); id != "" {
		return f.fetchArxivPDF(ctx, urlStr, id)
	}
	if isPDFURL(urlStr) {
		return extractFailureWithReason(urlStr, "pdf extraction unsupported", types.FailureReasonPDFUnsupported)
	}

	// Podcast/audio bypasses the driver — direct to podcast transcript pipeline.
	if result := f.fetchPodcast(ctx, urlStr, contentType); result != nil {
		return result
	}
	// A podcast-looking URL that is not a feed is an ordinary page. Do not let
	// its audio content type bypass the text quality gate.
	if contentType == types.ContentAudio {
		contentType = types.ContentText
	}

	return f.fetchFromDrivers(ctx, urlStr, contentType)
}

// fetchArxivPDF fetches the HTML abstract page for an arXiv PDF and rewrites
// the source back to the original PDF URL.
func (f *Fetcher) fetchArxivPDF(ctx context.Context, rawURL, id string) *types.ContentFetchResult {
	abstractURL := "https://arxiv.org/abs/" + id
	if f.fallback != nil {
		if fast := f.fetchUsable(ctx, f.fallback, abstractURL, types.ContentText); fast != nil {
			fast.SourceURL = rawURL

			return fast
		}
	}
	result := f.driver.FetchContent(ctx, abstractURL, types.ContentText)
	if result != nil {
		result.SourceURL = rawURL
	}

	return f.ensureFetchedQuality(result, types.ContentText, rawURL)
}

// fetchPodcast routes podcast/audio URLs to the transcript pipeline. It
// returns nil for podcast-looking URLs that are not feeds (for example a
// bestblogs.dev episode page), so the caller falls through to the drivers.
func (f *Fetcher) fetchPodcast(ctx context.Context, rawURL, contentType string) *types.ContentFetchResult {
	lowerURL := strings.ToLower(rawURL)
	if contentType != types.ContentAudio && !isPodcastLikeURL(lowerURL) && !isDirectAudioURL(lowerURL) {
		return nil
	}
	if !f.MediaEnabled {
		// Only genuine media is unusable when media extraction is off. A page
		// whose URL merely contains "podcast" can still use the normal drivers.
		if isDirectAudioURL(lowerURL) || isXiaoyuzhouEpisodeURL(lowerURL) {
			return extractFailureWithReason(rawURL, "media extraction disabled", types.FailureReasonEmpty)
		}

		return nil
	}

	return f.fetchPodcastTranscript(ctx, rawURL)
}

// fetchFromDrivers delegates to the configured drivers. Static (non-adapter)
// URLs prefer the fallback HTTP driver so the shared opencli browser tab is
// reserved for adapter sites; adapter URLs fall back to it only when the
// primary driver returns nothing usable.
func (f *Fetcher) fetchFromDrivers(ctx context.Context, rawURL, contentType string) *types.ContentFetchResult {
	needsAdapter := opencli.NeedsSiteAdapter(rawURL)
	// The static HTTP driver never extracts a transcript, and ingest rejects a
	// video without one as too_short. A fallback "rescue" can therefore only
	// replace a retryable extraction failure with a permanent one — never call
	// it for video.
	canFallback := f.fallback != nil && contentType != types.ContentVideo
	if fast := f.fetchGenericFallback(ctx, rawURL, contentType, needsAdapter, canFallback); fast != nil {
		return fast
	}

	isMedia := contentType == types.ContentVideo || contentType == types.ContentAudio
	driver := f.driver
	if isMedia && f.mediaDriver != nil {
		driver = f.mediaDriver
	}

	// Single quality gate: a driver can return a 200 anti-bot shell without an
	// error; that is still a failed primary extraction and deserves the
	// fallback.
	gated := f.ensureFetchedQuality(driver.FetchContent(ctx, rawURL, contentType), contentType, rawURL)
	if rescue := f.fetchRescue(ctx, rawURL, contentType, needsAdapter, canFallback, isMedia, driver, gated); rescue != nil {
		return rescue
	}

	return gated
}

// fetchGenericFallback lets the static HTTP driver serve non-adapter pages
// before the primary renderer runs. A thin landing/navigation page passes the
// 120-rune gate but would suppress the primary renderer, so require enough
// body to look like an article.
func (f *Fetcher) fetchGenericFallback(
	ctx context.Context, rawURL, contentType string, needsAdapter, canFallback bool,
) *types.ContentFetchResult {
	if !canFallback || needsAdapter {
		return nil
	}
	fast := f.fetchUsable(ctx, f.fallback, rawURL, contentType)
	if fast == nil || utf8.RuneCountInString(fast.Body) < minFallbackBodyRunes {
		return nil
	}
	slog.Debug("fallback driver served generic url", "url", rawURL, "driver", f.fallback.Name())

	return fast
}

// fetchRescue tries the applicable fallback after the primary driver failed:
// the static HTTP driver for adapter sites, or the media driver for a Bilibili
// video whose subtitle track came up empty.
func (f *Fetcher) fetchRescue(
	ctx context.Context, rawURL, contentType string,
	needsAdapter, canFallback, isMedia bool,
	driver ContentDriver, gated *types.ContentFetchResult,
) *types.ContentFetchResult {
	var fallback ContentDriver
	switch {
	case canFallback && needsAdapter && gated.Error != "":
		fallback = f.fallback
	// Bilibili's official AI summary lives in opencli, not yt-dlp. Use the
	// media-only fallback when the selected yt-dlp path found no transcript.
	case isMedia && driver == f.mediaDriver && f.mediaFallback != nil &&
		gated.Error != "" && gated.FailureReason == types.FailureReasonTooShort &&
		isBilibiliVideoURL(rawURL):
		fallback = f.mediaFallback
	}
	if fallback == nil {
		return nil
	}
	rescue := f.fetchUsable(ctx, fallback, rawURL, contentType)
	if rescue == nil {
		return nil
	}
	slog.Info("fallback driver served url",
		"url", rawURL, "primary", driver.Name(), "fallback", fallback.Name())

	return rescue
}

// minFallbackBodyRunes is the minimum body length for the generic HTTP
// fallback to be preferred over the primary renderer. Below this a page is
// usually navigation/landing chrome rather than an article.
const minFallbackBodyRunes = 300

// fetchUsable runs a driver and returns nil unless it produced content that
// passes the quality gate, so a driver that returns a login shell is not
// mistaken for a successful fallback.
func (f *Fetcher) fetchUsable(
	ctx context.Context,
	driver ContentDriver,
	urlStr, contentType string,
) *types.ContentFetchResult {
	if driver == nil {
		return nil
	}
	result := f.ensureFetchedQuality(driver.FetchContent(ctx, urlStr, contentType), contentType, urlStr)
	if result.Error != "" {
		// The caller reports the primary driver's error; keep the fallback's
		// own reason visible here instead of discarding it.
		slog.Debug("driver result unusable", "url", urlStr, "driver", driver.Name(), "reason", result.Error)

		return nil
	}

	return result
}

// ensureFetchedQuality is the single post-driver content gate. It runs for all
// drivers (opencli, http-readability, future ones) so a driver that returns a
// 200 anti-bot page cannot leak that page into classification. Repo and media
// results keep their own contracts.
func (f *Fetcher) ensureFetchedQuality(
	result *types.ContentFetchResult,
	contentType, fallbackURL string,
) *types.ContentFetchResult {
	if result == nil {
		return extractFailureWithReason(fallbackURL, "driver returned empty result", types.FailureReasonHTTPError)
	}
	if result.Error != "" {
		return result
	}

	var quality contentQuality
	if contentType == types.ContentText {
		quality = assessContentQuality(result.Title, result.Body, result.SourceURL)
	} else {
		// Non-text results (video metadata, podcast transcripts) must still not
		// archive anti-bot/browser-error shells. Skip only the text-specific
		// length/social/link checks.
		quality = assessShellSignals(result.Title, result.Body)
	}
	if quality.OK {
		return result
	}

	slog.Warn("fetched content rejected by quality gate",
		"url", result.SourceURL, "reason", quality.Reason, "failureReason", quality.FailureReason)

	return extractFailureWithReason(result.SourceURL, "low quality content: "+quality.Reason, quality.FailureReason)
}

// normalizeGitHubAPIURL rewrites api.github.com/repos/<owner>/<repo> to the
// canonical github.com page. The API endpoint returns a JSON error body for
// 404s, which the generic driver would otherwise archive as an article.
func normalizeGitHubAPIURL(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil || strings.ToLower(parsed.Hostname()) != "api.github.com" {
		return rawURL
	}
	parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	// Only the exact /repos/<owner>/<repo> endpoint maps to the repo page.
	// Subpaths (issues, contents, ...) must not be silently collapsed.
	if len(parts) != 3 || parts[0] != "repos" || parts[1] == "" || parts[2] == "" {
		return rawURL
	}

	return "https://github.com/" + parts[1] + "/" + strings.TrimSuffix(parts[2], ".git")
}

// arxivPDFID returns the arXiv identifier when rawURL points at a PDF.
func arxivPDFID(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	if !strings.EqualFold(parsed.Hostname(), "arxiv.org") &&
		!strings.EqualFold(parsed.Hostname(), "www.arxiv.org") {
		return ""
	}
	path := strings.Trim(parsed.Path, "/")
	if !strings.HasPrefix(path, "pdf/") {
		return ""
	}
	path = strings.TrimPrefix(path, "pdf/")
	if path == "" {
		return ""
	}
	path = strings.TrimSuffix(path, ".pdf")

	return path
}

// isPDFURL reports whether the URL itself is a PDF document.
func isPDFURL(rawURL string) bool {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	path := strings.ToLower(parsed.Path)

	return strings.HasSuffix(path, ".pdf")
}

// --- GitHub repo fetching ---

func (f *Fetcher) fetchGitHubRepo(ctx context.Context, rawURL string) *types.ContentFetchResult {
	repoRef, ok := urlutil.GitHubOwnerRepo(rawURL)
	if !ok {
		return &types.ContentFetchResult{SourceURL: rawURL, Error: "not a valid GitHub repo URL"}
	}
	owner, repoName := repoRef.Owner, repoRef.Name

	ghClient, err := f.githubClient()
	if err != nil {
		return &types.ContentFetchResult{SourceURL: rawURL, Error: err.Error()}
	}
	var (
		repoData *github.Repository
		readme   *github.RepositoryContent
	)
	g, gctx := errgroup.WithContext(ctx)
	g.Go(func() error {
		var err error
		repoData, _, err = ghClient.Repositories.Get(gctx, owner, repoName)

		return err
	})
	// README failures are non-fatal: a missing README leaves the metadata-only body.
	g.Go(func() error {
		var err error
		readme, _, err = ghClient.Repositories.GetReadme(gctx, owner, repoName, nil)
		if err != nil {
			readme = nil
		}

		return nil
	})
	if err := g.Wait(); err != nil {
		reason := types.FailureReasonHTTPError
		if strings.Contains(err.Error(), "404") || strings.Contains(err.Error(), "Not Found") {
			reason = types.FailureReasonNotFound
		}

		return &types.ContentFetchResult{
			SourceURL:     rawURL,
			Error:         err.Error(),
			FailureKind:   types.FailureResolve,
			FailureReason: reason,
		}
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

	// Metadata alone is not digestible: a README carries what the project
	// actually does and how it is used.
	if readme != nil {
		if content, contentErr := readme.GetContent(); contentErr == nil && strings.TrimSpace(content) != "" {
			body += "\n\nREADME:\n\n" + textutil.TruncateUTF8(content, f.MaxBodySize*3)
		}
	}

	title := fmt.Sprintf("%s/%s", owner, repoName)
	if repoData.GetDescription() != "" {
		title = fmt.Sprintf("%s/%s — %s", owner, repoName, repoData.GetDescription())
	}

	return &types.ContentFetchResult{Title: title, Body: body, SourceURL: rawURL}
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

// fetchPodcastTranscript resolves a podcast episode or feed URL to a
// transcript. It returns nil when the URL only looked podcast-like but is not
// an RSS feed, so the caller can fall through to the normal content drivers.
func (f *Fetcher) fetchPodcastTranscript(ctx context.Context, rawURL string) *types.ContentFetchResult {
	lowerURL := strings.ToLower(rawURL)
	if isDirectAudioURL(lowerURL) {
		return extractFailure(rawURL, "rss transcript unavailable: direct audio URL has no RSS metadata")
	}
	if isXiaoyuzhouEpisodeURL(lowerURL) {
		ep := &transcript.EpisodeRef{URL: rawURL}

		return f.fetchTranscriptFromEpisode(ctx, ep, rawURL)
	}

	feed, err := gofeed.NewParser().ParseURLWithContext(rawURL, ctx)
	if err != nil {
		// A URL that is clearly supposed to be a feed (.xml/.rss//feed) is
		// retryable: the failure may be a transient network/5xx error. A
		// podcast-looking episode page that is not a feed falls through.
		if isRSSFeedLike(lowerURL) {
			return extractFailureWithReason(
				rawURL,
				"rss transcript unavailable: feed parse: "+err.Error(),
				types.FailureReasonHTTPError,
			)
		}
		slog.Info("podcast-like url is not an rss feed; using normal drivers", "url", rawURL, "error", err)

		return nil
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
) *types.ContentFetchResult {
	providers := []transcript.Provider{
		transcript.NewXiaoyuzhouProvider(""),
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

func mediaContentResult(title, rawURL, source, content string, maxBodySize int) *types.ContentFetchResult {
	content = strings.TrimSpace(content)
	content = textutil.TruncateUTF8(content, maxBodySize)
	body := fmt.Sprintf("Title: %s\nURL: %s\nTranscript source: %s\n\n%s", title, rawURL, source, content)

	return &types.ContentFetchResult{Title: title, Body: body, SourceURL: rawURL}
}

// --- Content quality assessment ---

type contentQuality struct {
	Reason        string
	FailureReason types.FailureReason
	OK            bool
}

func assessContentQuality(title, body, rawURL string) contentQuality {
	trimmed := strings.TrimSpace(body)
	if trimmed == "" {
		return contentQuality{Reason: "empty body", FailureReason: types.FailureReasonEmpty}
	}

	// Normalize once; the shell checks and the text checks share the values.
	titleLower := strings.ToLower(normalizeShellText(strings.TrimSpace(title)))
	bodyLower := strings.ToLower(normalizeShellText(trimmed))
	runes := utf8.RuneCountInString(trimmed)
	if reason, failureReason := shellSignalReason(titleLower, bodyLower, runes); failureReason != "" {
		return contentQuality{Reason: reason, FailureReason: failureReason}
	}

	lower := titleLower + "\n" + bodyLower

	// The broad legacy matcher is only safe on short bodies: an article about
	// 404/login pages must not be rejected for mentioning them.
	if runes < 400 && errorShellMatcher.Match([]byte(lower)) != nil {
		return contentQuality{Reason: "matched error/login shell", FailureReason: types.FailureReasonAntiBot}
	}

	domain := urlutil.Domain(rawURL)
	if isSocialShellDomain(domain) && socialShellPatterns(lower) {
		return contentQuality{Reason: "social/login shell", FailureReason: types.FailureReasonAntiBot}
	}
	if runes < 120 {
		return contentQuality{Reason: "too short", FailureReason: types.FailureReasonTooShort}
	}
	if isLinkHeavy(trimmed) {
		return contentQuality{Reason: "link-heavy navigation shell", FailureReason: types.FailureReasonIndexPage}
	}

	return contentQuality{OK: true}
}

// assessShellSignals runs only the content-type-independent shell checks. It is
// used for video/audio results, where text length/social/link heuristics do not
// apply but an anti-bot page must still be rejected.
func assessShellSignals(title, body string) contentQuality {
	trimmed := strings.TrimSpace(body)
	if trimmed == "" {
		return contentQuality{Reason: "empty body", FailureReason: types.FailureReasonEmpty}
	}

	titleLower := strings.ToLower(normalizeShellText(strings.TrimSpace(title)))
	bodyLower := strings.ToLower(normalizeShellText(trimmed))
	runes := utf8.RuneCountInString(trimmed)
	if reason, failureReason := shellSignalReason(titleLower, bodyLower, runes); failureReason != "" {
		return contentQuality{Reason: reason, FailureReason: failureReason}
	}

	return contentQuality{OK: true}
}

// shellSignalReason detects the shell/error pages this pipeline has actually
// seen in production. It is deliberately more specific than LowQualityMatcher
// so legitimate articles that discuss anti-bot pages are not rejected. It
// takes the already-normalized lowercased title and body.
func shellSignalReason(titleLower, bodyLower string, runes int) (string, types.FailureReason) {
	if reason, failureReason := shellTitleReason(titleLower); failureReason != "" {
		return reason, failureReason
	}
	// Body keyword checks are only safe on short pages; a long article may
	// legitimately discuss Cloudflare, Zhihu verification, or browser errors.
	if runes < 1200 {
		if reason := cloudflareChallengeReason(bodyLower); reason != "" {
			return reason, types.FailureReasonCloudflare
		}
		if reason := zhihuVerificationReason(bodyLower); reason != "" {
			return reason, types.FailureReasonAntiBot
		}
		if reason, failureReason := browserErrorReason(bodyLower); reason != "" {
			return reason, failureReason
		}
	}
	if isJSONAPIError(bodyLower) {
		return "json api error body", types.FailureReasonNotFound
	}
	if reason := jsShellReason(bodyLower, runes); reason != "" {
		return reason, types.FailureReasonJSShell
	}
	if reason := accessDeniedReason(bodyLower, runes); reason != "" {
		return reason, types.FailureReasonAntiBot
	}
	if reason := partialExtractReason(bodyLower, runes); reason != "" {
		return reason, types.FailureReasonPartialExtract
	}

	return "", ""
}

func shellTitleReason(titleLower string) (string, types.FailureReason) {
	switch titleLower {
	case "安全验证":
		return "zhihu anti-bot verification page", types.FailureReasonAntiBot
	case "just a moment", "just a moment…", "just a moment...":
		return "cloudflare anti-bot: challenge page", types.FailureReasonCloudflare
	case "status:open":
		return "gerrit/SPA query shell", types.FailureReasonIndexPage
	case "loading…", "loading...":
		return "javascript shell: page still loading", types.FailureReasonJSShell
	case "untitled":
		return "empty extraction: untitled page", types.FailureReasonEmpty
	}

	return "", ""
}

func zhihuVerificationReason(bodyLower string) string {
	if strings.Contains(bodyLower, "系统监测到您的网络环境存在异常") ||
		strings.Contains(bodyLower, "人机验证") ||
		strings.Contains(bodyLower, "滑动验证") {
		return "zhihu/anti-bot verification page"
	}

	return ""
}

func browserErrorReason(bodyLower string) (string, types.FailureReason) {
	for _, pattern := range browserUnreachablePatterns {
		if strings.Contains(bodyLower, pattern) {
			return "browser error page: " + pattern, types.FailureReasonBrowserUnreachable
		}
	}
	for _, pattern := range browserTransientPatterns {
		if strings.Contains(bodyLower, pattern) {
			return "browser error page: " + pattern, types.FailureReasonBrowserError
		}
	}

	return "", ""
}

func jsShellReason(bodyLower string, runes int) string {
	if runes >= 800 {
		return ""
	}
	if strings.Contains(bodyLower, "loading…") ||
		strings.Contains(bodyLower, "loading...") ||
		strings.Contains(bodyLower, "please enable javascript") ||
		strings.Contains(bodyLower, "you need to enable javascript") ||
		strings.Contains(bodyLower, "this page requires javascript") {
		return "javascript shell: content not rendered"
	}

	return ""
}

func accessDeniedReason(bodyLower string, runes int) string {
	if runes >= 1500 {
		return ""
	}
	if strings.Contains(bodyLower, "verify you are human") ||
		strings.Contains(bodyLower, "are you a robot") ||
		strings.Contains(bodyLower, "access denied") {
		return "anti-bot/access-denied shell"
	}

	return ""
}

func partialExtractReason(bodyLower string, runes int) string {
	if runes >= 1200 {
		return ""
	}
	for _, pattern := range partialExtractPatterns {
		if strings.Contains(bodyLower, pattern) {
			return "partial extract: " + pattern
		}
	}

	return ""
}

// errorShellPatterns is the curated subset of LowQualityPatternsList that is
// safe to apply to arbitrary text: it omits broad words such as "captcha" and
// "forbidden" that appear in legitimate security articles.
var errorShellPatterns = []string{
	"this page requires javascript",
	"javascript is not available",
	"enable javascript",
	"please enable js",
	"please log in",
	"log in to continue",
	"sign in to continue",
	"404 not found",
	"page not found",
	"400 bad request",
}

var errorShellMatcher = ahocorasick.NewStringMatcher(errorShellPatterns)

var browserUnreachablePatterns = []string{
	"err_connection_closed",
	"err_connection_reset",
	"err_name_not_resolved",
	"err_internet_disconnected",
	"err_ssl",
	"this site can't be reached",
	"this site can’t be reached",
}

var browserTransientPatterns = []string{
	"unexpectedly closed the connection",
	"took too long to respond",
}

var partialExtractPatterns = []string{
	"continue reading",
	"subscribe to continue",
	"subscribe to read",
	"sign in to read",
	"this content is for subscribers",
}

// isJSONAPIError takes the already-normalized lowercased body.
func isJSONAPIError(bodyLower string) bool {
	if !strings.HasPrefix(bodyLower, "{") || utf8.RuneCountInString(bodyLower) > 800 {
		return false
	}
	if !strings.Contains(bodyLower, "\"message\"") && !strings.Contains(bodyLower, "\"error\"") {
		return false
	}

	return strings.Contains(bodyLower, "not found") ||
		strings.Contains(bodyLower, "\"status\": \"404\"") ||
		strings.Contains(bodyLower, "\"status\":404") ||
		strings.Contains(bodyLower, "documentation_url")
}

func normalizeShellText(s string) string {
	return strings.ReplaceAll(textutil.DecodeCommonHTMLEntities(s), " ", " ")
}

func cloudflareChallengeReason(lower string) string {
	if !strings.Contains(lower, "just a moment") {
		return ""
	}
	if textutil.ContainsAny(lower, []string{
		"checking your browser",
		"performing security verification",
		"verification successful",
		"cf-browser-verification",
		"cloudflare",
		"ray id:",
		"enable javascript and cookies",
	}) {
		return "cloudflare anti-bot: challenge page"
	}

	return ""
}

var LowQualityPatternsList = []string{
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

var LowQualityMatcher = ahocorasick.NewStringMatcher(LowQualityPatternsList)

func isSocialShellDomain(domain string) bool {
	switch strings.ToLower(domain) {
	case "x.com", "twitter.com", "mobile.twitter.com", "t.co", "instagram.com", "www.instagram.com":
		return true
	default:
		return false
	}
}

// socialShellPatterns matches only explicit login/chrome phrases. Broad words
// like "javascript" or "log in" appear in legitimate technical threads; the
// legacy short-body rejection also killed every tweet under 300 runes.

var socialShellMatcher = ahocorasick.NewStringMatcher([]string{
	"log in to continue",
	"sign in to continue",
	"sign up to continue",
	"enable javascript",
	"javascript is not available",
	"please enable js",
	"keyboard shortcuts",
	"keyboard shortcut",
	"to continue, please",
	"already have an account",
	"don't have an account",
})

func socialShellPatterns(lower string) bool {
	return len(socialShellMatcher.Match([]byte(lower))) > 0
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

func extractFailure(rawURL, reason string) *types.ContentFetchResult {
	return extractFailureWithReason(rawURL, reason, "")
}

func extractFailureWithReason(
	rawURL, reason string,
	failureReason types.FailureReason,
) *types.ContentFetchResult {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		reason = "content extraction failed"
	}
	if strings.HasPrefix(reason, "extract:") {
		return &types.ContentFetchResult{
			SourceURL:     rawURL,
			Error:         reason,
			FailureKind:   types.FailureExtract,
			FailureReason: failureReason,
		}
	}

	return &types.ContentFetchResult{
		SourceURL:     rawURL,
		Error:         "extract: " + reason,
		FailureKind:   types.FailureExtract,
		FailureReason: failureReason,
	}
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

// isXiaoyuzhouEpisodeURL reports a Xiaoyuzhou episode page, whose transcript
// comes from the Xiaoyuzhou API provider rather than RSS parsing.
func isXiaoyuzhouEpisodeURL(lowerURL string) bool {
	return strings.Contains(lowerURL, "xiaoyuzhoufm.com/episode/")
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

func DetectContentType(urlLower string) string {
	urlLower = strings.ToLower(strings.TrimSpace(urlLower))
	if isVideoURL(urlLower) {
		return types.ContentVideo
	}
	if strings.Contains(urlLower, "xiaoyuzhou") ||
		strings.Contains(urlLower, "podcast") ||
		strings.Contains(urlLower, "libsyn.com") {
		return types.ContentAudio
	}

	return types.ContentText
}
