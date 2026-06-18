package wiki

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"strings"

	"github.com/xbpk3t/docs-alfred/pkg/opencli"
	"github.com/xbpk3t/docs-alfred/pkg/textutil"
	"github.com/yuin/goldmark"
	gast "github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	geast "github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/text"
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

	transcriptLines := bilibiliTranscriptLines(transcript)
	if len(transcriptLines) < 5 {
		return
	}

	transcriptBlock := "\n\n## 字幕内容\n\n" + strings.Join(transcriptLines, "\n")
	result.Body += transcriptBlock
	slog.Info("Bilibili transcript appended", "url", rawURL, "lines", len(transcriptLines))
}

// transcriptLinesFromTable parses a markdown table using goldmark and returns
// the text from the specified column index (0-based). Skips table header rows.
// Used for both bilibili subtitle (| index | from | to | content |, col=3)
// and YouTube transcript (| timestamp | speaker | text |, col=2) tables.
func transcriptLinesFromTable(mdBody string, contentCol int) []string {
	md := goldmark.New(
		goldmark.WithExtensions(extension.Table),
	)
	reader := text.NewReader([]byte(mdBody))
	doc := md.Parser().Parse(reader)

	var lines []string
	_ = gast.Walk(doc, func(n gast.Node, entering bool) (gast.WalkStatus, error) {
		if !entering {
			return gast.WalkContinue, nil
		}
		if n.Kind() != geast.KindTable {
			return gast.WalkContinue, nil
		}
		lines = extractTableLinesFromRow(n, mdBody, contentCol)

		return gast.WalkStop, nil
	})

	return lines
}

// extractTableLinesFromRow iterates over table rows and extracts text from the specified column.
// It skips the header row and returns non-empty cell text.
func extractTableLinesFromRow(n gast.Node, mdBody string, contentCol int) []string {
	var lines []string
	isFirstRow := true
	for row := n.FirstChild(); row != nil; row = row.NextSibling() {
		if row.Kind() != geast.KindTableRow {
			continue
		}
		// Skip the header row (first row after the table node).
		if isFirstRow {
			isFirstRow = false

			continue
		}
		cell := row.FirstChild()
		for i := 0; i < contentCol && cell != nil; i++ {
			cell = cell.NextSibling()
		}
		if cell == nil || cell.Kind() != geast.KindTableCell {
			continue
		}
		cellText := strings.TrimSpace(string(cell.Text([]byte(mdBody))))
		if cellText == "" {
			continue
		}
		lines = append(lines, cellText)
	}

	return lines
}

// bilibiliTranscriptLines parses the bilibili subtitle markdown table
// (| index | from | to | content |) using goldmark and returns the content column.
func bilibiliTranscriptLines(mdBody string) []string {
	return transcriptLinesFromTable(mdBody, 3)
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

	cmd := exec.CommandContext(ctx, "curl", "-sL", "-o", "/dev/null",
		"-w", "%{url_effective}", "--connect-timeout", "10",
		"--max-time", "30", rawURL)
	out, err := cmd.Output()
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
	args := []string{"download", "--url", rawURL, "-f", "yaml", "--output", tmpDir}
	cmd := exec.CommandContext(ctx, "opencli", append([]string{"weixin"}, args...)...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	slog.Info("weixin: downloading article", "url", rawURL)
	if runErr := cmd.Run(); runErr != nil {
		return &ContentFetchResult{
			Error:       fmt.Sprintf("weixin: %v (stderr: %s)", runErr, strings.TrimSpace(stderr.String())),
			SourceURL:   rawURL,
			FailureKind: FailureFetch,
		}
	}

	// Parse the YAML output to find the saved file path.
	// YAML format:
	//   - title: ...
	//     ...
	//     saved: /tmp/weixin-xxx/红筹退潮.../红筹退潮....md
	savedPath := extractWeixinSavedPath(stdout.String())
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
	title := extractTitleFromMarkdown(body)

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
	prefix := "  saved: "
	_, after, ok := strings.Cut(line, prefix)
	if ok {
		path := strings.TrimSpace(after)
		if path != "" {
			return path
		}
	}

	// Fallback: handle "saved: " with varying whitespace.
	_, after, ok = strings.Cut(line, "saved: ")
	if ok {
		return strings.TrimSpace(after)
	}

	return ""
}

func (d *openCLIDriver) runOpenCLI(ctx context.Context, subcommand string, extraArgs []string) *ContentFetchResult {
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

	body = textutil.TruncateUTF8(body, d.maxBodySize*3)
	title := extractTitleFromMarkdown(body)

	slog.Info("opencli fetch succeeded", "subcommand", subcommand, "bodyLen", len(body))

	return &ContentFetchResult{Title: title, Body: body}
}

// extractTitleFromMarkdown parses the markdown body with goldmark and returns
// the title from either:
//   - the first heading (any level), or
//   - a metadata-style table (| field | value |) with a "title" field.
//
// This handles all opencli adapters — those that return heading-prefixed
// markdown (twitter, web read, etc.) and those that return metadata tables
// (bilibili video, etc.).
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
	transcriptLines := transcriptLinesFromTable(transcript, 2)

	if len(transcriptLines) < 5 {
		return
	}

	transcriptBlock := "\n\n## 字幕内容\n\n" + strings.Join(transcriptLines, "\n")
	result.Body += transcriptBlock
	slog.Info("YouTube transcript appended", "url", rawURL, "lines", len(transcriptLines))
}

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
