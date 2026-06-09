package wiki

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/adrg/frontmatter"
	"github.com/goccy/go-yaml"
	"github.com/xbpk3t/docs-alfred/pkg/fileutil"
	"github.com/xbpk3t/docs-alfred/pkg/textutil"
	"github.com/xbpk3t/docs-alfred/pkg/urlutil"
)

// SummaryFrontmatter is the YAML frontmatter for summary.md files.
type SummaryFrontmatter struct {
	Title     string `yaml:"title"`
	Date      string `yaml:"date"`
	Source    string `yaml:"source"`
	BatchID   string `yaml:"batch_id"`
	TotalURLs int    `yaml:"total_urls"`
	Succeeded int    `yaml:"succeeded"`
	Failed    int    `yaml:"failed"`
}

// Failure type constants for WriteFailureEntry.
const (
	FailureFetch    = "fetch"
	FailureResolve  = "resolve"
	FailureExtract  = "extract"
	FailureClassify = "classify"
)

// WriteOptions contains options for writing wiki entries.
type WriteOptions struct {
	WikiRoot string
	BatchID  string
	DryRun   bool
}

var pathLocks = struct {
	locks map[string]*sync.Mutex
	mu    sync.Mutex
}{locks: make(map[string]*sync.Mutex)}

func lockPath(path string) func() {
	key, err := filepath.Abs(path)
	if err != nil {
		key = filepath.Clean(path)
	}

	pathLocks.mu.Lock()
	mu := pathLocks.locks[key]
	if mu == nil {
		mu = &sync.Mutex{}
		pathLocks.locks[key] = mu
	}
	pathLocks.mu.Unlock()

	mu.Lock()

	return mu.Unlock
}

// WriteSummary writes a structured summary.md entry with YAML frontmatter.
// Follows TS writer.ts: appendToSummaryFile.
func WriteSummary(item *ClassifyItem, opts *WriteOptions) (string, error) {
	topicDir := resolveTopicDir(item, opts)
	summaryPath := filepath.Join(topicDir, "summary.md")
	if opts.DryRun {
		slog.Info("[DRY RUN] Would write summary", "path", summaryPath)

		return summaryPath, nil
	}
	unlock := lockPath(summaryPath)
	defer unlock()

	if err := fileutil.EnsureDir(topicDir); err != nil {
		return "", fmt.Errorf("create topic dir: %w", err)
	}

	today := time.Now().Format("2006-01-02")
	dateHeading := "## " + today

	batchID := opts.BatchID
	if batchID == "" {
		batchID = "wiki-" + today
	}

	entry := buildEntry(item)
	fm, existingBody := loadFrontmatter(summaryPath, item, today, batchID)

	fm.TotalURLs++
	fm.Succeeded++
	fm.BatchID = batchID
	fm.Date = today
	fm.Title = filepath.Base(item.TopicPath)

	newBody := appendEntryBody(existingBody, dateHeading, entry)

	content := renderContent(fm, newBody)

	if err := fileutil.AtomicWriteFile(summaryPath, []byte(content), fileutil.FilePermPrivate); err != nil {
		return "", fmt.Errorf("write summary: %w", err)
	}

	slog.Info("Summary written", "path", summaryPath, "topic", item.TopicPath)

	return summaryPath, nil
}

func resolveTopicDir(item *ClassifyItem, opts *WriteOptions) string {
	return filepath.Join(opts.WikiRoot, item.TopicPath)
}

func buildEntry(item *ClassifyItem) string {
	title := item.Title
	if title == "" {
		title = item.URL
	}

	return fmt.Sprintf("### %s\n\n- URL: %s\n- Type: %s\n\n%s\n", title, item.URL, item.Type, item.Summary)
}

//nolint:nonamedreturns
func loadFrontmatter(summaryPath string, item *ClassifyItem, today, batchID string) (fm *SummaryFrontmatter, body string) {
	fm = &SummaryFrontmatter{
		Title:     filepath.Base(item.TopicPath),
		Date:      today,
		Source:    "rss2nl-wiki",
		BatchID:   batchID,
		TotalURLs: 0,
		Succeeded: 0,
		Failed:    0,
	}
	if data, err := os.ReadFile(summaryPath); err == nil {
		if parsed := parseSummaryFrontmatter(string(data)); parsed != nil {
			fm = parsed.fm
			body = parsed.body

			return
		}
		body = string(data)

		return
	}

	return
}

func appendEntryBody(existingBody, dateHeading, entry string) string {
	if strings.Contains(existingBody, dateHeading) {
		return appendEntryToExistingDateSection(existingBody, dateHeading, entry)
	}
	insertPos := strings.Index(existingBody, "\n## ")
	if insertPos >= 0 {
		return fmt.Sprintf("%s\n%s\n\n%s\n%s", existingBody[:insertPos], dateHeading, entry, existingBody[insertPos:])
	}

	return fmt.Sprintf("%s%s\n\n%s\n", existingBody, dateHeading, entry)
}

func appendEntryToExistingDateSection(existingBody, dateHeading, entry string) string {
	dateStart := strings.Index(existingBody, dateHeading)
	if dateStart < 0 {
		return existingBody
	}
	sectionStart := dateStart + len(dateHeading)
	nextHeadingRel := strings.Index(existingBody[sectionStart:], "\n## ")
	insertPos := len(existingBody)
	if nextHeadingRel >= 0 {
		insertPos = sectionStart + nextHeadingRel
	}

	prefix := strings.TrimRight(existingBody[:insertPos], "\n")
	suffix := existingBody[insertPos:]
	if strings.TrimSpace(suffix) == "" {
		return fmt.Sprintf("%s\n\n%s\n", prefix, entry)
	}

	return fmt.Sprintf("%s\n\n%s\n%s", prefix, entry, suffix)
}

func renderContent(fm *SummaryFrontmatter, body string) string {
	fmYAML, err := yaml.Marshal(fm)
	if err != nil {
		return "---\n---\n\n" + body
	}

	return fmt.Sprintf("---\n%s---\n\n%s", string(fmYAML), body)
}

var failureFilenames = map[string]string{
	FailureFetch:    "fetch-failed.md",
	FailureResolve:  "resolve-failed.md",
	FailureExtract:  "extract-failed.md",
	FailureClassify: "group-failed.md",
}

// WriteFailureEntry writes a structured failure entry to the appropriate file
// under <wikiRoot>/failed/<failureType>-failed.md. Each file collects one type:
//   - fetch:     network-level errors (DNS, timeout) with no opencli fallback
//   - resolve:   HTTP-level errors (403 anti-bot) where opencli also failed
//   - extract:   URL fetched but usable article/media content could not be extracted
//   - classify:  content fetched but AI couldn't assign a topic
//
// Format is a structured markdown entry with title, failure reason, and content snippet.
func WriteFailureEntry(item *ClassifyItem, failureType, extraInfo string, opts *WriteOptions) (string, error) {
	filename, ok := failureFilenames[failureType]
	if !ok {
		return "", fmt.Errorf("unknown failure type: %s", failureType)
	}

	failedDir := filepath.Join(opts.WikiRoot, "failed")
	path := filepath.Join(failedDir, filename)

	if opts.DryRun {
		slog.Info("[DRY RUN] Would write failure entry", "path", path, "type", failureType)

		return path, nil
	}
	unlock := lockPath(path)
	defer unlock()

	if err := fileutil.EnsureDir(failedDir); err != nil {
		return "", fmt.Errorf("create failed dir: %w", err)
	}

	entry := buildFailureEntry(item, extraInfo)

	if err := appendToFile(path, entry); err != nil {
		return "", err
	}

	slog.Info("Failure entry written", "path", path, "type", failureType, "url", item.URL)

	return path, nil
}

func buildFailureEntry(item *ClassifyItem, extraInfo string) string {
	title := item.Title
	if title == "" {
		title = item.URL
	}

	bodySnippet := item.Summary
	if bodySnippet == "" {
		bodySnippet = "(无内容)"
	}
	bodySnippet = textutil.TruncateUTF8(bodySnippet, 500)

	return fmt.Sprintf(`---

## [%s](%s)

### 失败原因
%s

### 内容片段
%s

`, title, item.URL, extraInfo, bodySnippet)
}

func appendToFile(path, content string) error {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, fileutil.FilePermPrivate)
	if err != nil {
		return fmt.Errorf("open failed file: %w", err)
	}
	defer func() { _ = f.Close() }()

	if _, err := f.WriteString(content); err != nil {
		return fmt.Errorf("write failure entry: %w", err)
	}

	return nil
}

// parseSummaryFrontmatterResult holds the result of parsing.
type parseResult struct {
	fm   *SummaryFrontmatter
	body string
}

// parseSummaryFrontmatter parses YAML frontmatter from a summary.md file.
func parseSummaryFrontmatter(raw string) *parseResult {
	if !strings.HasPrefix(raw, "---") {
		return nil
	}

	var fm SummaryFrontmatter
	body, err := frontmatter.Parse(strings.NewReader(raw), &fm)
	if err != nil || len(body) == len(raw) {
		return nil
	}

	return &parseResult{fm: &fm, body: string(body)}
}

// ParseInbox parses inbox.md and returns a list of URL entries.
func ParseInbox(filePath string) ([]InboxEntry, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	lines := strings.Split(string(data), "\n")
	var entries []InboxEntry
	seen := make(map[string]bool)

	for i, line := range lines {
		for _, u := range extractInboxLineURLs(line) {
			entries = appendInboxEntry(entries, seen, u, i)
		}
	}

	return entries, nil
}

func extractInboxLineURLs(line string) []string {
	refs := extractInboxLineURLRefs(line)
	urls := make([]string, 0, len(refs))
	for _, ref := range refs {
		urls = append(urls, ref.URL)
	}

	return urls
}

func extractInboxLineURLRefs(line string) []urlutil.URLRef {
	if lineHasRawMalformedURL(line) {
		return nil
	}

	return urlutil.ExtractURLRefs(line, urlutil.ExtractOptions{
		Markdown:    true,
		BareURLs:    true,
		HTTPOnly:    true,
		Normalize:   true,
		Deduplicate: false,
	})
}

func appendInboxEntry(entries []InboxEntry, seen map[string]bool, raw string, lineIndex int) []InboxEntry {
	cleaned := cleanInboxURL(raw)
	if cleaned == "" {
		return entries
	}
	key := urlutil.Normalize(cleaned)
	if seen[key] {
		return entries
	}
	seen[key] = true

	return append(entries, InboxEntry{URL: cleaned, LineIndex: lineIndex})
}

func cleanInboxURL(raw string) string {
	return urlutil.CleanHTTPURL(raw)
}

// InboxEntry represents a URL found in inbox.md.
type InboxEntry struct {
	URL       string `json:"url"`
	LineIndex int    `json:"lineIndex"`
}

// FlushInbox removes handled URLs from inbox.md without dropping unhandled URLs
// that share the same line.
func FlushInbox(filePath string, handledURLsByLine map[int][]string) error {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}

	lines := strings.Split(string(data), "\n")
	var remaining []string
	for i, line := range lines {
		handled := normalizedURLSet(handledURLsByLine[i])
		if len(handled) == 0 {
			remaining = append(remaining, line)

			continue
		}

		flushedLine := flushInboxLine(line, handled)
		if strings.TrimSpace(flushedLine) != "" {
			remaining = append(remaining, flushedLine)
		}
	}

	var cleaned []string
	for _, l := range remaining {
		if strings.TrimSpace(l) != "" || len(cleaned) > 0 {
			cleaned = append(cleaned, l)
		}
	}

	if len(cleaned) == 0 {
		return fileutil.AtomicWriteFile(filePath, []byte{}, fileutil.FilePermPrivate)
	}

	return fileutil.AtomicWriteFile(filePath, []byte(strings.Join(cleaned, "\n")), fileutil.FilePermPrivate) // #nosec G703
}

func normalizedURLSet(urls []string) map[string]bool {
	return urlutil.NormalizeSet(urls)
}

func flushInboxLine(line string, handled map[string]bool) string {
	refs := extractInboxLineURLRefs(line)
	if len(refs) == 0 {
		return line
	}
	var unhandled int
	flushed := false
	for _, v := range slices.Backward(refs) {
		ref := v
		if !handled[ref.Normalized] {
			unhandled++

			continue
		}
		line = line[:ref.Start] + line[ref.End:]
		flushed = true
	}
	if !flushed {
		return line
	}
	if unhandled == 0 {
		return ""
	}

	return cleanFlushedInboxLine(line)
}

func cleanFlushedInboxLine(line string) string {
	line = strings.TrimSpace(line)
	for strings.Contains(line, "  ") {
		line = strings.ReplaceAll(line, "  ", " ")
	}
	line = strings.ReplaceAll(line, "- ,", "-")
	line = strings.ReplaceAll(line, "- .", "-")
	line = strings.ReplaceAll(line, "- ;", "-")
	line = strings.ReplaceAll(line, "- :", "-")

	return strings.TrimSpace(line)
}
