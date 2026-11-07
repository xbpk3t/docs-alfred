package fetch

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode"

	"github.com/samber/lo"

	"github.com/xbpk3t/docs-alfred/internal/docs/wiki/types"
	"github.com/xbpk3t/docs-alfred/internal/rss/transcript"
	"github.com/xbpk3t/docs-alfred/pkg/cmdutil"
	"github.com/xbpk3t/docs-alfred/pkg/textutil"
)

// ytdlpDriver extracts video subtitles through yt-dlp. It never plays media
// and never opens a browser: yt-dlp only reads metadata and downloads the
// selected subtitle track.
type ytdlpDriver struct {
	cookiesBrowser string
	subtitleLangs  []string
	maxBodySize    int
	mediaEnabled   bool
}

// hybridDriver routes video URLs to yt-dlp and everything else to opencli, so
// a full digest can run with the yt-dlp video path while text adapters keep
// working. Bilibili videos without subtitles fall back to opencli because its
// official AI summary is not available through yt-dlp.
type hybridDriver struct {
	media ContentDriver
	text  ContentDriver
}

func newYTDLPDriver(opts DriverOptions) *ytdlpDriver {
	maxBody := opts.MaxBodySize
	if maxBody <= 0 {
		maxBody = 5000
	}

	return &ytdlpDriver{
		maxBodySize:    maxBody,
		mediaEnabled:   opts.MediaEnabled,
		cookiesBrowser: opts.YTDLPCookiesBrowser,
		subtitleLangs:  opts.YTDLPSubtitleLangs,
	}
}

func newHybridDriver(opts DriverOptions) *hybridDriver {
	return &hybridDriver{
		media: newYTDLPDriver(opts),
		text:  newOpenCLIDriver(opts),
	}
}

func (d *ytdlpDriver) Name() string { return "ytdlp" }

func (d *hybridDriver) Name() string { return "hybrid" }

// UsesHTTPFallback lets the ingest fetcher route non-video pages to the static
// HTTP driver instead of failing them here.
func (d *ytdlpDriver) UsesHTTPFallback() bool { return true }

func (d *hybridDriver) UsesHTTPFallback() bool { return true }

func (d *ytdlpDriver) FetchContent(ctx context.Context, urlStr, _ string) *types.ContentFetchResult {
	if !isVideoURL(strings.ToLower(urlStr)) {
		return extractFailureWithReason(
			urlStr,
			"ytdlp driver only handles video URLs; use hybrid or opencli",
			types.FailureReasonEmpty,
		)
	}
	if !d.mediaEnabled {
		return extractFailure(urlStr, "media extraction disabled")
	}

	return d.fetchVideo(ctx, urlStr)
}

func (d *hybridDriver) FetchContent(ctx context.Context, urlStr, contentType string) *types.ContentFetchResult {
	lowerURL := strings.ToLower(urlStr)
	if !isVideoURL(lowerURL) {
		return d.text.FetchContent(ctx, urlStr, contentType)
	}

	result := d.media.FetchContent(ctx, urlStr, contentType)
	if result != nil && result.Error != "" &&
		result.FailureReason == types.FailureReasonTooShort &&
		isBilibiliVideoURL(lowerURL) {
		slog.Info("hybrid: yt-dlp has no subtitle, using opencli bilibili summary", "url", urlStr)

		return d.text.FetchContent(ctx, urlStr, contentType)
	}

	return result
}

func (d *ytdlpDriver) fetchVideo(ctx context.Context, rawURL string) *types.ContentFetchResult {
	meta, metaFailure := d.loadMetadata(ctx, rawURL)
	if metaFailure != nil {
		return metaFailure
	}

	langs := d.subtitleLangsFor(rawURL, meta)
	var body string
	var lang string
	var auto bool
	var failure *types.ContentFetchResult
	if pickedLang, pickedAuto, ok := pickYTDLPSubtitle(meta, langs); ok {
		lang, auto = pickedLang, pickedAuto
		body, failure = d.downloadSubtitle(ctx, rawURL, lang, auto)
	} else {
		// Bilibili's --dump-json often returns an empty subtitles map even
		// though --write-subs can fetch the ai-zh track. Fall back to a
		// language-filtered subtitle download instead of declaring no captions.
		body, failure = d.downloadSubtitleFallback(ctx, rawURL, langs)
		if failure == nil && body == "" {
			return extractFailureWithReason(
				rawURL,
				"no subtitle or automatic caption available",
				types.FailureReasonTooShort,
			)
		}
	}
	if failure != nil {
		return failure
	}
	body = strings.TrimSpace(body)
	if body == "" {
		return extractFailureWithReason(
			rawURL,
			"yt-dlp subtitle normalized to empty content",
			types.FailureReasonPartialExtract,
		)
	}

	title := strings.TrimSpace(meta.Title)
	if title == "" {
		title = rawURL
	}
	content := types.TranscriptSectionMarker + "\n\n" + body
	slog.Info("yt-dlp subtitle fetched", "url", rawURL, "lang", lang, "auto", auto, "bodyLen", len(content))

	return &types.ContentFetchResult{
		Title:         title,
		Body:          textutil.TruncateUTF8(content, d.maxBodySize*3),
		SourceURL:     rawURL,
		HasTranscript: true,
	}
}

// ytdlpMetadata is the subset of `yt-dlp --dump-json` the driver needs.
type ytdlpMetadata struct {
	Subtitles         map[string][]ytdlpSubtitle `json:"subtitles"`
	AutomaticCaptions map[string][]ytdlpSubtitle `json:"automatic_captions"`
	Title             string                     `json:"title"`
}

type ytdlpSubtitle struct {
	Ext string `json:"ext"`
}

func (d *ytdlpDriver) loadMetadata(ctx context.Context, rawURL string) (*ytdlpMetadata, *types.ContentFetchResult) {
	args := append(d.baseArgs(), "--dump-json", rawURL)
	stdout, stderr, err := cmdutil.RunSeparateLarge(ctx, "yt-dlp", args...)
	if err != nil {
		return nil, ytdlpFailure(rawURL, "metadata", stdout, stderr, err)
	}

	var meta ytdlpMetadata
	if err := json.NewDecoder(bytes.NewReader(stdout)).Decode(&meta); err != nil {
		return nil, extractFailureWithReason(
			rawURL,
			"yt-dlp metadata parse: "+err.Error(),
			types.FailureReasonPartialExtract,
		)
	}

	return &meta, nil
}

func (d *ytdlpDriver) downloadSubtitle(
	ctx context.Context,
	rawURL, lang string,
	auto bool,
) (string, *types.ContentFetchResult) {
	body, found, failure := d.downloadSubtitleOnce(ctx, rawURL, lang, auto)
	if failure != nil {
		return "", failure
	}
	if !found {
		return "", extractFailureWithReason(
			rawURL,
			fmt.Sprintf("yt-dlp reported subtitle %q but wrote no vtt/srt file", lang),
			types.FailureReasonPartialExtract,
		)
	}

	return body, nil
}

// downloadSubtitleFallback tries manual then automatic subtitles with a
// language filter, for metadata that omitted the subtitle tracks (Bilibili).
func (d *ytdlpDriver) downloadSubtitleFallback(
	ctx context.Context,
	rawURL string,
	langs []string,
) (string, *types.ContentFetchResult) {
	filters := append([]string(nil), langs...)
	if isBilibiliVideoURL(strings.ToLower(rawURL)) {
		filters = append(filters, "ai-zh")
	}
	filter := strings.Join(lo.Uniq(filters), ",")

	for _, auto := range []bool{false, true} {
		body, found, failure := d.downloadSubtitleOnce(ctx, rawURL, filter, auto)
		if failure != nil {
			return "", failure
		}
		if found {
			return body, nil
		}
	}

	return "", nil
}

func (d *ytdlpDriver) downloadSubtitleOnce(
	ctx context.Context,
	rawURL, langFilter string,
	auto bool,
) (string, bool, *types.ContentFetchResult) {
	dir, err := os.MkdirTemp("", "wiki-ytdlp-*")
	if err != nil {
		return "", false, extractFailureWithReason(
			rawURL,
			"yt-dlp temp dir: "+err.Error(),
			types.FailureReasonPartialExtract,
		)
	}
	defer func() {
		if rmErr := os.RemoveAll(dir); rmErr != nil {
			slog.Warn("yt-dlp: failed to remove temp dir", "path", dir, "error", rmErr)
		}
	}()

	args := d.baseArgs()
	if auto {
		args = append(args, "--write-auto-subs")
	} else {
		args = append(args, "--write-subs")
	}
	args = append(args,
		"--sub-langs", langFilter,
		"--sub-format", "vtt/srt",
		"-o", filepath.Join(dir, "%(id)s.%(ext)s"),
		rawURL,
	)

	stdout, stderr, err := cmdutil.RunSeparateLarge(ctx, "yt-dlp", args...)
	if err != nil {
		return "", false, ytdlpFailure(rawURL, "subtitle", stdout, stderr, err)
	}

	// Any vtt/srt file is acceptable here: the language filter already did
	// the selection, and Bilibili can write ai-zh when metadata had no map.
	path := firstSubtitleFile(dir)
	if path == "" {
		return "", false, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return "", false, extractFailureWithReason(
			rawURL,
			"read yt-dlp subtitle: "+err.Error(),
			types.FailureReasonPartialExtract,
		)
	}

	contentType := strings.TrimPrefix(strings.ToLower(filepath.Ext(path)), ".")
	body := strings.TrimSpace(transcript.NormalizeContent(string(data), contentType))
	if body == "" {
		return "", true, extractFailureWithReason(
			rawURL,
			"yt-dlp subtitle normalized to empty content",
			types.FailureReasonPartialExtract,
		)
	}

	return body, true, nil
}

// baseArgs keeps yt-dlp offline from user config and media downloads.
func (d *ytdlpDriver) baseArgs() []string {
	args := []string{"--ignore-config", "--skip-download", "--no-playlist", "--no-warnings"}
	if d.cookiesBrowser != "" {
		args = append(args, "--cookies-from-browser", d.cookiesBrowser)
	}

	return args
}

// subtitleLangsFor prefers Chinese captions for Bilibili and for videos whose
// title is Chinese, English otherwise; both lists end with the other language
// as a fallback. A configured override replaces the whole list.
func (d *ytdlpDriver) subtitleLangsFor(rawURL string, meta *ytdlpMetadata) []string {
	if len(d.subtitleLangs) > 0 {
		return d.subtitleLangs
	}

	if isBilibiliVideoURL(strings.ToLower(rawURL)) || (meta != nil && containsHan(meta.Title)) {
		return []string{"zh-Hans", "zh-CN", "zh", "zh-Hant", "zh-TW", "zho", "chi", "en"}
	}

	return []string{"en", "en-US", "en-GB", "zh-Hans", "zh-CN", "zh"}
}

// pickYTDLPSubtitle prefers manual captions in any supported language; only
// when none exist does it accept automatic captions.
func pickYTDLPSubtitle(meta *ytdlpMetadata, langs []string) (string, bool, bool) {
	if meta == nil {
		return "", false, false
	}
	if lang, ok := pickYTDLPLang(meta.Subtitles, langs); ok {
		return lang, false, true
	}
	if lang, ok := pickYTDLPLang(meta.AutomaticCaptions, langs); ok {
		return lang, true, true
	}

	return "", false, false
}

func pickYTDLPLang(captions map[string][]ytdlpSubtitle, langs []string) (string, bool) {
	available := usableCaptionLangs(captions)
	if len(available) == 0 {
		return "", false
	}
	for _, want := range langs {
		for _, have := range available {
			if ytdlpLangMatches(have, want) {
				return have, true
			}
		}
	}
	// No preferred language exists; any caption beats none.
	return available[0], true
}

func usableCaptionLangs(captions map[string][]ytdlpSubtitle) []string {
	langs := make([]string, 0, len(captions))
	for lang, items := range captions {
		if hasUsableCaption(items) {
			langs = append(langs, lang)
		}
	}
	sort.Strings(langs)

	return langs
}

func hasUsableCaption(items []ytdlpSubtitle) bool {
	for _, item := range items {
		switch strings.ToLower(strings.TrimSpace(strings.TrimPrefix(item.Ext, "."))) {
		case "", "vtt", "srt":
			return true
		}
	}

	return false
}

func ytdlpLangMatches(have, want string) bool {
	have = normalizeYTDLPLang(have)
	want = normalizeYTDLPLang(want)
	if have == "" || want == "" {
		return false
	}
	if have == want {
		return true
	}

	return strings.HasPrefix(have, want+"-") || strings.HasPrefix(want, have+"-")
}

// normalizeYTDLPLang lowercases the tag and strips Bilibili's "ai-" prefix so
// "ai-zh" matches the zh preference list.
func normalizeYTDLPLang(lang string) string {
	lang = strings.ToLower(strings.TrimSpace(lang))
	lang = strings.ReplaceAll(lang, "_", "-")

	return strings.TrimPrefix(lang, "ai-")
}

func containsHan(s string) bool {
	for _, r := range s {
		if unicode.Is(unicode.Han, r) {
			return true
		}
	}

	return false
}

func isBilibiliVideoURL(lowerURL string) bool {
	return strings.Contains(lowerURL, "bilibili.com/video/") ||
		strings.Contains(lowerURL, "b23.tv/")
}

// firstSubtitleFile returns the first vtt/srt file yt-dlp wrote; the language
// filter already selected the track.
func firstSubtitleFile(dir string) string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}
	for _, entry := range entries {
		switch strings.ToLower(filepath.Ext(entry.Name())) {
		case ".vtt", ".srt":
			return filepath.Join(dir, entry.Name())
		}
	}

	return ""
}

// ytdlpFailure classifies a yt-dlp command failure. Network errors, bot checks
// and cookie failures stay retryable (partial_extract); only explicit
// "unavailable" output is permanent.
func ytdlpFailure(
	rawURL, stage string,
	stdout, stderr []byte,
	err error,
) *types.ContentFetchResult {
	detail := strings.TrimSpace(string(stderr))
	if detail == "" {
		detail = strings.TrimSpace(string(stdout))
	}
	if detail == "" {
		detail = err.Error()
	}
	detail = textutil.TruncateUTF8(detail, 1500)

	reason := types.FailureReasonPartialExtract
	if textutil.ContainsAny(strings.ToLower(detail), []string{
		"video unavailable",
		"this video is private",
		"has been removed",
		"http error 404",
		"http error 410",
		"unsupported url",
	}) {
		reason = types.FailureReasonNotFound
	}
	slog.Warn("yt-dlp command failed", "url", rawURL, "stage", stage, "reason", reason, "error", detail)

	return &types.ContentFetchResult{
		SourceURL:     rawURL,
		Error:         "yt-dlp " + stage + ": " + detail,
		FailureKind:   types.FailureExtract,
		FailureReason: reason,
	}
}
