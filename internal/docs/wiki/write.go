package wiki

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	"github.com/adrg/frontmatter"
	carbon "github.com/dromara/carbon/v2"
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
	Type      string `yaml:"type"`
	BatchID   string `yaml:"batch_id"`
	TotalURLs int    `yaml:"total_urls"`
	Succeeded int    `yaml:"succeeded"`
	Failed    int    `yaml:"failed"`
}

// FailureKind classifies wiki ingestion failures.
type FailureKind string

// Failure type constants for WriteFailureEntry.
const (
	FailureFetch    FailureKind = "fetch"
	FailureResolve  FailureKind = "resolve"
	FailureExtract  FailureKind = "extract"
	FailureClassify FailureKind = "classify"
	FailureAI       FailureKind = "ai"
)

// String returns the stable wire value for the failure kind.
func (k FailureKind) String() string { return string(k) }

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

	return func() {
		mu.Unlock()
		// Clean up the map entry if no other goroutine is waiting.
		pathLocks.mu.Lock()
		if pathLocks.locks[key] == mu {
			delete(pathLocks.locks, key)
		}
		pathLocks.mu.Unlock()
	}
}

// WriteSummary writes a structured summary.md entry with YAML frontmatter.
// Follows TS writer.ts: appendToSummaryFile.
func WriteSummary(item *ClassifyItem, opts *WriteOptions) (string, error) {
	if item.TopicPath == "" {
		// Empty TopicPath would write to wiki root — route to classify rejection.
		_, err := WriteFailureEntry(item, FailureClassify, "empty topic path", opts)

		return "", err
	}

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

	today := carbon.Now().ToDateString()
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

	// Log success to JSONL.
	if _, err := LogSuccessEntry(item, summaryPath, opts); err != nil {
		slog.Warn("Failed to log success entry", "url", item.URL, "error", err)
	}

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

	// Build codeblock metadata section. URL is always the first field.
	metaBlock := fmt.Sprintf("URL: %s\n", item.URL)
	if item.MetadataBlock != "" {
		metaBlock += item.MetadataBlock
	} else {
		// Fallback: just Type if no structured metadata.
		metaBlock += fmt.Sprintf("Type: %s", item.Type)
	}

	return fmt.Sprintf("### %s\n\n```markdown\n%s\n```\n\n%s\n", title, metaBlock, RenderStructuredSummary(item.Summary))
}

//nolint:nonamedreturns
func loadFrontmatter(summaryPath string, item *ClassifyItem, today, batchID string) (fm *SummaryFrontmatter, body string) {
	fm = &SummaryFrontmatter{
		Title:     filepath.Base(item.TopicPath),
		Date:      today,
		Source:    "rss2nl-wiki",
		Type:      string(item.Type),
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

// WriteFailureEntry logs a structured failure entry to the appropriate JSONL log
// under <wikiRoot>/digest-<type>-error.jsonl. Each log file collects one type:
//   - fetch:     network-level errors (DNS, timeout) with no opencli fallback
//   - resolve:   HTTP-level errors (403 anti-bot) where opencli also failed
//   - extract:   URL fetched but usable article/media content could not be extracted
//   - classify:  content fetched but AI couldn't assign a topic
//
// The legacy MD file format is replaced by JSONL for machine-parseable diagnostics.
func WriteFailureEntry(item *ClassifyItem, failureType FailureKind, extraInfo string, opts *WriteOptions) (string, error) {
	if opts == nil {
		return "", errors.New("write options required for failure entry")
	}

	entry := DigestEntry{
		URL:         itemURL(item),
		Stage:       digestStageForFailure(failureType),
		Status:      DigestFailure,
		FailureKind: string(failureType),
		Error:       extraInfo,
		TopicPath:   item.TopicPath,
	}

	path, err := LogDigestEntry(&entry, opts)
	if err != nil {
		return "", fmt.Errorf("log %s failure for %s: %w", failureType, itemURL(item), err)
	}

	slog.Info("Failure entry logged", "path", path, "type", failureType, "url", item.URL)

	return path, nil
}

// LogSuccessEntry logs a successful pipeline outcome to digest-success.jsonl.
func LogSuccessEntry(item *ClassifyItem, outputPath string, opts *WriteOptions) (string, error) {
	if opts == nil {
		return "", nil
	}

	entry := DigestEntry{
		URL:        itemURL(item),
		Stage:      StageWrite,
		Status:     DigestSuccess,
		TopicPath:  item.TopicPath,
		OutputPath: outputPath,
	}

	return LogDigestEntry(&entry, opts)
}

// WriteManualReviewEntry appends a human-readable entry to wiki/uncat.md
// for items with NeedsManualReview=true that have good AI-generated summaries.
func WriteManualReviewEntry(item *ClassifyItem, opts *WriteOptions) (string, error) {
	if opts == nil || item == nil {
		return "", nil
	}

	path := filepath.Join(opts.WikiRoot, "uncat.md")
	if opts.DryRun {
		slog.Info("[DRY RUN] Would append to uncat.md", "path", path, "url", item.URL)

		return path, nil
	}
	unlock := lockPath(path)
	defer unlock()

	today := carbon.Now().ToDateString()
	dateHeading := "## " + today

	title, metaBlock := buildReviewEntryMeta(item)
	entry := fmt.Sprintf("\n### %s\n\n```markdown\n%s\n```\n\n%s\n", title, metaBlock, RenderStructuredSummary(item.Summary))

	if err := appendToDateSectionAndWrite(path, dateHeading, entry); err != nil {
		return "", fmt.Errorf("write uncat.md: %w", err)
	}

	slog.Info("Manual review entry appended", "path", path, "url", item.URL)

	// Log success to JSONL so uncat.md entries count toward the success rate.
	if _, err := LogSuccessEntry(item, path, opts); err != nil {
		slog.Warn("Failed to log success entry for manual review", "url", item.URL, "error", err)
	}

	return path, nil
}

// buildReviewEntryMeta builds the title and metadata block for a manual review entry.
//
//nolint:nonamedreturns
func buildReviewEntryMeta(item *ClassifyItem) (title, metaBlock string) {
	title = item.Title
	if title == "" {
		title = item.URL
	}

	// Build codeblock metadata section matching buildEntry format.
	metaBlock = fmt.Sprintf("URL: %s\n", item.URL)
	if item.MetadataBlock != "" {
		metaBlock += item.MetadataBlock
	} else if item.Type != "" {
		metaBlock += fmt.Sprintf("Type: %s", item.Type)
	}

	return
}

// appendToDateSectionAndWrite appends an entry to the appropriate date section
// in the uncat.md file and writes the result.
func appendToDateSectionAndWrite(path, dateHeading, entry string) error {
	var existing strings.Builder
	if data, err := os.ReadFile(path); err == nil {
		existing.Write(data)
	}

	var newContent string
	if strings.Contains(existing.String(), dateHeading) {
		newContent = appendEntryToExistingDateSection(existing.String(), dateHeading, entry)
	} else if existing.Len() > 0 {
		newContent = fmt.Sprintf("%s\n%s\n\n%s\n", strings.TrimRight(existing.String(), "\n"), dateHeading, entry)
	} else {
		newContent = fmt.Sprintf("%s\n\n%s\n", dateHeading, entry)
	}

	return fileutil.AtomicWriteFile(path, []byte(newContent), fileutil.FilePermPrivate)
}

func itemURL(item *ClassifyItem) string {
	if item == nil {
		return ""
	}

	return item.URL
}

func digestStageForFailure(failureType FailureKind) DigestStage {
	switch failureType {
	case FailureFetch, FailureResolve:
		return StageFetch
	case FailureExtract:
		return StageExtract
	case FailureClassify:
		return StageClassify
	default:
		return StageClassify
	}
}

func buildFailureEntry(item *ClassifyItem, extraInfo string) string {
	title := item.Title
	if title == "" {
		title = item.URL
	}

	bodySnippet := RenderStructuredSummary(item.Summary)
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

// nolint: unused
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
	// Fix punctuation artifacts from URL removal.
	if strings.Contains(line, "- ,") || strings.Contains(line, "- .") ||
		strings.Contains(line, "- ;") || strings.Contains(line, "- :") {
		slog.Debug("cleaning punctuation artifacts from flushed inbox line", "original", line)
	}
	line = strings.ReplaceAll(line, "- ,", "-")
	line = strings.ReplaceAll(line, "- .", "-")
	line = strings.ReplaceAll(line, "- ;", "-")
	line = strings.ReplaceAll(line, "- :", "-")

	return strings.TrimSpace(line)
}
