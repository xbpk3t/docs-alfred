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
	"unicode"

	"github.com/xbpk3t/docs-alfred/pkg/ai"
	"github.com/xbpk3t/docs-alfred/pkg/cmdutil"
	"github.com/xbpk3t/docs-alfred/pkg/textutil"
	wikisvc "github.com/xbpk3t/docs-alfred/service/wiki"
	"gopkg.in/yaml.v3"
)

const (
	roleUser      = "user"
	roleAssistant = "assistant"
)

// ExportInput contains inputs for session export.
type ExportInput struct {
	AIConfig  *ai.ClientConfig
	WikiRoot  string
	OutputDir string
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
	if err := validateExportInput(input); err != nil {
		return nil, err
	}

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

	mergedMessages, err := extractAndMergeSessions(chain, input.Verbose)
	if err != nil {
		return nil, err
	}

	topicPath, title, engTitle := classifyAndGenerateTitle(mergedMessages, input)
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

	if err := writeExportFile(outputPath, title, mergedMessages); err != nil {
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

// extractAndMergeSessions extracts and merges all sessions in the chain.
func extractAndMergeSessions(chain []ChainRecord, verbose bool) ([]Message, error) {
	sessions, err := extractAllSessions(chain, verbose)
	if err != nil {
		return nil, err
	}

	mergedMessages := mergeSessions(sessions)

	if verbose {
		fmt.Fprintf(os.Stderr, "Merged %d messages\n", len(mergedMessages))
	}

	return mergedMessages, nil
}

// classifyAndGenerateTitle classifies content and generates AI title + engTitle.
func classifyAndGenerateTitle(messages []Message, input ExportInput) (string, string, string) {
	content := renderToMarkdown(messages)
	topicPath := classifyWithFallback(content, input.WikiRoot, input.Verbose, input.AIConfig)

	title, engTitle, err := generateTitles(messages, input.AIConfig)
	if err != nil {
		fallback := fallbackTitle(extractUserMessages(messages))

		return topicPath, fallback, fallback
	}

	return topicPath, title, engTitle
}

// extractUserMessages extracts user messages from the message list.
func extractUserMessages(messages []Message) []string {
	var userMessages []string
	for _, msg := range messages {
		if msg.Role == roleUser {
			userMessages = append(userMessages, msg.Content)
		}
	}

	return userMessages
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
		if msg.Role != roleUser && msg.Role != roleAssistant {
			continue
		}

		// Clean up system-generated wrapper content
		content := unwrapSystemContent(msg.Content)
		if content == "" {
			continue
		}

		msg.Content = content
		filtered = append(filtered, msg)
	}

	return filtered
}

// unwrapSystemContent extracts meaningful content from system-wrapped messages.
// Returns empty string if the message is purely system noise.
func unwrapSystemContent(content string) string {
	// Skip pure system output injections
	if strings.Contains(content, "<local-command-stdout>") {
		return ""
	}
	if strings.Contains(content, "session-scoped Stop hook") {
		return ""
	}

	// Extract content from <command-name> wrappers (user's actual input)
	if strings.Contains(content, "<command-name>") {
		return extractCommandContent(content)
	}

	return content
}

// extractCommandContent extracts the meaningful content from a command-wrapped message.
func extractCommandContent(content string) string {
	// Extract content from <command-args>...</command-args> if present
	if start := strings.Index(content, "<command-args>"); start >= 0 {
		start += len("<command-args>")
		if end := strings.Index(content, "</command-args>"); end > start {
			return strings.TrimSpace(content[start:end])
		}
	}

	// Fallback: extract from <command-message>...</command-message>
	if start := strings.Index(content, "<command-message>"); start >= 0 {
		start += len("<command-message>")
		if end := strings.Index(content, "</command-message>"); end > start {
			return strings.TrimSpace(content[start:end])
		}
	}

	return ""
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

// generateTitles generates a semantic title and an English filename-safe slug using AI.
// Returns (title, engTitle, error).
func generateTitles(messages []Message, aiConfig *ai.ClientConfig) (string, string, error) {
	// Extract user messages
	var userMessages []string
	for _, msg := range messages {
		if msg.Role == roleUser {
			userMessages = append(userMessages, msg.Content)
		}
	}

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

		// Sanitize content: remove emoji, strip embedded section headers
		content := sanitizeContent(msg.Content)
		sb.WriteString(content)
		sb.WriteString("\n\n---\n\n")
	}

	return sb.String()
}

// sanitizeContent removes emoji and embedded section headers from message content.
func sanitizeContent(content string) string {
	// Remove emoji characters (Unicode category So, Sk, and supplementary planes)
	content = removeEmoji(content)

	// Strip embedded section headers (## ...) that come from assistant's markdown
	// These would conflict with the ## User / ## Claude headers
	lines := strings.Split(content, "\n")
	var filtered []string

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		// Skip lines that look like section headers (## ...) but not code blocks
		if strings.HasPrefix(trimmed, "## ") && !strings.HasPrefix(trimmed, "## User") && !strings.HasPrefix(trimmed, "## Claude") {
			// Replace with the text content only (remove the ## prefix)
			headerText := strings.TrimPrefix(trimmed, "## ")
			filtered = append(filtered, headerText)

			continue
		}
		filtered = append(filtered, line)
	}

	result := strings.Join(filtered, "\n")

	// Collapse multiple spaces on each line (from emoji removal)
	collapsed := make([]string, 0, len(filtered))
	for line := range strings.SplitSeq(result, "\n") {
		// Collapse 2+ spaces into 1
		for strings.Contains(line, "  ") {
			line = strings.ReplaceAll(line, "  ", " ")
		}
		collapsed = append(collapsed, line)
	}

	return strings.Join(collapsed, "\n")
}

// removeEmoji removes emoji characters from a string.
func removeEmoji(s string) string {
	return strings.Map(func(r rune) rune {
		// Keep normal ASCII and common punctuation (including backtick U+0060)
		if r < 0x2600 {
			return r
		}
		// Remove variation selectors (U+FE00-U+FE0F)
		if r >= 0xFE00 && r <= 0xFE0F {
			return -1
		}
		// Remove emoji and symbol ranges (but not Sk which includes backtick)
		if unicode.Is(unicode.So, r) {
			return -1
		}
		// Remove supplemental symbols (0x1F000+)
		if r >= 0x1F000 {
			return -1
		}
		// Remove dingbats (0x2700-0x27BF)
		if r >= 0x2700 && r <= 0x27BF {
			return -1
		}
		// Remove miscellaneous symbols (0x2600-0x26FF)
		if r >= 0x2600 && r <= 0x26FF {
			return -1
		}
		// Keep everything else (CJK, Latin extended, etc.)
		return r
	}, s)
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

func slugTitle(value string) string {
	return textutil.SlugFilename(value)
}
