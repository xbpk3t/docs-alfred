package internal

import (
	"bytes"
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	wikisvc "github.com/xbpk3t/docs-alfred/internal/docs/wiki"
	ghindex "github.com/xbpk3t/docs-alfred/internal/gh/index"
	"github.com/xbpk3t/docs-alfred/pkg/ai"
	session "github.com/xbpk3t/docs-alfred/pkg/ai/session"
	"github.com/xbpk3t/docs-alfred/pkg/textutil"
	"github.com/xbpk3t/docs-alfred/pkg/validator"
	"gopkg.in/yaml.v3"
)

//go:embed prompts/*.txt
var promptFS embed.FS

const (
	roleUser = "user"

	// mergedAITimeout is the timeout for the single merged AI call
	// that handles both classification and title generation.
	mergedAITimeout = 200 * time.Second
)

// ExportInput contains inputs for session export.
type ExportInput struct {
	AIConfig  *ai.ClientConfig
	Agent     Agent `validate:"required|in:cc,codex"`
	WikiRoot  string
	OutputDir string
	ProjectDir string // Resolved project directory; set once by CLI layer.
	SessionID string // Explicit session/thread ID; defaults to the selected agent env var.
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
func ExportSession(input *ExportInput) (*ExportResult, error) {
	if err := validateExportInput(input); err != nil {
		return nil, err
	}

	resolved, err := ResolveSession(input.Agent, input.SessionID, input.ProjectDir)
	if err != nil {
		return nil, fmt.Errorf("resolve session: %w", err)
	}

	if input.Verbose {
		fmt.Fprintf(os.Stderr, "Resolved %s session %s\n", resolved.Agent, resolved.SessionID)
		fmt.Fprintf(os.Stderr, "Transcript: %s\n", resolved.TranscriptPath)
	}

	messages, err := parseResolvedSession(resolved, input.Verbose)
	if err != nil {
		return nil, err
	}

	if len(messages) == 0 {
		return nil, errors.New("no messages found after parsing and filtering")
	}

	topicPath, title, engTitle, err := classifyAndGenerateTitle(messages, input)
	if err != nil {
		return nil, fmt.Errorf("classify: %w", err)
	}
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

	if err := writeExportFile(outputPath, title, resolved.Source, messages); err != nil {
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
func validateExportInput(input *ExportInput) error {
	if input == nil {
		return errors.New("export input is nil")
	}

	if err := validator.Struct(input); err != nil {
		return err
	}

	if input.OutputDir != "" {
		return nil
	}

	wikiRoot := input.WikiRoot
	// WikiRoot is always absolute — canonicalized at config-load time.

	if _, err := os.Stat(wikiRoot); os.IsNotExist(err) {
		return fmt.Errorf("wiki-root does not exist: %s", input.WikiRoot)
	}

	return nil
}

func parseResolvedSession(resolved SessionRef, verbose bool) ([]session.Message, error) {
	messages, err := parseTranscript(resolved)
	if err != nil {
		return nil, err
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

func parseTranscript(resolved SessionRef) ([]session.Message, error) {
	switch resolved.Agent {
	case AgentCC:
		messages, err := session.Parse(resolved.TranscriptPath)
		if err != nil {
			return nil, fmt.Errorf("parse cc session: %w", err)
		}

		return messages, nil
	case AgentCodex:
		messages, err := session.ParseCodex(resolved.TranscriptPath)
		if err != nil {
			return nil, fmt.Errorf("parse codex session: %w", err)
		}

		return messages, nil
	default:
		return nil, fmt.Errorf("unsupported agent %q", resolved.Agent)
	}
}

// classifyAndGenerateTitle performs a single AI call to classify content and generate titles.
// Returns an error when the AI call fails or the topic path is invalid.
func classifyAndGenerateTitle(messages []session.Message, input *ExportInput) (string, string, string, error) {
	topicPath, title, engTitle, err := mergedClassifyAndTitle(messages, input)
	if err != nil {
		return "", "", "", fmt.Errorf("classify and title: %w", err)
	}

	return topicPath, title, engTitle, nil
}

// mergedClassifyAndTitle makes a single AI call to determine topicPath, title, and engTitle.
func mergedClassifyAndTitle(messages []session.Message, input *ExportInput) (string, string, string, error) {
	prompt, candidates, err := renderClassifyTitlePrompt(messages)
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
	topicPath, err := normalizeTopicPath(input.WikiRoot, result.TopicPath, candidates)
	if err != nil {
		return "", "", "", err
	}

	return topicPath, title, engTitle, nil
}

func normalizeTopicPath(wikiRoot, topicPath string, candidates []ghindex.TopicCandidate) (string, error) {
	topicPath = strings.TrimSpace(topicPath)
	if topicPath == "" || topicPath == "none" || topicPath == "inbox" {
		return "", fmt.Errorf("AI returned unresolvable topic path %q", topicPath)
	}
	if err := wikisvc.ValidateRelativeWikiPath(wikiRoot, topicPath); err != nil {
		return "", fmt.Errorf("AI topic path is unsafe: %w", err)
	}
	if !hasTopicCandidate(candidates, topicPath) {
		return "", fmt.Errorf("AI topic path %q not found in topic candidates", topicPath)
	}
	if strings.Count(topicPath, "/") != 2 {
		return "", fmt.Errorf("AI topic path %q has unsupported depth", topicPath)
	}

	return topicPath, nil
}

func hasTopicCandidate(candidates []ghindex.TopicCandidate, topicPath string) bool {
	for _, candidate := range candidates {
		if candidate.Path == topicPath {
			return true
		}
	}

	return false
}

// renderClassifyTitlePrompt renders the classify-title.txt prompt template.
// Returns the rendered prompt and topic candidates for validation.
func renderClassifyTitlePrompt(messages []session.Message) (string, []ghindex.TopicCandidate, error) {
	candidates := wikisvc.LoadClassificationCandidates("")
	if len(candidates) == 0 {
		return "", nil, errors.New("no topic candidates available")
	}

	tmpl, err := template.New("classify-title.txt").
		Option("missingkey=error").
		ParseFS(promptFS, "prompts/classify-title.txt")
	if err != nil {
		return "", nil, fmt.Errorf("parse prompt: %w", err)
	}

	userMessages := extractUserMessages(messages)
	content := strings.Join(selectMessages(userMessages, 3, 1), "\n\n")
	if len(content) > 2000 {
		content = content[:2000]
	}

	data := map[string]string{
		"CandidateTree": wikisvc.FormatTopicCandidatesGrouped(candidates),
		"Content":       content,
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", nil, fmt.Errorf("render prompt: %w", err)
	}

	return buf.String(), candidates, nil
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
// If head+tail exceeds len(messages), returns all messages without duplication.
func selectMessages(messages []string, head, tail int) []string {
	if head < 0 {
		head = 0
	}
	if tail < 0 {
		tail = 0
	}
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

	return truncateRunes(title, 50)
}

// trimEngTitle cleans up an English title for use as filename.
func trimEngTitle(engTitle string) string {
	engTitle = strings.TrimSpace(engTitle)
	engTitle = strings.Trim(engTitle, `"'「」『』`)
	engTitle = slugTitle(engTitle)
	return truncateRunes(engTitle, 50)
}

// writeExportFile writes the final markdown file with Turn-structured formatting.
func writeExportFile(outputPath, title, source string, messages []session.Message) error {
	frontmatter, err := generateFrontmatter(title, source)
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
func generateFrontmatter(title, source string) (string, error) {
	fm := Frontmatter{
		Type:   "research",
		Title:  title,
		Date:   time.Now().Format("2006-01-02"),
		Source: source,
	}

	data, err := yaml.Marshal(fm)
	if err != nil {
		return "", fmt.Errorf("marshal frontmatter: %w", err)
	}

	return "---\n" + string(data) + "---\n\n", nil
}

// determineOutputPath determines the output path for the exported session.
func determineOutputPath(input *ExportInput, title, topicPath string) string {
	// Generate filename
	date := time.Now().Format("2006-01-02")
	filename := fmt.Sprintf("%s-%s.md", date, title)

	// Determine base directory
	// WikiRoot is always absolute — canonicalized at config-load time.
	baseDir := input.WikiRoot
	if input.OutputDir != "" {
		baseDir = input.OutputDir
		// Only OutputDir may be relative; resolve it against project root.
		if !filepath.IsAbs(baseDir) {
			baseDir = filepath.Join(input.ProjectDir, baseDir)
		}
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

func truncateRunes(value string, limit int) string {
	if limit <= 0 {
		return ""
	}

	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}

	return string(runes[:limit])
}
