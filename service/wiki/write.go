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

// WriteOptions contains options for writing wiki entries.
type WriteOptions struct {
	WikiRoot    string
	PendingPath string
	BatchID     string
	DryRun      bool
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
	if item.Type == TypeInbox || item.TopicPath == "inbox" {
		return filepath.Join(opts.WikiRoot, "inbox")
	}

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

// WritePending writes a pending.md entry (TS pending.ts style).
func WritePending(item *ClassifyItem, opts *WriteOptions) (string, error) {
	pendingBase := opts.PendingPath
	if pendingBase == "" {
		pendingBase = "inbox/pending.md"
	}

	pendingPath := filepath.Join(opts.WikiRoot, pendingBase)

	if err := os.MkdirAll(filepath.Dir(pendingPath), fileutil.DirPerm); err != nil {
		return "", fmt.Errorf("create pending dir: %w", err)
	}

	title := item.Title
	if title == "" {
		title = item.URL
	}

	suggestedTopic := item.TopicPath
	if suggestedTopic == "" || suggestedTopic == noneVal {
		suggestedTopic = "(待补充)"
	}

	entry := fmt.Sprintf(`---

## [%s](%s)

%s

### 建议的 topic 路径 (未分类)
%s

`, title, item.URL, item.Summary, suggestedTopic)

	if opts.DryRun {
		slog.Info("[DRY RUN] Would append pending", "path", pendingPath)

		return pendingPath, nil
	}

	// Append to file
	f, err := os.OpenFile(pendingPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, fileutil.FilePermPrivate)
	if err != nil {
		return "", fmt.Errorf("open pending file: %w", err)
	}
	defer func() { _ = f.Close() }()

	if _, err := f.WriteString(entry); err != nil {
		return "", fmt.Errorf("write pending: %w", err)
	}

	slog.Info("Pending written", "path", pendingPath, "topic", suggestedTopic)

	return pendingPath, nil
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
		mdLinks := extractMarkdownLinks(line)
		for _, u := range mdLinks {
			if !seen[u] {
				seen[u] = true
				entries = append(entries, InboxEntry{URL: u, LineIndex: i})
			}
		}

		bareURLs := extractBareURLs(line)
		for _, u := range bareURLs {
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

// extractMarkdownLinks extracts URLs from markdown links [text](url).
func extractMarkdownLinks(line string) []string {
	var urls []string
	for {
		start := strings.Index(line, "](")
		if start < 0 {
			break
		}
		end := strings.Index(line[start+2:], ")")
		if end < 0 {
			break
		}
		url := line[start+2 : start+2+end]
		if strings.HasPrefix(url, "http") {
			urls = append(urls, url)
		}
		line = line[start+2+end+1:]
	}

	return urls
}

// extractBareURLs extracts bare HTTP URLs from a line.
func extractBareURLs(line string) []string {
	var urls []string
	words := strings.FieldsSeq(line)
	for w := range words {
		w = strings.Trim(w, `,.;:!?()[]{}"'<>`)
		if strings.HasPrefix(w, "http://") || strings.HasPrefix(w, "https://") {
			urls = append(urls, w)
		}
	}

	return urls
}
