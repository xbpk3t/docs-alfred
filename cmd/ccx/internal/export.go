package internal

import (
	"bytes"
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	ghindex "github.com/xbpk3t/docs-alfred/internal/gh/index"
	wikisvc "github.com/xbpk3t/docs-alfred/internal/docs/wiki"
	session "github.com/xbpk3t/docs-alfred/pkg/ai/session"
	"github.com/xbpk3t/docs-alfred/pkg/ai"
	"github.com/xbpk3t/docs-alfred/pkg/textutil"
	"gopkg.in/yaml.v3"
)

//go:embed prompts/*.txt
var promptFS embed.FS

const (
	roleUser = "user"

	// mergedAITimeout is the timeout for the single merged AI call
	// that handles both classification and title generation.
	mergedAITimeout = 100 * time.Second
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

// classifyTitleResult is the JSON response from the merged classify+title AI call.
type classifyTitleResult struct {
	TopicPath string `json:"topicPath"`
	Title     string `json:"title"`
	EngTitle  string `json:"engTitle"`
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

// classifyAndGenerateTitle performs a single AI call to classify content and generate titles.
// Falls back to empty topicPath + fallbackTitle on failure.
func classifyAndGenerateTitle(messages []session.Message, input ExportInput) (string, string, string) {
	topicPath, title, engTitle, err := mergedClassifyAndTitle(messages, input)
	if err != nil {
		slog.Warn("Merged AI call failed, using fallback", "error", err)
		fallback := fallbackTitle(extractUserMessages(messages))

		return "", fallback, fallback
	}

	return topicPath, title, engTitle
}

// mergedClassifyAndTitle makes a single AI call to determine topicPath, title, and engTitle.
func mergedClassifyAndTitle(messages []session.Message, input ExportInput) (string, string, string, error) {
	candidates := wikisvc.LoadClassificationCandidates(input.WikiRoot)
	if len(candidates) == 0 {
		return "", "", "", errors.New("no topic candidates available")
	}

	prompt, err := renderClassifyTitlePrompt(candidates, messages)
	if err != nil {
		return "", "", "", fmt.Errorf("render prompt: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), mergedAITimeout)
	defer cancel()

	response, err := ai.ChatContext(ctx, input.AIConfig, []ai.Message{
		{Role: roleUser, Content: prompt},
	})
	if err != nil {
		return "", "", "", fmt.Errorf("AI call: %w", err)
	}

	result, err := parseClassifyTitleResult(response)
	if err != nil {
		return "", "", "", fmt.Errorf("parse AI response: %w", err)
	}

	title := trimTitle(result.Title)
	engTitle := trimEngTitle(result.EngTitle)
	if title == "" || engTitle == "" {
		return "", "", "", errors.New("empty title from AI")
	}

	return result.TopicPath, title, engTitle, nil
}

// renderClassifyTitlePrompt renders the classify-title.txt prompt template.
func renderClassifyTitlePrompt(candidates []ghindex.TopicCandidate, messages []session.Message) (string, error) {
	tmpl, err := template.New("classify-title.txt").
		Option("missingkey=error").
		ParseFS(promptFS, "prompts/classify-title.txt")
	if err != nil {
		return "", fmt.Errorf("parse prompt: %w", err)
	}

	userMessages := extractUserMessages(messages)
	content := strings.Join(selectMessages(userMessages, 3, 1), "\n\n")
	if len(content) > 2000 {
		content = content[:2000]
	}

	data := map[string]string{
		"CandidateTree": wikisvc.FormatTopicCandidates(candidates),
		"Content":       content,
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("render prompt: %w", err)
	}

	return buf.String(), nil
}

// parseClassifyTitleResult parses the JSON response from the merged AI call.
func parseClassifyTitleResult(raw string) (*classifyTitleResult, error) {
	// Strip markdown code fence if present
	raw = strings.TrimSpace(raw)
	if strings.HasPrefix(raw, "```") {
		lines := strings.Split(raw, "\n")
		if len(lines) >= 3 {
			lines = lines[1 : len(lines)-1]
			raw = strings.Join(lines, "\n")
		}
	}

	var result classifyTitleResult
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		return nil, err
	}

	return &result, nil
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
	engTitle = slugTitle(engTitle)
	if len(engTitle) > 50 {
		engTitle = engTitle[:50]
	}

	return engTitle
}

// fallbackTitle generates a fallback title from user messages, skipping URLs.
func fallbackTitle(userMessages []string) string {
	for _, msg := range userMessages {
		title := strings.TrimSpace(msg)
		if idx := strings.IndexByte(title, '\n'); idx > 0 {
			title = title[:idx]
		}

		// Skip messages that are just URLs
		lower := strings.ToLower(title)
		if strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://") {
			continue
		}

		title = slugTitle(title)
		if len(title) > 50 {
			title = title[:50]
		}
		if title != "" {
			return title
		}
	}

	return time.Now().Format("2006-01-02-15-04-05")
}

// writeExportFile writes the final markdown file with Turn-structured formatting.
func writeExportFile(outputPath, title string, messages []session.Message) error {
	frontmatter, err := generateFrontmatter(title)
	if err != nil {
		return err
	}
	body := session.FormatMessages(messages)
	finalContent := frontmatter + body

	if err := os.MkdirAll(filepath.Dir(outputPath), 0o750); err != nil {
		return fmt.Errorf("create output dir: %w", err)
	}

	if err := os.WriteFile(outputPath, []byte(finalContent), 0o600); err != nil {
		return fmt.Errorf("write output file: %w", err)
	}

	return nil
}

// generateFrontmatter generates YAML frontmatter for the wiki file.
func generateFrontmatter(title string) (string, error) {
	fm := Frontmatter{
		Type:   "research",
		Title:  title,
		Date:   time.Now().Format("2006-01-02"),
		Source: "claude-code",
	}

	data, err := yaml.Marshal(fm)
	if err != nil {
		return "", fmt.Errorf("marshal frontmatter: %w", err)
	}

	return "---\n" + string(data) + "---\n\n", nil
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
