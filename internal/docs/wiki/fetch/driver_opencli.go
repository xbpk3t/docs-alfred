package fetch

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/avast/retry-go/v4"
	yaml "github.com/goccy/go-yaml"
	"github.com/xbpk3t/docs-alfred/internal/docs/wiki/types"
	"github.com/xbpk3t/docs-alfred/pkg/cmdutil"
	"github.com/xbpk3t/docs-alfred/pkg/md"
	"github.com/xbpk3t/docs-alfred/pkg/opencli"
	"github.com/xbpk3t/docs-alfred/pkg/textutil"
)

// openCLIDriver fetches content via the opencli CLI tool.
// Site-specific adapters (youtube/bilibili/twitter...) extract structured content;
// generic URLs fall back to opencli web read.
type openCLIDriver struct {
	maxBodySize  int
	mediaEnabled bool
}

func newOpenCLIDriver(opts DriverOptions) *openCLIDriver {
	maxBody := opts.MaxBodySize
	if maxBody <= 0 {
		maxBody = 5000
	}

	return &openCLIDriver{
		maxBodySize:  maxBody,
		mediaEnabled: opts.MediaEnabled,
	}
}

func (d *openCLIDriver) Name() string { return "opencli" }

func (d *openCLIDriver) FetchContent(ctx context.Context, urlStr, contentType string) *types.ContentFetchResult {
	u := strings.ToLower(urlStr)

	if isVideoURL(u) && !d.mediaEnabled {
		return extractFailure(urlStr, "media extraction disabled")
	}

	// Resolve t.co shortlinks to their final URLs before routing.
	// t.co links are Twitter/X link wrappers that HTTP-redirect to the actual URL.
	resolvedURL := d.resolveTcoURL(ctx, urlStr)
	if resolvedURL != urlStr {
		slog.Info("t.co URL resolved", "original", urlStr, "resolved", resolvedURL)
		urlStr = resolvedURL
	}

	if opencli.HasAdapter(urlStr) {
		return d.fetchWithAdapter(ctx, urlStr)
	}

	return d.fetchWebRead(ctx, urlStr)
}

func (d *openCLIDriver) fetchWithAdapter(ctx context.Context, rawURL string) *types.ContentFetchResult {
	subcommand, extraArgs := opencli.CommandForURL(rawURL)

	// Per-adapter driver behavior lives in this switch; the route table only
	// maps domains to adapters. A new domain with special handling = one
	// table row + one case here.
	switch subcommand {
	case opencli.AdapterWeixin:
		// weixin download saves to a local file (not stdout): parse the
		// saved path from YAML output, then read that file as the result.
		return d.fetchWeixinArticle(ctx, rawURL)
	case opencli.AdapterBilibili, opencli.AdapterYoutube:
		// These adapters return metadata only; merge the transcript from a
		// second run so the AI has actual content to summarize.
		return d.dispatchMedia(ctx, subcommand, extraArgs, rawURL)
	case opencli.AdapterZhihu:
		return d.fetchZhihu(ctx, extraArgs, rawURL)
	default:
		result := d.runOpenCLI(ctx, subcommand, extraArgs)
		result.SourceURL = rawURL

		return result
	}
}

// zhihuAnswerRow is one row of `opencli zhihu question -f yaml` (columns:
// rank, id, author, votes, url, content). Content is the adapter's 200-char
// preview — the full body comes from a separate answer-detail fetch.
type zhihuAnswerRow struct {
	Author  string `yaml:"author"`
	URL     string `yaml:"url"`
	Content string `yaml:"content"`
	Rank    int    `yaml:"rank"`
	Votes   int    `yaml:"votes"`
}

// zhihuAnswer is the single row of `opencli zhihu answer-detail -f yaml`.
type zhihuAnswer struct {
	ID            string `yaml:"id"`
	Author        string `yaml:"author"`
	QuestionTitle string `yaml:"question_title"`
	URL           string `yaml:"url"`
	Content       string `yaml:"content"`
	Votes         int    `yaml:"votes"`
}

// fetchZhihu runs a zhihu adapter (routed with -f yaml) and parses it
// structurally. The default md rendering is a markdown table, which mangles a
// long answer body into one cell; question additionally truncates every answer
// to a 200-char preview. Neither shape can carry a full answer.
func (d *openCLIDriver) fetchZhihu(
	ctx context.Context,
	extraArgs []string,
	rawURL string,
) *types.ContentFetchResult {
	result := d.runOpenCLIRaw(ctx, opencli.AdapterZhihu, extraArgs)
	result.SourceURL = rawURL
	if result.Error != "" {
		return result
	}

	switch opencli.SubcommandForURL(rawURL) {
	case opencli.SubcmdZhihuQuestion:
		return d.parseZhihuQuestion(result)
	case opencli.SubcmdZhihuAnswerDetail:
		return d.parseZhihuAnswer(result)
	default:
		return result
	}
}

// parseZhihuQuestion turns the answer list into the thread's synthesis body
// plus the per-answer permalinks the pipeline digests as their own entries.
func (d *openCLIDriver) parseZhihuQuestion(result *types.ContentFetchResult) *types.ContentFetchResult {
	var rows []zhihuAnswerRow
	if err := yaml.Unmarshal([]byte(result.Body), &rows); err != nil {
		return extractFailure(result.SourceURL, fmt.Sprintf("parse zhihu question yaml: %v", err))
	}
	if len(rows) == 0 {
		return extractFailure(result.SourceURL, "zhihu question has no answers")
	}

	result.ChildURLs = make([]string, 0, len(rows))
	for _, r := range rows {
		if u := strings.TrimSpace(r.URL); u != "" {
			result.ChildURLs = append(result.ChildURLs, u)
		}
	}
	result.Body = renderZhihuAnswers(rows)

	slog.Info("zhihu question parsed", "answers", len(rows), "children", len(result.ChildURLs))

	return result
}

// parseZhihuAnswer extracts the full answer body and the entry title.
func (d *openCLIDriver) parseZhihuAnswer(result *types.ContentFetchResult) *types.ContentFetchResult {
	var rows []zhihuAnswer
	if err := yaml.Unmarshal([]byte(result.Body), &rows); err != nil {
		return extractFailure(result.SourceURL, fmt.Sprintf("parse zhihu answer yaml: %v", err))
	}
	if len(rows) == 0 {
		return extractFailure(result.SourceURL, "zhihu answer-detail returned no rows")
	}

	answer := &rows[0]
	body := textutil.TruncateUTF8(strings.TrimSpace(answer.Content), d.maxBodySize*3)
	if body == "" {
		return extractFailure(result.SourceURL, "zhihu answer has empty content")
	}

	result.Title = zhihuAnswerTitle(answer)
	result.Body = body

	slog.Info("zhihu answer parsed", "id", answer.ID, "votes", answer.Votes, "bodyLen", len(body))

	return result
}

// renderZhihuAnswers renders the answer previews as markdown for the
// thread-level synthesis prompt.
func renderZhihuAnswers(rows []zhihuAnswerRow) string {
	var b strings.Builder
	for _, r := range rows {
		fmt.Fprintf(&b, "## 回答 %d（%s，%d 赞同）\n\n%s\n\n", r.Rank, r.Author, r.Votes, strings.TrimSpace(r.Content))
	}

	return strings.TrimSpace(b.String())
}

// zhihuAnswerTitle builds the entry title "{question} - {author}", which is
// what keeps an answer locatable once it is cut out of its thread.
func zhihuAnswerTitle(a *zhihuAnswer) string {
	question := strings.TrimSpace(a.QuestionTitle)
	author := strings.TrimSpace(a.Author)
	switch {
	case question != "" && author != "":
		return question + " - " + author
	case question != "":
		return question
	case author != "":
		return author
	default:
		return strings.TrimSpace(a.URL)
	}
}

// dispatchMedia runs the adapter command and merges a transcript run for
// media adapters that expose only metadata (bilibili, youtube).
func (d *openCLIDriver) dispatchMedia(ctx context.Context, subcommand string, extraArgs []string, rawURL string) *types.ContentFetchResult {
	result := d.runOpenCLI(ctx, subcommand, extraArgs)
	result.SourceURL = rawURL

	if result.Error != "" || result.Body == "" {
		return result
	}
	switch subcommand {
	case opencli.AdapterBilibili:
		d.appendBilibiliTranscript(ctx, result, rawURL)
	case opencli.AdapterYoutube:
		d.appendYoutubeTranscript(ctx, result, rawURL)
	}

	return result
}

// appendBilibiliTranscript runs `opencli bilibili subtitle <bvid>` for the given
// bilibili URL and merges any transcript content into the fetch result body.
// When no subtitles are available, falls back to `bilibili summary <bvid>` for
// B站's official AI-generated summary (with section outlines and timestamps).
func (d *openCLIDriver) appendBilibiliTranscript(ctx context.Context, result *types.ContentFetchResult, rawURL string) {
	subResult := d.runOpenCLI(ctx, "bilibili", []string{"subtitle", rawURL, "--format", "md"})
	if subResult.Error != "" || subResult.Body == "" {
		// Fallback: try B站官方 AI summary when no subtitles are available.
		d.appendBilibiliSummary(ctx, result, rawURL)

		return
	}
	transcript := strings.TrimSpace(subResult.Body)
	if len([]rune(transcript)) < 100 {
		return
	}

	transcriptLines := md.ExtractTranscriptLines(transcript, 3)
	if len(transcriptLines) < 5 {
		return
	}

	transcriptBlock := "\n\n## 字幕内容\n\n" + strings.Join(transcriptLines, "\n")
	result.Body += transcriptBlock
	slog.Info("Bilibili transcript appended", "url", rawURL, "lines", len(transcriptLines))
}

// resolveTcoURL follows the redirect chain for a t.co shortlink using `curl -sL`
// and returns the final resolved URL. Uses curl (not http.Client) because t.co
// redirect chains can pass through multiple hosts (t.co → twitter.com → x.com)
// and curl handles the full chain without downloading response bodies.
// If resolution fails or the URL is not a t.co link, the original URL is returned
// unchanged. Resolved X.com URLs have their trailing media path segments
// (/photo/N, /video/N) stripped to produce a clean status URL.
func (d *openCLIDriver) resolveTcoURL(ctx context.Context, rawURL string) string {
	if !opencli.IsTcoURL(rawURL) {
		return rawURL
	}

	out, err := cmdutil.RunStdout(ctx, "curl", "-sL", "-o", "/dev/null",
		"-w", "%{url_effective}", "--connect-timeout", "10",
		"--max-time", "30", rawURL)
	if err != nil {
		slog.Warn("t.co resolution: curl failed", "url", rawURL, "error", err)

		return rawURL
	}

	finalURL := strings.TrimSpace(string(out))
	if finalURL == "" || finalURL == rawURL {
		return rawURL
	}

	slog.Info("t.co URL resolved", "original", rawURL, "resolved", finalURL)

	// Strip /photo/N and /video/N suffixes from resolved X.com URLs.
	cleaned := opencli.CleanXMediaSuffix(finalURL)
	if cleaned != finalURL {
		slog.Info("t.co resolution: cleaned media suffix",
			"original", finalURL, "cleaned", cleaned)
	}

	return cleaned
}

// appendBilibiliSummary runs `opencli bilibili summary <bvid>` and appends
// B站's official AI-generated summary to the result body. Used as a fallback
// when a video has no subtitles/transcript available.
func (d *openCLIDriver) appendBilibiliSummary(ctx context.Context, result *types.ContentFetchResult, rawURL string) {
	summaryResult := d.runOpenCLI(ctx, "bilibili", []string{"summary", rawURL, "--format", "md"})
	if summaryResult.Error != "" || summaryResult.Body == "" {
		return
	}
	summary := strings.TrimSpace(summaryResult.Body)
	if len([]rune(summary)) < 50 {
		return
	}

	summaryBlock := "\n\n## AI 总结\n\n" + summary
	result.Body += summaryBlock
	slog.Info("Bilibili AI summary appended (no transcript available)",
		"url", rawURL, "len", len([]rune(summary)))
}

func (d *openCLIDriver) fetchWebRead(ctx context.Context, rawURL string) *types.ContentFetchResult {
	r := d.runOpenCLI(ctx, "web", []string{"read", "--url", rawURL, "--stdout"})
	r.SourceURL = rawURL

	return r
}

// fetchWeixinArticle fetches a WeChat public account article via
// `opencli weixin download`, which saves the content to a local file
// under /tmp and outputs metadata + the saved file path on stdout.
// We parse the saved path, read the markdown file, and return it as
// the fetch result body.
func (d *openCLIDriver) fetchWeixinArticle(ctx context.Context, rawURL string) *types.ContentFetchResult {
	// Create a temp directory for weixin downloads.
	tmpDir, err := os.MkdirTemp("", "weixin-*")
	if err != nil {
		return &types.ContentFetchResult{
			Error:       fmt.Sprintf("weixin: create temp dir: %v", err),
			SourceURL:   rawURL,
			FailureKind: types.FailureExtract,
		}
	}
	defer func() {
		if rmErr := os.RemoveAll(tmpDir); rmErr != nil {
			slog.Warn("weixin: failed to remove temp dir", "path", tmpDir, "error", rmErr)
		}
	}()

	// Run weixin download with YAML output format so we can parse the saved path.
	args := []string{"weixin", "download", "--url", rawURL, "-f", "yaml", "--output", tmpDir}

	slog.Info("weixin: downloading article", "url", rawURL)
	stdout, stderr, runErr := cmdutil.RunSeparate(ctx, "opencli", args...)
	if runErr != nil {
		return &types.ContentFetchResult{
			Error:       fmt.Sprintf("weixin: %v (stderr: %s)", runErr, strings.TrimSpace(string(stderr))),
			SourceURL:   rawURL,
			FailureKind: types.FailureFetch,
		}
	}

	// Parse the YAML output to find the saved file path.
	// YAML format:
	//   - title: ...
	//     ...
	//     saved: /tmp/weixin-xxx/红筹退潮.../红筹退潮....md
	savedPath := extractWeixinSavedPath(string(stdout))
	if savedPath == "" {
		return &types.ContentFetchResult{
			Error:       "weixin: could not find saved file path in output",
			SourceURL:   rawURL,
			FailureKind: types.FailureExtract,
		}
	}

	// Read the saved markdown file content.
	content, err := os.ReadFile(savedPath)
	if err != nil {
		return &types.ContentFetchResult{
			Error:       fmt.Sprintf("weixin: read saved file: %v", err),
			SourceURL:   rawURL,
			FailureKind: types.FailureExtract,
		}
	}

	body := string(content)
	body = textutil.TruncateUTF8(body, d.maxBodySize*3)
	title := md.ExtractTitleFromMarkdown(body)

	slog.Info("weixin: article fetched successfully",
		"url", rawURL, "title", title, "size", len(body))

	return &types.ContentFetchResult{Title: title, Body: body, SourceURL: rawURL}
}

// extractWeixinSavedPath parses the YAML output from `weixin download -f yaml`
// and returns the value of the `saved:` field (the absolute path to the saved
// markdown file).
func extractWeixinSavedPath(yamlOutput string) string {
	var entries []weixinDownloadEntry
	if err := yaml.Unmarshal([]byte(yamlOutput), &entries); err == nil {
		if path := firstSavedPath(entries); path != "" {
			return path
		}
	}

	var entry weixinDownloadEntry
	if err := yaml.Unmarshal([]byte(yamlOutput), &entry); err == nil {
		return strings.TrimSpace(entry.Saved)
	}

	return ""
}

type weixinDownloadEntry struct {
	Saved string `yaml:"saved"`
}

func firstSavedPath(entries []weixinDownloadEntry) string {
	for _, entry := range entries {
		if path := strings.TrimSpace(entry.Saved); path != "" {
			return path
		}
	}

	return ""
}

// navRetryAttempts is the number of retries after a browser-bridge refusal.
const navRetryAttempts = 2

// navRetryBackoff is the base delay between navigation retries; tests shrink it.
var navRetryBackoff = 2 * time.Second

// runOpenCLI runs an opencli subcommand, retries transient browser-bridge
// refusals, and caps the markdown body at the body-size limit.
func (d *openCLIDriver) runOpenCLI(ctx context.Context, subcommand string, extraArgs []string) *types.ContentFetchResult {
	result := d.runOpenCLIRaw(ctx, subcommand, extraArgs)
	if result.Error != "" {
		return result
	}

	result.Body = textutil.TruncateUTF8(result.Body, d.maxBodySize*3)
	result.Title = md.ExtractTitleFromMarkdown(result.Body)

	slog.Info("opencli fetch succeeded", "subcommand", subcommand, "bodyLen", len(result.Body))

	return result
}

// runOpenCLIRaw runs an opencli subcommand and returns stdout untruncated.
// Structured callers use it because a truncated document is not parseable;
// they cap each field after parsing instead.
//
// opencli drives a single shared browser tab, so a batch of concurrent fetches
// makes Chrome reject some navigations ("Navigation rejected." from the tabs
// API). Retrying keeps a collision from being recorded as a permanent fetch
// failure for a perfectly good URL.
func (d *openCLIDriver) runOpenCLIRaw(ctx context.Context, subcommand string, extraArgs []string) *types.ContentFetchResult {
	var last *types.ContentFetchResult
	err := retry.Do(
		func() error {
			r := d.runOpenCLIOnce(ctx, subcommand, extraArgs)
			last = r
			if isNavigationRejected(r.Error) {
				return fmt.Errorf("%s", r.Error)
			}

			return nil
		},
		retry.Attempts(navRetryAttempts+1),
		retry.Delay(navRetryBackoff),
		retry.DelayType(retry.BackOffDelay),
		retry.Context(ctx),
		retry.LastErrorOnly(true),
	)
	if err != nil {
		slog.Warn("opencli navigation rejected, final retry", "subcommand", subcommand, "url", strings.Join(extraArgs, " "))
		if last == nil {
			last = &types.ContentFetchResult{Error: err.Error()}
		}
	}

	return last
}

// isNavigationRejected reports whether opencli failed because the browser
// refused to navigate the automation tab, which is transient.
func isNavigationRejected(errText string) bool {
	return strings.Contains(errText, "Navigation rejected")
}

func (d *openCLIDriver) runOpenCLIOnce(
	ctx context.Context,
	subcommand string,
	extraArgs []string,
) *types.ContentFetchResult {
	args := append([]string{subcommand}, extraArgs...)
	stdout, stderr, err := cmdutil.RunSeparate(ctx, "opencli", args...)
	if err != nil {
		return &types.ContentFetchResult{
			Error: fmt.Sprintf("opencli: %v (stderr: %s)", err, strings.TrimSpace(string(stderr))),
		}
	}

	body := string(stdout)
	if body == "" {
		return &types.ContentFetchResult{Error: "opencli returned empty content"}
	}

	return &types.ContentFetchResult{Body: body}
}

// appendYoutubeTranscript runs `opencli youtube transcript <url>` and merges
// transcript text into the fetch result body.
func (d *openCLIDriver) appendYoutubeTranscript(ctx context.Context, result *types.ContentFetchResult, rawURL string) {
	subResult := d.runOpenCLI(ctx, "youtube", []string{"transcript", rawURL, "--format", "md"})
	if subResult.Error != "" || subResult.Body == "" {
		return
	}
	transcript := strings.TrimSpace(subResult.Body)
	if len([]rune(transcript)) < 100 {
		return
	}

	// Parse the YouTube transcript table (| timestamp | speaker | text |) with goldmark.
	transcriptLines := md.ExtractTranscriptLines(transcript, 2)

	if len(transcriptLines) < 5 {
		return
	}

	transcriptBlock := "\n\n## 字幕内容\n\n" + strings.Join(transcriptLines, "\n")
	result.Body += transcriptBlock
	slog.Info("YouTube transcript appended", "url", rawURL, "lines", len(transcriptLines))
}
