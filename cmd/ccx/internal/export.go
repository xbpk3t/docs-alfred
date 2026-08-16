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
	"unicode"

	wikiclassify "github.com/xbpk3t/docs-alfred/internal/docs/wiki/classify"
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
	AIConfig   *ai.ClientConfig
	Agent      Agent `validate:"required|in:cc,codex"`
	WikiRoot   string
	OutputDir  string
	ProjectDir string // Resolved project directory; set once by CLI layer.
	SessionID  string // Explicit session/thread ID; defaults to the selected agent env var.
	Issue      string // Optional issue URL (Linear/GitHub/...); omitted from frontmatter when empty.
	DryRun     bool
	Verbose    bool
}

// ExportResult contains the result of session export.
type ExportResult struct {
	OutputPath string
	TopicPath  string
	Title      string // Original AI-generated title (may contain Chinese)
	DryRun     bool
}

// Frontmatter represents the YAML frontmatter for wiki files.
type Frontmatter struct {
	Type    string `yaml:"type"`
	Title   string `yaml:"title"`
	Date    string `yaml:"date"`
	Source  string `yaml:"source"`
	Session string `yaml:"session"`
	Model   string `yaml:"model,omitempty"`
	Issue   string `yaml:"issue,omitempty"`
	// Score is a manual quality rating; always written (default 0).
	// No omitempty — zero must appear as score: 0.
	Score int `yaml:"score"`
}

// classifyTopicResult is the JSON response from the AI topic classification call.
type classifyTopicResult struct {
	TopicPath string `json:"topicPath"`
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

	messages, err := parseResolvedSession(&resolved, input.Verbose)
	if err != nil {
		return nil, err
	}

	if len(messages) == 0 {
		return nil, fmt.Errorf("no messages found after parsing and filtering: %w", ErrSessionEmpty)
	}

	// Extract model before the AI call so a rare IO/scan failure fails fast
	// without spending classify/title tokens. Missing model is not an error.
	model, err := extractPrimaryModel(&resolved)
	if err != nil {
		return nil, fmt.Errorf("extract model: %w", err)
	}
	if input.Verbose && model != "" {
		fmt.Fprintf(os.Stderr, "Primary model: %s\n", model)
	}

	topicPath, err := classifyTopicPath(messages, input)
	if err != nil {
		return nil, fmt.Errorf("classify: %w", err)
	}

	// Three-part title (display title, filename stem, frontmatter title) is
	// set from a single authoritative source: the agent's session name. AI
	// never decides the title. When the session name is missing (e.g. a short
	// cc session that was never renamed has no custom-title/ai-title event) the
	// export aborts — no fallback, so an unapproved title never reaches disk.
	if resolved.Title == "" {
		return nil, errors.New("session has no session name; cannot export without a title")
	}
	title := trimTitle(resolved.Title)
	filename := sanitizeFilename(title)
	outputPath := determineOutputPath(input, filename, topicPath)

	if input.Verbose {
		fmt.Fprintf(os.Stderr, "Generated title: %s\n", title)
		fmt.Fprintf(os.Stderr, "Generated filename: %s\n", filename)
		fmt.Fprintf(os.Stderr, "Topic path: %s\n", topicPath)
	}

	if input.DryRun {
		return &ExportResult{
			OutputPath: outputPath,
			TopicPath:  topicPath,
			Title:      title,
			DryRun:     true,
		}, nil
	}

	issue := strings.TrimSpace(input.Issue)
	if err := writeExportFile(outputPath, title, resolved.Source, resolved.SessionID, model, issue, messages); err != nil {
		return nil, err
	}

	return &ExportResult{
		OutputPath: outputPath,
		TopicPath:  topicPath,
		Title:      title,
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

func parseResolvedSession(resolved *SessionRef, verbose bool) ([]session.Message, error) {
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

func parseTranscript(resolved *SessionRef) ([]session.Message, error) {
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

// classifyTopicPath determines the topic path via AI. The AI has no role in
// title generation: the title derives from the session name (resolved.Title).
// An AI failure or unresolvable topic path maps to an empty topic path, which
// downstream code interprets as "write to wiki root directory".
func classifyTopicPath(messages []session.Message, input *ExportInput) (string, error) {
	if input.AIConfig == nil {
		slog.Warn("no AI config, exporting to wiki root")

		return "", nil
	}

	topicPath, err := mergedClassifyTopicPath(messages, input)
	if err != nil {
		slog.Warn("AI classification failed, exporting to wiki root", "error", err)

		return "", nil
	}

	return topicPath, nil
}

// trimTitle cleans up a semantic title (remove quotes, truncate to 50).
func trimTitle(title string) string {
	title = strings.TrimSpace(title)
	title = strings.Trim(title, `"'「」『』`)

	return truncateRunes(title, 50)
}

// sanitizeFilename turns a title into a filesystem-safe filename stem,
// preserving non-ASCII characters (Chinese titles keep their readability).
// Only characters that are invalid or dangerous in paths are replaced.
func sanitizeFilename(title string) string {
	// Replace characters invalid on common filesystems and path separators.
	replacer := strings.NewReplacer(
		`/`, " ", `\`, " ", ":", " ", "*", " ", `?`, " ", `"`, " ", "<", " ", ">", " ", "|", " ",
	)
	stem := replacer.Replace(title)

	// Collapse runs of spaces (which could hide a path traversal after
	// trimming) and strip leading/trailing whitespace and dots.
	fields := strings.Fields(stem)
	stem = strings.Join(fields, " ")
	stem = strings.TrimLeft(stem, ".")
	stem = strings.TrimSpace(stem)

	// Reserved names must not match a directory entry.
	if stem == "." || stem == ".." {
		return fallbackFilename
	}

	// Control characters break tooling and terminal output.
	var b strings.Builder
	for _, r := range stem {
		if unicode.IsControl(r) {
			continue
		}
		b.WriteRune(r)
	}
	stem = strings.TrimSpace(b.String())

	// Unicode titles that survive sanitization keep their characters; only
	// an empty result falls back to the ASCII-safe stem of the raw title.
	if stem == "" {
		stem = strings.Trim(textutil.SlugFilename(title), "-")
	}
	if stem == "" {
		return fallbackFilename
	}

	return truncateRunes(stem, 50)
}

// mergedClassifyTopicPath determines topicPath via a single AI call, retrying
// once on failure. The prompt forbids an empty topicPath, but a model may still
// emit an unresolvable value on an unlucky sample; a single retry absorbs that
// transient jitter without spending unbounded tokens.
func mergedClassifyTopicPath(messages []session.Message, input *ExportInput) (string, error) {
	prompt, candidates, err := renderClassifyTitlePrompt(messages, input.WikiRoot)
	if err != nil {
		return "", fmt.Errorf("render prompt: %w", err)
	}

	topicPath, err := classifyOnce(prompt, candidates, input)
	if err == nil {
		return topicPath, nil
	}

	slog.Warn("classification failed, retrying once", "error", err)
	topicPath, retryErr := classifyOnce(prompt, candidates, input)
	if retryErr != nil {
		return "", fmt.Errorf("classify retry: %w (first attempt: %w)", retryErr, err)
	}

	return topicPath, nil
}

// classifyOnce runs a single classification AI call and normalizes its result.
func classifyOnce(prompt string, candidates []ghindex.TopicCandidate, input *ExportInput) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), mergedAITimeout)
	defer cancel()

	response, err := ai.ChatContext(ctx, input.AIConfig, []ai.Message{
		{Role: roleUser, Content: prompt},
	})
	if err != nil {
		return "", fmt.Errorf("AI call: %w", err)
	}

	result, err := parseClassifyTopicResult(response)
	if err != nil {
		return "", fmt.Errorf("parse AI response: %w", err)
	}

	topicPath, err := normalizeTopicPath(input.WikiRoot, result.TopicPath, candidates)
	if err != nil {
		return "", err
	}

	return topicPath, nil
}

func normalizeTopicPath(wikiRoot, topicPath string, candidates []ghindex.TopicCandidate) (string, error) {
	topicPath = strings.TrimSpace(topicPath)
	if topicPath == "" || topicPath == "none" || topicPath == "inbox" {
		return "", fmt.Errorf("AI returned unresolvable topic path %q", topicPath)
	}
	if err := wikiclassify.ValidateRelativeWikiPath(wikiRoot, topicPath); err != nil {
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
func renderClassifyTitlePrompt(messages []session.Message, wikiRoot string) (string, []ghindex.TopicCandidate, error) {
	candidates := wikiclassify.LoadClassificationCandidates(wikiRoot)
	if len(candidates) == 0 {
		return "", nil, errors.New("no topic candidates available")
	}

	tmpl, err := template.New("classify-title.txt").
		Option("missingkey=error").
		ParseFS(promptFS, "prompts/classify-title.txt")
	if err != nil {
		return "", nil, fmt.Errorf("parse prompt: %w", err)
	}

	// Use all user messages (no truncation) so the classifier has full context
	// to match against topic candidates. Session-sized prompts are fine for a
	// single lightweight classification call.
	userMessages := extractUserMessages(messages)
	content := strings.Join(userMessages, "\n\n")

	data := map[string]string{
		"CandidateTree": wikiclassify.FormatTopicCandidatesGrouped(candidates),
		"Content":       content,
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", nil, fmt.Errorf("render prompt: %w", err)
	}

	return buf.String(), candidates, nil
}

// parseClassifyTopicResult parses the JSON response from the AI topic call.
func parseClassifyTopicResult(raw string) (*classifyTopicResult, error) {
	// Strip markdown code fence if present
	raw = strings.TrimSpace(raw)
	if strings.HasPrefix(raw, "```") {
		lines := strings.Split(raw, "\n")
		if len(lines) >= 3 {
			lines = lines[1 : len(lines)-1]
			raw = strings.Join(lines, "\n")
		}
	}

	var result classifyTopicResult
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

// fallbackFilename is used when a title sanitizes to something unusable.
const fallbackFilename = "untitled"

// extractPrimaryModel returns the last real model used in the transcript.
// Missing model is not an error (returns "").
func extractPrimaryModel(resolved *SessionRef) (string, error) {
	switch resolved.Agent {
	case AgentCC:
		return session.ExtractPrimaryModelCC(resolved.TranscriptPath)
	case AgentCodex:
		return session.ExtractPrimaryModelCodex(resolved.TranscriptPath)
	default:
		return "", fmt.Errorf("unsupported agent %q", resolved.Agent)
	}
}

// writeExportFile writes the final markdown file with Turn-structured formatting.
func writeExportFile(outputPath, title, source, sessionID, model, issue string, messages []session.Message) error {
	frontmatter, err := generateFrontmatter(title, source, sessionID, model, issue)
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
// model and issue may be empty; empty values are omitted via yaml omitempty.
// score is always written and defaults to 0.
func generateFrontmatter(title, source, sessionID, model, issue string) (string, error) {
	fm := Frontmatter{
		Type:    "research",
		Title:   title,
		Date:    time.Now().Format("2006-01-02"),
		Source:  source,
		Session: sessionID,
		Model:   model,
		Issue:   issue,
		Score:   0,
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
