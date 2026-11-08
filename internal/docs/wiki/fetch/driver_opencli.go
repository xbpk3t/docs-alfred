package fetch

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/avast/retry-go/v4"
	yaml "github.com/goccy/go-yaml"
	"github.com/xbpk3t/docs-alfred/internal/docs/wiki/types"
	"github.com/xbpk3t/docs-alfred/pkg/cmdutil"
	"github.com/xbpk3t/docs-alfred/pkg/fileutil"
	"github.com/xbpk3t/docs-alfred/pkg/md"
	"github.com/xbpk3t/docs-alfred/pkg/opencli"
	"github.com/xbpk3t/docs-alfred/pkg/textutil"
	"github.com/xbpk3t/docs-alfred/pkg/urlutil"
)

// openCLIDriver fetches content via the opencli CLI tool.
// Site-specific adapters (youtube/bilibili/twitter...) extract structured content;
// generic URLs fall back to opencli web read.
//
// opencli drives one shared browser tab, so every invocation goes through
// browserSem: running wiki.concurrency opencli processes at once makes the
// browser tabs API reject navigations ("Navigation rejected."), drop the
// connection, or detach the target. The semaphore keeps some overlap without
// colliding on the tab.
type openCLIDriver struct {
	browserSem   chan struct{}
	maxBodySize  int
	mediaEnabled bool
}

// openCLIEnvelope is the opencli 2.x wire format: successful JSON rows are
// wrapped as {"ok":true,"data":[...]}; failures are {"ok":false,"error":{...}}.
// opencli 1.x printed the bare array and wrote errors to stderr.
type openCLIEnvelope[T any] struct {
	Data []T  `json:"data"`
	OK   bool `json:"ok"`
}

type openCLIErrorEnvelope struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
	OK bool `json:"ok"`
}

// decodeOpenCLIRows accepts both the 1.x bare array and the 2.x envelope.
func decodeOpenCLIRows[T any](body string) ([]T, bool) {
	var rows []T
	if err := json.Unmarshal([]byte(body), &rows); err == nil {
		return rows, true
	}
	var env openCLIEnvelope[T]
	if err := json.Unmarshal([]byte(body), &env); err == nil && env.OK {
		return env.Data, true
	}

	return nil, false
}

// openCLIOutputDetail returns a bounded, human-readable description of the
// command output for logs/errors. opencli 2.x puts its error envelope on stdout.
func openCLIOutputDetail(stdout, stderr []byte) string {
	var parts []string
	if s := strings.TrimSpace(string(stdout)); s != "" {
		parts = append(parts, "stdout: "+textutil.TruncateUTF8(s, 2000))
	}
	if s := strings.TrimSpace(string(stderr)); s != "" {
		parts = append(parts, "stderr: "+textutil.TruncateUTF8(s, 2000))
	}

	return strings.Join(parts, "\n")
}

// openCLIExitCode recovers the exit code cmdutil attached to the error. It
// returns 0 when unknown.
func openCLIExitCode(err error) int {
	var exitErr *cmdutil.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.Code
	}

	return 0
}

// openCLIFailureReason classifies an opencli failure using the 2.x error
// envelope, then the exit code, then the raw text. This keeps retry policy
// independent of which stream the tool used.
func openCLIFailureReason(stdout, stderr []byte, err error) types.FailureReason {
	if env, ok := parseOpenCLIErrorEnvelope(stdout); ok {
		return openCLIEnvelopeReason(env)
	}

	switch openCLIExitCode(err) {
	case 66:
		return types.FailureReasonEmpty
	case 2:
		return types.FailureReasonNotFound
	case 77:
		return types.FailureReasonAuthRequired
	case 69, 75:
		return types.FailureReasonHTTPError
	}

	// Text fallback for older builds or errors that bypass the envelope.
	text := strings.ToLower(string(stdout) + "\n" + string(stderr) + "\n" + err.Error())
	if reason, ok := deterministicAPIReason(text); ok {
		return reason
	}
	switch {
	case strings.Contains(text, "empty_result"):
		return types.FailureReasonEmpty
	case strings.Contains(text, "auth_required") || strings.Contains(text, "noperm"):
		return types.FailureReasonAuthRequired
	default:
		return types.FailureReasonHTTPError
	}
}

// deterministicAPIReason maps provider codes whose outcome cannot improve on
// retry. Bilibili 62002 is a removed/not-visible video; 62012 is paywalled.
func deterministicAPIReason(lower string) (types.FailureReason, bool) {
	switch {
	case strings.Contains(lower, "62002"), strings.Contains(lower, "稿件不可见"):
		return types.FailureReasonNotFound, true
	case strings.Contains(lower, "62012"), strings.Contains(lower, "需要充电"), strings.Contains(lower, "充电专属"):
		return types.FailureReasonPaidContent, true
	default:
		return "", false
	}
}

// openCLIEnvelopeReason classifies a parsed 2.x error envelope by code, with a
// message fallback for codes the adapter embeds in prose.
func openCLIEnvelopeReason(env openCLIErrorEnvelope) types.FailureReason {
	message := strings.ToLower(env.Error.Message)
	if reason, ok := deterministicAPIReason(message); ok {
		return reason
	}
	if strings.Contains(message, "404") || strings.Contains(message, "410") ||
		strings.Contains(message, "not found") {
		return types.FailureReasonNotFound
	}
	switch strings.ToUpper(strings.TrimSpace(env.Error.Code)) {
	case "EMPTY_RESULT":
		return types.FailureReasonEmpty
	case "ARGUMENT", "USAGE_ERROR", "USAGE":
		return types.FailureReasonNotFound
	case "AUTH_REQUIRED", "NOPERM", "PERMISSION":
		return types.FailureReasonAuthRequired
	case "BROWSER_CONNECT":
		return types.FailureReasonBrowserError
	default:
		return types.FailureReasonHTTPError
	}
}

// savedFileFromAttempt reports whether path was written during the most recent
// opencli attempt. A retry can reuse the same --output directory; without this
// check a partial file from a failed attempt can be mistaken for this attempt's
// output.
func savedFileFromAttempt(path string, attemptStart time.Time) bool {
	if attemptStart.IsZero() {
		return true
	}
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	// Tolerate coarse filesystem timestamp resolution. Retries are separated by
	// navRetryBackoff (2s), so a stale file is still rejected.
	return !info.ModTime().Before(attemptStart.Add(-500 * time.Millisecond))
}

// defaultBrowserConcurrency is the number of concurrent opencli processes the
// driver allows. opencli serializes browser access internally per process, so
// 2 overlaps I/O waits while staying under the tab-lease collision threshold.
const defaultBrowserConcurrency = 2

func newOpenCLIDriver(opts DriverOptions) *openCLIDriver {
	maxBody := opts.MaxBodySize
	if maxBody <= 0 {
		maxBody = 5000
	}
	concurrency := opts.BrowserConcurrency
	if concurrency <= 0 {
		concurrency = defaultBrowserConcurrency
	}
	// opencli's browser command deadline defaults to 60s, shorter than the
	// wiki per-URL timeout. Slow bilibili/youtube pages then fail as TIMEOUT
	// even though the page eventually loads. Give the subprocess headroom;
	// the caller's context still enforces the real per-URL deadline.
	if os.Getenv("OPENCLI_BROWSER_COMMAND_TIMEOUT") == "" {
		if err := os.Setenv("OPENCLI_BROWSER_COMMAND_TIMEOUT", "180"); err != nil {
			slog.Warn("failed to set opencli browser timeout", "error", err)
		}
	}

	return &openCLIDriver{
		maxBodySize:  maxBody,
		mediaEnabled: opts.MediaEnabled,
		browserSem:   make(chan struct{}, concurrency),
	}
}

// acquireBrowser blocks until a browser slot is free or ctx is canceled.
func (d *openCLIDriver) acquireBrowser(ctx context.Context) error {
	select {
	case d.browserSem <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (d *openCLIDriver) releaseBrowser() {
	<-d.browserSem
}

func (d *openCLIDriver) Name() string { return "opencli" }

// UsesHTTPFallback lets the ingest fetcher prefer the static HTTP driver for
// generic non-adapter pages, keeping the shared browser tab for adapters.
func (d *openCLIDriver) UsesHTTPFallback() bool { return true }

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
	case opencli.AdapterTwitter:
		return d.fetchTwitterThread(ctx, extraArgs, rawURL)
	default:
		if subcommand == opencli.AdapterWeb {
			// Web-read routes need the same settle time as the generic path.
			return d.fetchWebRead(ctx, rawURL)
		}
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

// twitterThreadRow is one row of `opencli twitter thread -f json`. The first
// row is the root tweet in structural order; replies follow.
type twitterCard struct {
	URL         string `json:"url"`
	Title       string `json:"title"`
	Description string `json:"description"`
}

type twitterThreadRow struct {
	Card     *twitterCard `json:"card"`
	ID       string       `json:"id"`
	Author   string       `json:"author"`
	Text     string       `json:"text"`
	URL      string       `json:"url"`
	Likes    int          `json:"likes"`
	Retweets int          `json:"retweets"`
}

// fetchTwitterThread parses the thread rows instead of the md table. Some X
// posts only contain a link; the thread's replies are not the article, so a
// root tweet that is essentially one URL is replaced by the linked page.
func (d *openCLIDriver) fetchTwitterThread(
	ctx context.Context,
	extraArgs []string,
	rawURL string,
) *types.ContentFetchResult {
	result := d.runOpenCLIRaw(ctx, opencli.AdapterTwitter, extraArgs)
	result.SourceURL = rawURL
	if result.Error != "" {
		return result
	}

	rows, ok := decodeOpenCLIRows[twitterThreadRow](result.Body)
	if !ok {
		// Keep compatibility with older opencli builds that rendered markdown.
		result.Body = textutil.TruncateUTF8(result.Body, d.maxBodySize*3)
		result.Title = md.ExtractTitleFromMarkdown(result.Body)

		return result
	}
	if len(rows) == 0 {
		// opencli returns ok:true with an empty array for a tweet that no
		// longer exists (deleted/protected); retrying cannot recover it.
		return extractFailureWithReason(rawURL, "twitter thread returned no rows", types.FailureReasonNotFound)
	}

	root := twitterRootRow(rows, rawURL)
	if root == nil || strings.TrimSpace(root.Text) == "" {
		return extractFailureWithReason(rawURL, "twitter root tweet body missing", types.FailureReasonPartialExtract)
	}

	target := twitterLinkPostTarget(root.Text)
	if target == "" && root.Card != nil {
		// The link often lives in the card rather than the tweet text (X hides
		// the URL behind a preview). Follow the card target when it is external.
		target = strings.TrimSpace(root.Card.URL)
	}
	if target != "" {
		return d.fetchTwitterLinkPost(ctx, root, target, rawURL)
	}

	var body strings.Builder
	for _, row := range rows {
		fmt.Fprintf(&body, "## @%s\n\n%s\n\n", row.Author, strings.TrimSpace(row.Text))
	}

	title := md.ExtractTitleFromMarkdown(body.String())
	if title == "" || strings.HasPrefix(title, "@") {
		title = "X thread by @" + root.Author
	}

	return &types.ContentFetchResult{
		Title:     title,
		Body:      textutil.TruncateUTF8(body.String(), d.maxBodySize*3),
		SourceURL: rawURL,
	}
}

// fetchTwitterLinkPost follows a link-only tweet to its target page and renders
// the page body with the original tweet quoted beneath it.
func (d *openCLIDriver) fetchTwitterLinkPost(
	ctx context.Context,
	root *twitterThreadRow,
	target, rawURL string,
) *types.ContentFetchResult {
	resolved := d.resolveTcoURL(ctx, target)
	if isXStatusURL(resolved) {
		return extractFailureWithReason(rawURL, "twitter root tweet is a link-only post", types.FailureReasonLinkOnlyPost)
	}

	followed := d.fetchWebRead(ctx, resolved)
	if followed.Error != "" || strings.TrimSpace(followed.Body) == "" {
		return extractFailureWithReason(
			rawURL,
			"x link post target fetch failed: "+followed.Error,
			types.FailureReasonPartialExtract,
		)
	}

	title := strings.TrimSpace(followed.Title)
	if title == "" {
		title = md.ExtractTitleFromMarkdown(followed.Body)
	}
	if title == "" {
		title = "X link post by @" + root.Author
	}
	body := fmt.Sprintf("# %s\n\n%s\n\n---\n\n原推文（@%s）：\n\n%s\n\n链接：%s\n",
		title, followed.Body, root.Author, root.Text, resolved)

	slog.Info("x link post followed", "tweet", rawURL, "target", resolved)

	return &types.ContentFetchResult{
		Title:     title,
		Body:      textutil.TruncateUTF8(body, d.maxBodySize*3),
		SourceURL: rawURL,
	}
}

// twitterRootRow finds the tweet whose id matches the URL's /status/<id>.
func twitterRootRow(rows []twitterThreadRow, rawURL string) *twitterThreadRow {
	parsed, err := url.Parse(rawURL)
	if err == nil {
		parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
		for i := 0; i+1 < len(parts); i++ {
			if parts[i] != "status" {
				continue
			}
			id := parts[i+1]
			for j := range rows {
				if rows[j].ID == id || strings.HasSuffix(rows[j].URL, "/status/"+id) {
					return &rows[j]
				}
			}
		}
	}
	if len(rows) == 0 {
		return nil
	}

	return &rows[0]
}

// twitterLinkPostTarget returns the single external URL when the tweet text is
// essentially just that link. It returns "" for tweets with real commentary.
func twitterLinkPostTarget(text string) string {
	refs := urlutil.ExtractURLRefs(text, urlutil.ExtractOptions{
		BareURLs:    true,
		HTTPOnly:    true,
		Normalize:   true,
		Deduplicate: true,
	})
	if len(refs) == 0 {
		return ""
	}

	remaining := text
	for i := len(refs) - 1; i >= 0; i-- {
		if refs[i].Start < 0 || refs[i].End > len(remaining) || refs[i].Start >= refs[i].End {
			continue
		}
		remaining = remaining[:refs[i].Start] + " " + remaining[refs[i].End:]
	}
	if len([]rune(strings.TrimSpace(remaining))) > 120 {
		return ""
	}

	return refs[0].URL
}

func isXStatusURL(rawURL string) bool {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	host := strings.ToLower(parsed.Hostname())
	if host != "x.com" && host != "twitter.com" && host != "mobile.twitter.com" {
		return false
	}

	return strings.Contains(parsed.Path, "/status/")
}

// zhihuDownloadEntry is one row of `opencli zhihu download -f json`.
// status is an extraction status ("success", "failed — no title", ...), not an
// HTTP status; saved is the absolute path of the markdown file.
type zhihuDownloadEntry struct {
	Title       string `json:"title"`
	Author      string `json:"author"`
	PublishTime string `json:"publish_time"`
	Status      string `json:"status"`
	Size        string `json:"size"`
	Saved       string `json:"saved"`
}

// fetchZhihuArticle runs `zhihu download`, which uses the zhihu site session
// and writes clean markdown to disk. The generic web reader uses the `web`
// site session and gets zhihu's 安全验证 page instead.
func (d *openCLIDriver) fetchZhihuArticle(ctx context.Context, rawURL string) *types.ContentFetchResult {
	tmpDir, err := os.MkdirTemp("", "docs-zhihu-download-*")
	if err != nil {
		return extractFailureWithReason(rawURL, "create zhihu download dir: "+err.Error(), types.FailureReasonEmpty)
	}
	defer func() {
		if rmErr := os.RemoveAll(tmpDir); rmErr != nil {
			slog.Warn("zhihu download: failed to remove temp dir", "path", tmpDir, "error", rmErr)
		}
	}()

	args := []string{
		opencli.SubcmdZhihuDownload,
		"--url", rawURL,
		"--output", tmpDir,
		"--format", "json",
	}
	var lastAttemptStart time.Time
	result := d.runOpenCLIRawWithHook(ctx, opencli.AdapterZhihu, args, func(_ int, started time.Time) {
		lastAttemptStart = started
	})
	result.SourceURL = rawURL
	if result.Error != "" {
		return result
	}

	rows, ok := decodeOpenCLIRows[zhihuDownloadEntry](result.Body)
	if !ok {
		// Older opencli builds answered in markdown; keep that shape usable.
		result.Body = textutil.TruncateUTF8(result.Body, d.maxBodySize*3)
		result.Title = md.ExtractTitleFromMarkdown(result.Body)

		return result
	}
	if len(rows) == 0 {
		return extractFailureWithReason(rawURL, "zhihu download returned no rows", types.FailureReasonEmpty)
	}

	return d.zhihuResultFromRow(rawURL, &rows[0], lastAttemptStart, tmpDir)
}

// zhihuResultFromRow validates the download row's status and saved file, then
// builds the article result from the saved markdown.
func (d *openCLIDriver) zhihuResultFromRow(
	rawURL string,
	row *zhihuDownloadEntry,
	attemptStart time.Time,
	tmpDir string,
) *types.ContentFetchResult {
	if status := strings.ToLower(strings.TrimSpace(row.Status)); status != "" && status != "success" {
		reason := types.FailureReasonPartialExtract
		if strings.Contains(status, "no title") || strings.Contains(status, "no content") {
			reason = types.FailureReasonEmpty
		}

		return extractFailureWithReason(rawURL, "zhihu download "+row.Status, reason)
	}

	return d.downloadResultFromSavedFile(rawURL, "zhihu download", row.Saved, tmpDir, attemptStart, strings.TrimSpace(row.Title))
}

// downloadResultFromSavedFile is the shared tail of the download-style
// adapters (zhihu, weixin): validate that the saved file came from this
// attempt, read it, and build the article result. Per-adapter status mapping
// stays in the callers; label prefixes the failure messages.
func (d *openCLIDriver) downloadResultFromSavedFile(
	rawURL, label, saved, tmpDir string,
	attemptStart time.Time,
	title string,
) *types.ContentFetchResult {
	saved = strings.TrimSpace(saved)
	if saved == "" || saved == "-" || !fileutil.PathWithin(tmpDir, saved) {
		return extractFailureWithReason(
			rawURL,
			label+" did not save a usable file (saved="+saved+")",
			types.FailureReasonPartialExtract,
		)
	}
	if !savedFileFromAttempt(saved, attemptStart) {
		return extractFailureWithReason(rawURL, label+" file is from an earlier failed attempt", types.FailureReasonPartialExtract)
	}

	content, err := os.ReadFile(saved) // #nosec G304 -- path is constrained to tmpDir.
	if err != nil {
		return extractFailureWithReason(rawURL, label+" read saved file: "+err.Error(), types.FailureReasonPartialExtract)
	}
	body := strings.TrimSpace(string(content))
	if body == "" {
		return extractFailureWithReason(rawURL, label+" saved file is empty", types.FailureReasonEmpty)
	}

	body = textutil.TruncateUTF8(body, d.maxBodySize*3)
	if title == "" || title == "安全验证" {
		title = md.ExtractTitleFromMarkdown(body)
	}

	slog.Info("download adapter article fetched", "adapter", label, "url", rawURL, "title", title, "bodyLen", len(body))

	return &types.ContentFetchResult{Title: title, Body: body, SourceURL: rawURL}
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
	subcommand := opencli.SubcommandForURL(rawURL)
	if subcommand == opencli.SubcmdZhihuDownload {
		return d.fetchZhihuArticle(ctx, rawURL)
	}

	result := d.runOpenCLIRaw(ctx, opencli.AdapterZhihu, extraArgs)
	result.SourceURL = rawURL
	if result.Error != "" {
		return result
	}

	switch subcommand {
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
		if failure := d.appendBilibiliTranscript(ctx, result, rawURL); failure != nil {
			return failure
		}
	case opencli.AdapterYoutube:
		if failure := d.appendYoutubeTranscript(ctx, result, rawURL); failure != nil {
			return failure
		}
	}

	return result
}

// appendBilibiliTranscript runs `opencli bilibili subtitle <bvid>` for the given
// bilibili URL and merges any transcript content into the fetch result body.
// When no subtitles are available, falls back to `bilibili summary <bvid>` for
// B站's official AI-generated summary (with section outlines and timestamps).
// It returns a failure result only when the summary service itself failed
// transiently; a video that simply has no summary is left to the video
// too-short gate.
func (d *openCLIDriver) appendBilibiliTranscript(
	ctx context.Context,
	result *types.ContentFetchResult,
	rawURL string,
) *types.ContentFetchResult {
	subResult := d.runOpenCLI(ctx, "bilibili", []string{"subtitle", rawURL, "--format", "md"})
	if subResult.Error != "" || subResult.Body == "" {
		// Fallback: try B站官方 AI summary when no subtitles are available.
		return d.appendBilibiliSummary(ctx, result, rawURL)
	}
	transcript := strings.TrimSpace(subResult.Body)
	if len([]rune(transcript)) < 100 {
		return d.appendBilibiliSummary(ctx, result, rawURL)
	}

	transcriptLines := md.ExtractTranscriptLines(transcript, 3)
	if len(transcriptLines) < 5 {
		return d.appendBilibiliSummary(ctx, result, rawURL)
	}

	transcriptBlock := "\n\n" + types.TranscriptSectionMarker + "\n\n" + strings.Join(transcriptLines, "\n")
	result.Body += transcriptBlock
	result.HasTranscript = true
	slog.Info("Bilibili transcript appended", "url", rawURL, "lines", len(transcriptLines))

	return nil
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
// when a video has no subtitles/transcript available. A missing summary
// (EMPTY_RESULT) is not a failure — the metadata-only result later trips the
// video too-short gate. A transient summary API error is returned so ingest
// keeps the URL retryable instead of recording a permanent too-short failure.
func (d *openCLIDriver) appendBilibiliSummary(
	ctx context.Context,
	result *types.ContentFetchResult,
	rawURL string,
) *types.ContentFetchResult {
	summaryResult := d.runOpenCLI(ctx, "bilibili", []string{"summary", rawURL, "--format", "md"})
	if summaryResult.Error != "" || summaryResult.Body == "" {
		if isBilibiliNoSummary(summaryResult.Error) {
			return nil
		}

		reason := summaryResult.FailureReason
		switch reason {
		case types.FailureReasonAuthRequired:
			// Bilibili's summary auth failure means the video is paid/charging
			// for this account; retrying without payment cannot recover it.
			reason = types.FailureReasonPaidContent
		case types.FailureReasonPaidContent, types.FailureReasonNotFound:
			// Keep deterministic causes visible.
		default:
			// A generic summary API failure remains a retryable partial extract.
			reason = types.FailureReasonPartialExtract
		}

		return extractFailureWithReason(
			rawURL,
			"bilibili summary unavailable: "+summaryResult.Error,
			reason,
		)
	}
	summary := strings.TrimSpace(summaryResult.Body)
	if len([]rune(summary)) < 50 {
		return nil
	}

	summaryBlock := "\n\n## AI 总结\n\n" + summary
	result.Body += summaryBlock
	result.HasTranscript = true
	slog.Info("Bilibili AI summary appended (no transcript available)",
		"url", rawURL, "len", len([]rune(summary)))

	return nil
}

// isBilibiliNoSummary reports the "video has no official AI summary" shape,
// which is permanent content absence rather than a service failure.
func isBilibiliNoSummary(errText string) bool {
	return textutil.ContainsAny(errText, []string{
		"EMPTY_RESULT",
		"has not generated an AI summary",
		"summary returned no data",
	})
}

func (d *openCLIDriver) fetchWebRead(ctx context.Context, rawURL string) *types.ContentFetchResult {
	// Give client-rendered pages a little longer to settle: the default 3s
	// occasionally yields a null DOM (cloneNode TypeError) on heavy SPAs.
	r := d.runOpenCLI(ctx, "web", []string{"read", "--url", rawURL, "--stdout", "--wait", "5"})
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

	// Run weixin download with YAML output format so we can parse title,
	// status and saved path. It shares the opencli browser tab.
	args := []string{"weixin", "download", "--url", rawURL, "-f", "yaml", "--output", tmpDir}

	slog.Info("weixin: downloading article", "url", rawURL)
	var lastAttemptStart time.Time
	stdout, stderr, runErr := d.execOpenCLI(ctx, args, func(_ int, started time.Time) {
		lastAttemptStart = started
	})
	if runErr != nil {
		detail := fmt.Sprintf("weixin: %v", runErr)
		if out := openCLIOutputDetail(stdout, stderr); out != "" {
			detail += "\n" + out
		}

		return &types.ContentFetchResult{
			Error:         detail,
			SourceURL:     rawURL,
			FailureKind:   types.FailureFetch,
			FailureReason: openCLIFailureReason(stdout, stderr, runErr),
		}
	}

	entry, ok := parseWeixinDownloadEntry(string(stdout))
	if !ok {
		// Older opencli builds printed the article body directly. Preserve
		// that shape, but do not mistake an error envelope or an unparsable
		// metadata row for content.
		body := strings.TrimSpace(string(stdout))
		if body == "" {
			return extractFailureWithReason(rawURL, "weixin: empty download output", types.FailureReasonEmpty)
		}
		if strings.HasPrefix(body, "ok: false") || looksLikeWeixinYAMLRow(body) {
			return extractFailureWithReason(
				rawURL,
				"weixin: unparsable download output: "+body,
				types.FailureReasonPartialExtract,
			)
		}

		return &types.ContentFetchResult{
			Title:     md.ExtractTitleFromMarkdown(body),
			Body:      textutil.TruncateUTF8(body, d.maxBodySize*3),
			SourceURL: rawURL,
		}
	}

	return d.weixinResultFromEntry(rawURL, &entry, lastAttemptStart, tmpDir)
}

// weixinResultFromEntry validates the download entry's status and saved file,
// then builds the article result from the saved markdown. Status is reported
// separately from HTTP status: "failed — verification required in WeChat
// browser page" is an anti-bot wall, retryable, so the inbox line survives.
func (d *openCLIDriver) weixinResultFromEntry(
	rawURL string,
	entry *weixinDownloadEntry,
	attemptStart time.Time,
	tmpDir string,
) *types.ContentFetchResult {
	switch status := strings.ToLower(strings.TrimSpace(entry.Status)); status {
	case "", "success":
	case "invalid url":
		return extractFailureWithReason(rawURL, "weixin: not a wechat article url", types.FailureReasonNotFound)
	default:
		// "no title"/"no content" are permanent extraction outcomes, not an
		// anti-bot wall; retrying them would keep the inbox line forever.
		reason := types.FailureReasonAntiBot
		if strings.Contains(status, "no title") || strings.Contains(status, "no content") {
			reason = types.FailureReasonEmpty
		}

		return extractFailureWithReason(rawURL, "weixin: download "+entry.Status, reason)
	}

	return d.downloadResultFromSavedFile(rawURL, "weixin: download", entry.Saved, tmpDir, attemptStart, strings.TrimSpace(entry.Title))
}

// parseWeixinDownloadEntry parses the YAML output from `weixin download -f yaml`.
// It accepts both the sequence form and a single mapping.
func parseWeixinDownloadEntry(yamlOutput string) (weixinDownloadEntry, bool) {
	var entries []weixinDownloadEntry
	if err := yaml.Unmarshal([]byte(yamlOutput), &entries); err == nil && len(entries) > 0 {
		return entries[0], true
	}

	var entry weixinDownloadEntry
	if err := yaml.Unmarshal([]byte(yamlOutput), &entry); err == nil &&
		(entry.Status != "" || entry.Saved != "" || entry.Title != "") {
		return entry, true
	}

	return weixinDownloadEntry{}, false
}

type weixinDownloadEntry struct {
	Title  string `yaml:"title"`
	Status string `yaml:"status"`
	Saved  string `yaml:"saved"`
}

// looksLikeWeixinYAMLRow reports output that was meant to be a download
// metadata row but did not parse as YAML (for example the invalid plain
// scalar `saved: -`), so it is treated as a failure instead of article body.
func looksLikeWeixinYAMLRow(body string) bool {
	return strings.HasPrefix(body, "- title:") || strings.HasPrefix(body, "title:") ||
		strings.Contains(body, "\nstatus:") || strings.Contains(body, "\nsaved:")
}

// navRetryAttempts is the number of retries after a transient browser-bridge
// refusal (Navigation rejected, dropped connection, detached target).
const navRetryAttempts = 3

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
	return d.runOpenCLIRawWithHook(ctx, subcommand, extraArgs, nil)
}

// openCLIAttemptHook observes each retry attempt. Commands that write files
// under --output use it to reject a file left behind by an earlier, failed
// attempt.
type openCLIAttemptHook func(attempt int, started time.Time)

func (d *openCLIDriver) runOpenCLIRawWithHook(
	ctx context.Context,
	subcommand string,
	extraArgs []string,
	hook openCLIAttemptHook,
) *types.ContentFetchResult {
	args := append([]string{subcommand}, extraArgs...)
	stdout, stderr, err := d.execOpenCLI(ctx, args, hook)
	if err != nil {
		slog.Warn("opencli failure", "subcommand", subcommand, "url", strings.Join(extraArgs, " "))

		detail := "opencli: " + err.Error()
		if out := openCLIOutputDetail(stdout, stderr); out != "" {
			detail += "\n" + out
		}

		return &types.ContentFetchResult{
			Error:         detail,
			FailureKind:   types.FailureFetch,
			FailureReason: openCLIFailureReason(stdout, stderr, err),
		}
	}

	body := string(stdout)
	if body == "" {
		return &types.ContentFetchResult{Error: "opencli returned empty content"}
	}

	return &types.ContentFetchResult{Body: body}
}

// execOpenCLI runs an opencli command under the browser semaphore and retries
// transient browser-bridge failures. It returns raw stdout/stderr so callers
// that parse structured output decide how to treat empty or unparsable bodies.
func (d *openCLIDriver) execOpenCLI(
	ctx context.Context,
	args []string,
	hook openCLIAttemptHook,
) (stdout, stderr []byte, runErr error) {
	attempt := 0
	retryErr := retry.Do(
		func() error {
			attempt++
			if hook != nil {
				hook(attempt, time.Now())
			}
			if err := d.acquireBrowser(ctx); err != nil {
				return err
			}
			defer d.releaseBrowser()
			var err error
			stdout, stderr, err = cmdutil.RunSeparate(ctx, "opencli", args...)
			runErr = err
			if err != nil && (isTransientOpenCLIError(err.Error()+"\n"+openCLIOutputDetail(stdout, stderr)) ||
				isTransientOpenCLIExitCode(err)) {
				return err
			}

			return nil
		},
		retry.Attempts(navRetryAttempts+1),
		retry.Delay(navRetryBackoff),
		retry.DelayType(retry.BackOffDelay),
		retry.Context(ctx),
		retry.LastErrorOnly(true),
	)
	if retryErr != nil {
		runErr = retryErr
	}

	return stdout, stderr, runErr
}

// isTransientOpenCLIExitCode reports opencli's TEMPFAIL/SERVICE_UNAVAIL exit
// codes, which are retryable even when the envelope is unavailable.
func isTransientOpenCLIExitCode(err error) bool {
	switch openCLIExitCode(err) {
	case 69, 75:
		return true
	default:
		return false
	}
}

// isTransientOpenCLIError reports whether an opencli failure is the shared
// browser tab refusing or dropping the command, which a retry can clear.
func isTransientOpenCLIError(errText string) bool {
	return textutil.ContainsAny(errText, []string{
		"Navigation rejected",
		"Cannot attach to this target",
		"Browser connection dropped",
		"stale page identity",
		"TypeError: Failed to fetch",
		"fetchJsonOrLoginWall",
		"cloneNode",
		"Cannot read properties of null",
		"TEMPFAIL",
		"SERVICE_UNAVAIL",
		"BROWSER_CONNECT",
	})
}

// appendYoutubeTranscript runs `opencli youtube transcript <url>` and merges
// transcript text into the fetch result body. A failed transcript command
// (network/anti-bot, "Caption URL returned empty response") returns a
// retryable failure so ingest keeps the inbox line; only explicit no-caption
// output counts as content absence and falls through to the too-short gate.
func (d *openCLIDriver) appendYoutubeTranscript(
	ctx context.Context,
	result *types.ContentFetchResult,
	rawURL string,
) *types.ContentFetchResult {
	subResult := d.runOpenCLI(ctx, "youtube", []string{"transcript", rawURL, "--format", "md"})

	detail := subResult.Error
	if detail == "" {
		if envelopeDetail, isErr := openCLIErrorDetail(subResult.Body); isErr {
			detail = envelopeDetail
		}
	}
	if detail != "" {
		if isYoutubeNoCaptions(detail) {
			slog.Info("YouTube transcript unavailable (no captions)", "url", rawURL)

			return nil
		}
		slog.Warn("YouTube transcript command failed; keeping URL retryable",
			"url", rawURL, "reason", subResult.FailureReason, "error", detail)

		return extractFailureWithReason(
			rawURL,
			"youtube transcript unavailable: "+detail,
			types.FailureReasonPartialExtract,
		)
	}

	transcript := strings.TrimSpace(subResult.Body)
	if len([]rune(transcript)) < 100 {
		return nil
	}

	// Parse the YouTube transcript table (| timestamp | speaker | text |) with goldmark.
	transcriptLines := md.ExtractTranscriptLines(transcript, 2)

	if len(transcriptLines) < 5 {
		return nil
	}

	transcriptBlock := "\n\n" + types.TranscriptSectionMarker + "\n\n" + strings.Join(transcriptLines, "\n")
	result.Body += transcriptBlock
	result.HasTranscript = true
	slog.Info("YouTube transcript appended", "url", rawURL, "lines", len(transcriptLines))

	return nil
}

// parseOpenCLIErrorEnvelope parses opencli's 2.x error envelope from output.
func parseOpenCLIErrorEnvelope(body []byte) (openCLIErrorEnvelope, bool) {
	var env openCLIErrorEnvelope
	if json.Unmarshal(body, &env) == nil && !env.OK && env.Error.Code != "" {
		return env, true
	}

	return env, false
}

// openCLIErrorDetail extracts the 2.x error envelope from stdout for callers
// that inspect structured output without an exec error.
func openCLIErrorDetail(body string) (string, bool) {
	env, ok := parseOpenCLIErrorEnvelope([]byte(body))
	if !ok {
		return "", false
	}

	return env.Error.Code + ": " + env.Error.Message, true
}

// isYoutubeNoCaptions reports explicit "this video has no captions" output.
// Anything else — including "Caption URL returned empty response" — is a
// transient command failure, so the URL stays retryable.
func isYoutubeNoCaptions(detail string) bool {
	lower := strings.ToLower(detail)

	return strings.Contains(lower, "empty_result") ||
		textutil.ContainsAny(lower, []string{
			"no caption",
			"captions not available",
			"captions unavailable",
			"no subtitle",
			"no transcript",
			"transcript not available",
			"transcript unavailable",
		})
}
