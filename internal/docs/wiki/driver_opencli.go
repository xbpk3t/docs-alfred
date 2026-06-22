package wiki

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"

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

func (d *openCLIDriver) FetchContent(ctx context.Context, urlStr, contentType string) *ContentFetchResult {
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

func (d *openCLIDriver) fetchWithAdapter(ctx context.Context, rawURL string) *ContentFetchResult {
	subcommand, extraArgs := opencli.CommandForURL(rawURL)

	// weixin download saves article content to a local file (not stdout),
	// so we need a special path: save to /tmp, parse the saved path from
	// the YAML output, then read the file content as the fetch result.
	if subcommand == "weixin" {
		return d.fetchWeixinArticle(ctx, rawURL)
	}

	result := d.runOpenCLI(ctx, subcommand, extraArgs)
	result.SourceURL = rawURL

	// For bilibili, `bilibili video` only returns metadata (title, stats, description)
	// without transcript. Run `bilibili subtitle <bvid>` separately and merge the
	// transcript content into the body so the AI has actual content to summarize.
	if subcommand == "bilibili" && result.Error == "" && result.Body != "" {
		d.appendBilibiliTranscript(ctx, result, rawURL)
	}
	if subcommand == "youtube" && result.Error == "" && result.Body != "" {
		d.appendYoutubeTranscript(ctx, result, rawURL)
	}

	return result
}

// appendBilibiliTranscript runs `opencli bilibili subtitle <bvid>` for the given
// bilibili URL and merges any transcript content into the fetch result body.
// When no subtitles are available, falls back to `bilibili summary <bvid>` for
// B站's official AI-generated summary (with section outlines and timestamps).
func (d *openCLIDriver) appendBilibiliTranscript(ctx context.Context, result *ContentFetchResult, rawURL string) {
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
func (d *openCLIDriver) appendBilibiliSummary(ctx context.Context, result *ContentFetchResult, rawURL string) {
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

func (d *openCLIDriver) fetchWebRead(ctx context.Context, rawURL string) *ContentFetchResult {
	r := d.runOpenCLI(ctx, "web", []string{"read", "--url", rawURL, "--stdout"})
	r.SourceURL = rawURL

	return r
}

// fetchWeixinArticle fetches a WeChat public account article via
// `opencli weixin download`, which saves the content to a local file
// under /tmp and outputs metadata + the saved file path on stdout.
// We parse the saved path, read the markdown file, and return it as
// the fetch result body.
func (d *openCLIDriver) fetchWeixinArticle(ctx context.Context, rawURL string) *ContentFetchResult {
	// Create a temp directory for weixin downloads.
	tmpDir, err := os.MkdirTemp("", "weixin-*")
	if err != nil {
		return &ContentFetchResult{
			Error:       fmt.Sprintf("weixin: create temp dir: %v", err),
			SourceURL:   rawURL,
			FailureKind: FailureExtract,
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
		return &ContentFetchResult{
			Error:       fmt.Sprintf("weixin: %v (stderr: %s)", runErr, strings.TrimSpace(string(stderr))),
			SourceURL:   rawURL,
			FailureKind: FailureFetch,
		}
	}

	// Parse the YAML output to find the saved file path.
	// YAML format:
	//   - title: ...
	//     ...
	//     saved: /tmp/weixin-xxx/红筹退潮.../红筹退潮....md
	savedPath := extractWeixinSavedPath(string(stdout))
	if savedPath == "" {
		return &ContentFetchResult{
			Error:       "weixin: could not find saved file path in output",
			SourceURL:   rawURL,
			FailureKind: FailureExtract,
		}
	}

	// Read the saved markdown file content.
	content, err := os.ReadFile(savedPath)
	if err != nil {
		return &ContentFetchResult{
			Error:       fmt.Sprintf("weixin: read saved file: %v", err),
			SourceURL:   rawURL,
			FailureKind: FailureExtract,
		}
	}

	body := string(content)
	body = textutil.TruncateUTF8(body, d.maxBodySize*3)
	title := md.ExtractTitleFromMarkdown(body)

	slog.Info("weixin: article fetched successfully",
		"url", rawURL, "title", title, "size", len(body))

	return &ContentFetchResult{Title: title, Body: body, SourceURL: rawURL}
}

// extractWeixinSavedPath parses the YAML output from `weixin download -f yaml`
// and returns the value of the `saved:` field (the absolute path to the saved
// markdown file).
func extractWeixinSavedPath(yamlOutput string) string {
	for line := range strings.SplitSeq(yamlOutput, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "saved:") {
			if path := extractSavedPathFromLine(line); path != "" {
				return path
			}
		}
	}

	return ""
}

// extractSavedPathFromLine extracts the saved file path from a YAML line
// containing the "saved:" field. Returns empty string if no path found.
func extractSavedPathFromLine(line string) string {
	_, after, ok := strings.Cut(line, "saved: ")
	if !ok {
		return ""
	}

	return strings.TrimSpace(after)
}

func (d *openCLIDriver) runOpenCLI(ctx context.Context, subcommand string, extraArgs []string) *ContentFetchResult {
	args := append([]string{subcommand}, extraArgs...)
	stdout, stderr, err := cmdutil.RunSeparate(ctx, "opencli", args...)
	if err != nil {
		return &ContentFetchResult{
			Error: fmt.Sprintf("opencli: %v (stderr: %s)", err, strings.TrimSpace(string(stderr))),
		}
	}

	body := string(stdout)
	if body == "" {
		return &ContentFetchResult{Error: "opencli returned empty content"}
	}

	body = textutil.TruncateUTF8(body, d.maxBodySize*3)
	title := md.ExtractTitleFromMarkdown(body)

	slog.Info("opencli fetch succeeded", "subcommand", subcommand, "bodyLen", len(body))

	return &ContentFetchResult{Title: title, Body: body}
}

// appendYoutubeTranscript runs `opencli youtube transcript <url>` and merges
// transcript text into the fetch result body.
func (d *openCLIDriver) appendYoutubeTranscript(ctx context.Context, result *ContentFetchResult, rawURL string) {
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
