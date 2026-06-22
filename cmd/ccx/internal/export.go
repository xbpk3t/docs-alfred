package internal

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	wikisvc "github.com/xbpk3t/docs-alfred/internal/docs/wiki"
	session "github.com/xbpk3t/docs-alfred/pkg/ai/session"
	"github.com/xbpk3t/docs-alfred/pkg/ai"
	"github.com/xbpk3t/docs-alfred/pkg/textutil"
	"gopkg.in/yaml.v3"
)

const (
	roleUser = "user"
)

// ExportInput contains inputs for session export.
type ExportInput struct {
	AIConfig  *ai.ClientConfig
	WikiRoot  string
	OutputDir string
	SessionID string // Explicit session ID; overrides CLAUDE_CODE_SESSION_ID env var.
	DryRun    bool
	Verbose   bool
}

// ExportResult contains the result of session export.
type ExportResult struct {
	OutputPath string
	TopicPath  string
	Title      string // Original AI-generated title (may contain Chinese)
	EngTitle   string // English ASCII-safe title for filename
	DryRun     bool
}

// Frontmatter represents the YAML frontmatter for wiki files.
type Frontmatter struct {
	Type   string `yaml:"type"`
	Title  string `yaml:"title"`
	Date   string `yaml:"date"`
	Source string `yaml:"source"`
}

// ExportSession exports the current session to wiki.
func ExportSession(input ExportInput) (*ExportResult, error) {
	if err := validateExportInput(input); err != nil {
		return nil, err
	}

	chain, err := WalkSessionChain(input.SessionID)
	if err != nil {
		return nil, fmt.Errorf("walk session chain: %w", err)
	}

	if len(chain) == 0 {
		return nil, errors.New("no sessions found in chain")
	}

	if input.Verbose {
		fmt.Fprintf(os.Stderr, "Found %d sessions in chain\n", len(chain))
	}

	// Parse all sessions directly from JSONL (no claude-extract needed)
	messages, err := extractAndMergeSessions(chain, input.Verbose)
	if err != nil {
		return nil, err
	}

	if len(messages) == 0 {
		return nil, errors.New("no messages found after parsing and filtering")
	}

	topicPath, title, engTitle := classifyAndGenerateTitle(messages, input)
	outputPath := determineOutputPath(input, engTitle, topicPath)

	if input.Verbose {
		fmt.Fprintf(os.Stderr, "Generated title: %s\n", title)
		fmt.Fprintf(os.Stderr, "Generated engTitle: %s\n", engTitle)
		fmt.Fprintf(os.Stderr, "Topic path: %s\n", topicPath)
	}

	if input.DryRun {
		return &ExportResult{
			OutputPath: outputPath,
			TopicPath:  topicPath,
			Title:      title,
			EngTitle:   engTitle,
			DryRun:     true,
		}, nil
	}

	if err := writeExportFile(outputPath, title, messages); err != nil {
		return nil, err
	}

	return &ExportResult{
		OutputPath: outputPath,
		TopicPath:  topicPath,
		Title:      title,
		EngTitle:   engTitle,
	}, nil
}

// validateExportInput validates the export input parameters.
func validateExportInput(input ExportInput) error {
	if input.OutputDir != "" {
		return nil
	}

	wikiRoot := input.WikiRoot
	if !filepath.IsAbs(wikiRoot) {
		wikiRoot = filepath.Join(getProjectDir(), wikiRoot)
	}

	if _, err := os.Stat(wikiRoot); os.IsNotExist(err) {
		return fmt.Errorf("wiki-root does not exist: %s", input.WikiRoot)
	}

	return nil
}

// extractAndMergeSessions parses all session JSONL files for the chain,
// filters out noise, and returns the clean messages.
// This replaces the old claude-extract pipeline.
func extractAndMergeSessions(chain []ChainRecord, verbose bool) ([]session.Message, error) {
	// Build list of transcript paths from the chain
	paths := make([]string, len(chain))
	for i, c := range chain {
		paths[i] = c.TranscriptPath
	}

	// Parse all sessions directly from JSONL (no external tool needed)
	messages, err := session.ParseAll(paths)
	if err != nil {
		return nil, fmt.Errorf("parse sessions: %w", err)
	}

	if verbose {
		fmt.Fprintf(os.Stderr, "Parsed %d raw messages\n", len(messages))
	}

	// Filter out noise (API errors, transition sentences, emoji, etc.)
	messages = session.Filter(messages)

	if verbose {
		fmt.Fprintf(os.Stderr, "After filter: %d messages\n", len(messages))
	}

	return messages, nil
}

// classifyAndGenerateTitle classifies content and generates AI title + engTitle.
func classifyAndGenerateTitle(messages []session.Message, input ExportInput) (string, string, string) {
	// Build plain text for AI classification (Turn formatting is visual noise for AI)
	content := messagesToPlainText(messages)
	topicPath := classifyWithFallback(content, input.WikiRoot, input.Verbose, input.AIConfig)

	title, engTitle, err := generateTitles(messages, input.AIConfig)
	if err != nil {
		fallback := fallbackTitle(extractUserMessages(messages))

		return topicPath, fallback, fallback
	}

	return topicPath, title, engTitle
}

// messagesToPlainText joins message contents into a simple text representation
// for AI classification, without any Turn formatting.
func messagesToPlainText(messages []session.Message) string {
	var sb strings.Builder
	for _, msg := range messages {
		sb.WriteString(msg.Content)
		sb.WriteString("\n")
	}

	return sb.String()
}

// extractUserMessages extracts user messages from the message list.
func extractUserMessages(messages []session.Message) []string {
	var userMessages []string
	for _, msg := range messages {
		if msg.Role == roleUser {
			userMessages = append(userMessages, msg.Content)
		}
	}

	return userMessages
}

// writeExportFile writes the final markdown file with Turn-structured formatting.
func writeExportFile(outputPath, title string, messages []session.Message) error {
	frontmatter := generateFrontmatter(title)
	body := session.FormatMessages(messages)
	finalContent := frontmatter + body

	if err := os.MkdirAll(filepath.Dir(outputPath), 0750); err != nil {
		return fmt.Errorf("create output dir: %w", err)
	}

	if err := os.WriteFile(outputPath, []byte(finalContent), 0600); err != nil {
		return fmt.Errorf("write output file: %w", err)
	}

	return nil
}

// generateFrontmatter generates YAML frontmatter for the wiki file.
func generateFrontmatter(title string) string {
	fm := Frontmatter{
		Type:   "research",
		Title:  title,
		Date:   time.Now().Format("2006-01-02"),
		Source: "claude-code",
	}

	data, err := yaml.Marshal(fm)
	if err != nil {
		slog.Warn("Failed to marshal frontmatter, using fallback", "error", err)

		return "---\n---\n\n"
	}

	return "---\n" + string(data) + "---\n\n"
}

// generateTitles generates a semantic title and an English filename-safe slug using AI.
// Returns (title, engTitle, error).
func generateTitles(messages []session.Message, aiConfig *ai.ClientConfig) (string, string, error) {
	userMessages := extractUserMessages(messages)

	if len(userMessages) == 0 {
		ts := time.Now().Format("2006-01-02-15-04-05")

		return ts, ts, nil
	}

	// Select: first 3 + last 1
	selected := selectMessages(userMessages, 3, 1)
	content := strings.Join(selected, "\n\n")

	// Truncate to avoid excessive tokens
	if len(content) > 200 {
		content = content[:200]
	}

	systemMsg := `Output exactly two lines about this conversation:
TITLE: a short semantic title of the topic (any language, max 50 chars)
ENGLISH-TITLE: a short ASCII-only filename-safe slug (English, lowercase, hyphen-separated, max 50 chars)

Example:
TITLE: 用 Go 重写用户认证模块
ENGLISH-TITLE: go-user-auth-rewrite`
	userMsg := "User messages:\n" + content

	aiMessages := []ai.Message{
		{Role: "system", Content: systemMsg},
		{Role: roleUser, Content: userMsg},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	response, err := ai.ChatContext(ctx, aiConfig, aiMessages)
	if err != nil {
		return "", "", fmt.Errorf("AI title generation: %w", err)
	}

	title, engTitle := parseTitles(response)
	trimmedTitle := trimTitle(title)
	trimmedEng := trimEngTitle(engTitle)

	// If either is empty after trimming, fall back
	if trimmedTitle == "" || trimmedEng == "" {
		fallback := fallbackTitle(userMessages)

		return fallback, fallback, nil
	}

	return trimmedTitle, trimmedEng, nil
}

// parseTitles extracts TITLE and ENGLISH-TITLE from AI response.
func parseTitles(response string) (string, string) {
	title := ""
	engTitle := ""

	for line := range strings.SplitSeq(response, "\n") {
		trimmed := strings.TrimSpace(line)
		if after, ok := strings.CutPrefix(trimmed, "TITLE:"); ok {
			title = strings.TrimSpace(after)
		} else if after, ok := strings.CutPrefix(trimmed, "ENGLISH-TITLE:"); ok {
			engTitle = strings.TrimSpace(after)
		}
	}

	return title, engTitle
}

// trimTitle cleans up a semantic title (remove quotes, truncate to 50).
func trimTitle(title string) string {
	title = strings.TrimSpace(title)
	title = strings.Trim(title, `"'「」『』`)
	if len(title) > 50 {
		title = title[:50]
	}

	return title
}

// trimEngTitle cleans up an English title for use as filename.
func trimEngTitle(engTitle string) string {
	engTitle = strings.TrimSpace(engTitle)
	engTitle = strings.Trim(engTitle, `"'「」『』`)
	// Apply slugification for safety (gosimple/slug will not transliterate
	// if input is already ASCII, serving as a sanitizer)
	engTitle = slugTitle(engTitle)
	if len(engTitle) > 50 {
		engTitle = engTitle[:50]
	}

	return engTitle
}

// fallbackTitle generates a fallback title from the first user message.
func fallbackTitle(userMessages []string) string {
	if len(userMessages) == 0 {
		return time.Now().Format("2006-01-02-15-04-05")
	}

	// Use first 50 chars of first message
	title := userMessages[0]
	title = strings.TrimSpace(title)
	// Take first line only
	if idx := strings.IndexByte(title, '\n'); idx > 0 {
		title = title[:idx]
	}
	title = slugTitle(title)
	if len(title) > 50 {
		title = title[:50]
	}
	if title == "" {
		return time.Now().Format("2006-01-02-15-04-05")
	}

	return title
}

// selectMessages selects head messages from the beginning and tail messages from the end.
func selectMessages(messages []string, head, tail int) []string {
	if len(messages) <= head+tail {
		return messages
	}

	result := make([]string, 0, head+tail)
	result = append(result, messages[:head]...)
	result = append(result, messages[len(messages)-tail:]...)

	return result
}

// classifyWithFallback classifies content and returns empty string on failure.
func classifyWithFallback(content, wikiRoot string, verbose bool, aiConfig *ai.ClientConfig) string {
	topicPath, err := wikisvc.ClassifyContent(content, wikiRoot, aiConfig)
	if err != nil {
		if verbose {
			fmt.Fprintf(os.Stderr, "Classification failed: %v\n", err)
		}

		return ""
	}

	return topicPath
}

// determineOutputPath determines the output path for the exported session.
func determineOutputPath(input ExportInput, title, topicPath string) string {
	// Generate filename
	date := time.Now().Format("2006-01-02")
	filename := fmt.Sprintf("%s-%s.md", date, title)

	// Determine base directory
	baseDir := input.WikiRoot
	if input.OutputDir != "" {
		baseDir = input.OutputDir
	}

	// If relative path, make it relative to project root
	if !filepath.IsAbs(baseDir) {
		projectDir := getProjectDir()
		baseDir = filepath.Join(projectDir, baseDir)
	}

	// Add topic path if available
	if topicPath != "" {
		baseDir = filepath.Join(baseDir, topicPath)
	}

	return filepath.Join(baseDir, filename)
}

func slugTitle(value string) string {
	return textutil.SlugFilename(value)
}
