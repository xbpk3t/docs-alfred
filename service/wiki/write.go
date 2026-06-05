package wiki

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/goccy/go-yaml"
	"github.com/xbpk3t/docs-alfred/pkg/fileutil"
	"mvdan.cc/xurls/v2"
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
	FailureClassify = "classify"
)

// WriteOptions contains options for writing wiki entries.
type WriteOptions struct {
	WikiRoot string
	BatchID  string
	DryRun   bool
}

// WriteSummary writes a structured summary.md entry with YAML frontmatter.
// Follows TS writer.ts: appendToSummaryFile.
func WriteSummary(item *ClassifyItem, opts *WriteOptions) (string, error) {
	topicDir := resolveTopicDir(item, opts)
	if err := os.MkdirAll(topicDir, fileutil.DirPerm); err != nil {
		return "", fmt.Errorf("create topic dir: %w", err)
	}

	summaryPath := filepath.Join(topicDir, "summary.md")
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

	if opts.DryRun {
		slog.Info("[DRY RUN] Would write summary", "path", summaryPath)

		return summaryPath, nil
	}

	if err := os.WriteFile(summaryPath, []byte(content), fileutil.FilePermPrivate); err != nil {
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
		return strings.Replace(existingBody, dateHeading, fmt.Sprintf("%s\n\n%s", dateHeading, entry), 1)
	}
	insertPos := strings.Index(existingBody, "\n## ")
	if insertPos >= 0 {
		return fmt.Sprintf("%s\n%s\n\n%s\n%s", existingBody[:insertPos], dateHeading, entry, existingBody[insertPos:])
	}

	return fmt.Sprintf("%s%s\n\n%s\n", existingBody, dateHeading, entry)
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
	FailureClassify: "group-failed.md",
}

// WriteFailureEntry writes a structured failure entry to the appropriate file
// under <wikiRoot>/failed/<failureType>-failed.md. Each file collects one type:
//   - fetch:     network-level errors (DNS, timeout) with no opencli fallback
//   - resolve:   HTTP-level errors (403 anti-bot) where opencli also failed
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

	if err := os.MkdirAll(failedDir, fileutil.DirPerm); err != nil {
		return "", fmt.Errorf("create failed dir: %w", err)
	}

	entry := buildFailureEntry(item, extraInfo)

	if opts.DryRun {
		slog.Info("[DRY RUN] Would write failure entry", "path", path, "type", failureType)

		return path, nil
	}

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
	if len(bodySnippet) > 500 {
		bodySnippet = bodySnippet[:500] + "..."
	}

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
	idx := strings.Index(raw, "---")
	if idx < 0 {
		return nil
	}
	end := strings.Index(raw[idx+3:], "---")
	if end < 0 {
		return nil
	}
	fmRaw := raw[idx+3 : idx+3+end]
	bodyRaw := raw[idx+3+end+3:]

	var fm SummaryFrontmatter
	if err := yaml.Unmarshal([]byte(fmRaw), &fm); err != nil {
		return nil
	}

	return &parseResult{fm: &fm, body: bodyRaw}
}

// urlRegex extracts URLs with schemes (http, https, ftp, etc.) from plain text.
// Handles both markdown [text](url) and bare URLs in a single pass.
var urlRegex = xurls.Strict()

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
		for _, u := range urlRegex.FindAllString(line, -1) {
			if !strings.HasPrefix(u, "http") {
				continue
			}
			if !seen[u] {
				seen[u] = true
				entries = append(entries, InboxEntry{URL: u, LineIndex: i})
			}
		}
	}

	return entries, nil
}

// InboxEntry represents a URL found in inbox.md.
type InboxEntry struct {
	URL       string `json:"url"`
	LineIndex int    `json:"lineIndex"`
}

// FlushInbox removes processed lines from inbox.md.
func FlushInbox(filePath string, processedLineIndices map[int]bool) error {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}

	lines := strings.Split(string(data), "\n")
	var remaining []string
	for i, line := range lines {
		if !processedLineIndices[i] {
			remaining = append(remaining, line)
		}
	}

	var cleaned []string
	for _, l := range remaining {
		if strings.TrimSpace(l) != "" || len(cleaned) > 0 {
			cleaned = append(cleaned, l)
		}
	}

	if len(cleaned) == 0 {
		return os.WriteFile(filePath, []byte{}, fileutil.FilePermPrivate)
	}

	return os.WriteFile(filePath, []byte(strings.Join(cleaned, "\n")), fileutil.FilePermPrivate) // #nosec G703
}
