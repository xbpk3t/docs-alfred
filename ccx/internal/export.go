package internal

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/xbpk3t/docs-alfred/pkg/ai"
	"github.com/xbpk3t/docs-alfred/pkg/cmdutil"
	"gopkg.in/yaml.v3"
)

const (
	roleUser      = "user"
	roleAssistant = "assistant"
)

// ExportInput contains inputs for session export.
type ExportInput struct {
	WikiRoot  string
	OutputDir string
	DryRun    bool
	Verbose   bool
}

// ExportResult contains the result of session export.
type ExportResult struct {
	OutputPath string
	TopicPath  string
	Title      string
	DryRun     bool
}

// SessionJSON represents the JSON structure from claude-extract.
type SessionJSON struct {
	SessionID    string    `json:"session_id"`
	Date         string    `json:"date"`
	Messages     []Message `json:"messages"`
	MessageCount int       `json:"message_count"`
}

// Message represents a single message in the conversation.
type Message struct {
	Role      string `json:"role"`
	Content   string `json:"content"`
	Timestamp string `json:"timestamp"`
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
	chain, err := WalkSessionChain()
	if err != nil {
		return nil, fmt.Errorf("walk session chain: %w", err)
	}

	if len(chain) == 0 {
		return nil, errors.New("no sessions found in chain")
	}

	if input.Verbose {
		fmt.Fprintf(os.Stderr, "Found %d sessions in chain\n", len(chain))
	}

	sessions, err := extractAllSessions(chain, input.Verbose)
	if err != nil {
		return nil, err
	}

	mergedMessages := mergeSessions(sessions)

	if input.Verbose {
		fmt.Fprintf(os.Stderr, "Merged %d messages\n", len(mergedMessages))
	}

	title, err := generateAITitle(mergedMessages)
	if err != nil {
		return nil, fmt.Errorf("generate title: %w", err)
	}

	if input.Verbose {
		fmt.Fprintf(os.Stderr, "Generated title: %s\n", title)
	}

	outputPath := determineOutputPath(input, title)

	if input.DryRun {
		return &ExportResult{
			OutputPath: outputPath,
			Title:      title,
			DryRun:     true,
		}, nil
	}

	if err := writeExportFile(outputPath, title, mergedMessages); err != nil {
		return nil, err
	}

	return &ExportResult{
		OutputPath: outputPath,
		Title:      title,
	}, nil
}

// writeExportFile writes the final markdown file.
func writeExportFile(outputPath, title string, messages []Message) error {
	frontmatter := generateFrontmatter(title)
	content := renderToMarkdown(messages)
	finalContent := frontmatter + content

	if err := os.MkdirAll(filepath.Dir(outputPath), 0750); err != nil {
		return fmt.Errorf("create output dir: %w", err)
	}

	if err := os.WriteFile(outputPath, []byte(finalContent), 0600); err != nil {
		return fmt.Errorf("write output file: %w", err)
	}

	return nil
}

// extractAllSessions extracts JSON for all sessions in the chain.
func extractAllSessions(chain []ChainRecord, verbose bool) ([]SessionJSON, error) {
	sessions := make([]SessionJSON, 0, len(chain))

	for _, entry := range chain {
		if verbose {
			fmt.Fprintf(os.Stderr, "Extracting session %s\n", entry.SessionID)
		}

		session, err := extractSessionJSON(entry.TranscriptPath)
		if err != nil {
			return nil, fmt.Errorf("extract session %s: %w", entry.SessionID, err)
		}

		sessions = append(sessions, *session)
	}

	return sessions, nil
}

// extractSessionJSON extracts a single session as JSON using claude-extract.
func extractSessionJSON(sessionPath string) (*SessionJSON, error) {
	if _, ok := cmdutil.LookPath("claude-extract"); !ok {
		return nil, errors.New("claude-extract not found (install with: uv tool install claude-conversation-extractor)")
	}

	tmpDir, err := os.MkdirTemp("", "ccx-session-*")
	if err != nil {
		return nil, fmt.Errorf("create temp dir: %w", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	if err := symlinkSessionFile(sessionPath, tmpDir); err != nil {
		return nil, err
	}

	if err := runClaudeExtract(tmpDir); err != nil {
		return nil, err
	}

	return findAndParseJSON(tmpDir)
}

// symlinkSessionFile creates a symlink to the session file in the temp directory.
func symlinkSessionFile(sessionPath, tmpDir string) error {
	err := os.Symlink(sessionPath, filepath.Join(tmpDir, filepath.Base(sessionPath)))
	if err != nil {
		return fmt.Errorf("symlink session file: %w", err)
	}

	return nil
}

// runClaudeExtract runs claude-extract to export the session as JSON.
func runClaudeExtract(tmpDir string) error {
	_, err := cmdutil.RunStdout(context.Background(), "claude-extract",
		"--extract", "1",
		"--output", tmpDir,
		"--format", "json",
	)
	if err != nil {
		return fmt.Errorf("claude-extract: %w", err)
	}

	return nil
}

// findAndParseJSON finds the JSON file in the directory and parses it.
func findAndParseJSON(tmpDir string) (*SessionJSON, error) {
	entries, err := os.ReadDir(tmpDir)
	if err != nil {
		return nil, fmt.Errorf("read output dir: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		return parseJSONFile(filepath.Join(tmpDir, entry.Name()))
	}

	return nil, errors.New("no JSON file found in output")
}

// parseJSONFile reads and parses a JSON file.
func parseJSONFile(filePath string) (*SessionJSON, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("read output file: %w", err)
	}

	var session SessionJSON
	if err := json.Unmarshal(data, &session); err != nil {
		return nil, fmt.Errorf("parse JSON: %w", err)
	}

	return &session, nil
}

// filterMessages filters messages to only include non-empty user/assistant messages.
func filterMessages(messages []Message) []Message {
	filtered := make([]Message, 0, len(messages))

	for _, msg := range messages {
		// Skip empty messages
		if strings.TrimSpace(msg.Content) == "" {
			continue
		}

		// Only keep user/assistant
		if msg.Role == roleUser || msg.Role == roleAssistant {
			filtered = append(filtered, msg)
		}
	}

	return filtered
}

// mergeSessions merges multiple sessions into a single message list.
func mergeSessions(sessions []SessionJSON) []Message {
	var allMessages []Message

	for _, s := range sessions {
		filtered := filterMessages(s.Messages)
		allMessages = append(allMessages, filtered...)
	}

	return allMessages
}

// generateAITitle generates a semantic title using AI.
func generateAITitle(messages []Message) (string, error) {
	// Extract user messages
	var userMessages []string
	for _, msg := range messages {
		if msg.Role == roleUser {
			userMessages = append(userMessages, msg.Content)
		}
	}

	if len(userMessages) == 0 {
		return time.Now().Format("2006-01-02-15-04-05"), nil
	}

	// Select: first 3 + last 1
	selected := selectMessages(userMessages, 3, 1)
	content := strings.Join(selected, "\n\n")

	// Truncate to avoid excessive tokens
	if len(content) > 300 {
		content = content[:300]
	}

	// AI call
	aiConfig := &ai.ClientConfig{
		APIKey:  os.Getenv("OPENAI_API_KEY"),
		BaseURL: "https://api.lucc.dev/v1",
		Model:   "deepseek-v4-flash",
	}

	systemMsg := "Generate a short semantic title (max 50 chars) for this conversation. Output ONLY the title, nothing else. No quotes, no punctuation, no explanation. Just the title."
	userMsg := "User messages:\n" + content

	aiMessages := []ai.Message{
		{Role: "system", Content: systemMsg},
		{Role: roleUser, Content: userMsg},
	}

	title, err := ai.ChatContext(context.Background(), aiConfig, aiMessages)
	if err != nil {
		return "", err
	}

	// Clean up and limit length
	title = sanitizeFilename(strings.TrimSpace(title))
	if len(title) > 50 {
		title = title[:50]
	}

	return title, nil
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

// generateFrontmatter generates YAML frontmatter for the wiki file.
func generateFrontmatter(title string) string {
	fm := Frontmatter{
		Type:   "research",
		Title:  title,
		Date:   time.Now().Format("2006-01-02"),
		Source: "claude-code",
	}

	data, _ := yaml.Marshal(fm)

	return "---\n" + string(data) + "---\n\n"
}

// renderToMarkdown renders messages to markdown format.
func renderToMarkdown(messages []Message) string {
	var sb strings.Builder

	for _, msg := range messages {
		// Add header
		if msg.Role == roleUser {
			sb.WriteString("## User\n\n")
		} else {
			sb.WriteString("## Claude\n\n")
		}

		// Add content
		sb.WriteString(msg.Content)
		sb.WriteString("\n\n---\n\n")
	}

	return sb.String()
}

// determineOutputPath determines the output path for the exported session.
func determineOutputPath(input ExportInput, title string) string {
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

	return filepath.Join(baseDir, filename)
}

// sanitizeFilename sanitizes a string for use as a filename.
func sanitizeFilename(s string) string {
	// Replace spaces with hyphens
	s = strings.ReplaceAll(s, " ", "-")

	// Remove special characters
	s = strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			return r
		}

		return -1
	}, s)

	// Collapse multiple hyphens
	for strings.Contains(s, "--") {
		s = strings.ReplaceAll(s, "--", "-")
	}

	// Trim hyphens from ends
	s = strings.Trim(s, "-")

	return s
}
