package internal

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/xbpk3t/docs-alfred/pkg/ai"
	"github.com/xbpk3t/docs-alfred/pkg/cmdutil"
	wikisvc "github.com/xbpk3t/docs-alfred/service/wiki"
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

// ExportSession exports the current session to wiki.
func ExportSession(input ExportInput) (*ExportResult, error) {
	// 1. Walk session chain
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

	// 2. Export each session to markdown
	markdowns, err := exportAllSessions(chain, input.Verbose)
	if err != nil {
		return nil, err
	}

	// 3. Merge markdowns
	mergedMarkdown := mergeMarkdowns(markdowns, chain)

	// 4. Generate semantic title from content
	title := generateTitle(mergedMarkdown)

	// 5. Classify content to determine topic path
	topicPath := classifyWithFallback(mergedMarkdown, input.WikiRoot, input.Verbose)

	// 6. Determine output path
	outputPath, err := determineOutputPath(input, topicPath, title)
	if err != nil {
		return nil, fmt.Errorf("determine output path: %w", err)
	}

	// 7. Write to file (or dry-run)
	if input.DryRun {
		return &ExportResult{
			OutputPath: outputPath,
			TopicPath:  topicPath,
			Title:      title,
			DryRun:     true,
		}, nil
	}

	if err := os.MkdirAll(filepath.Dir(outputPath), 0750); err != nil {
		return nil, fmt.Errorf("create output dir: %w", err)
	}

	if err := os.WriteFile(outputPath, []byte(mergedMarkdown), 0600); err != nil {
		return nil, fmt.Errorf("write output file: %w", err)
	}

	return &ExportResult{
		OutputPath: outputPath,
		TopicPath:  topicPath,
		Title:      title,
	}, nil
}

// exportAllSessions exports all sessions in the chain to markdown.
func exportAllSessions(chain []ChainRecord, verbose bool) ([]string, error) {
	markdowns := make([]string, 0, len(chain))
	for _, entry := range chain {
		if verbose {
			fmt.Fprintf(os.Stderr, "Exporting session %s\n", entry.SessionID)
		}

		markdown, exportErr := exportSessionToMarkdown(entry.TranscriptPath)
		if exportErr != nil {
			return nil, fmt.Errorf("export session %s: %w", entry.SessionID, exportErr)
		}

		markdowns = append(markdowns, markdown)
	}

	return markdowns, nil
}

// classifyWithFallback classifies content and returns empty string on failure.
func classifyWithFallback(content, wikiRoot string, verbose bool) string {
	topicPath, err := classifyContent(content, wikiRoot)
	if err != nil {
		if verbose {
			fmt.Fprintf(os.Stderr, "Classification failed: %v\n", err)
		}

		return ""
	}

	return topicPath
}

// exportSessionToMarkdown exports a single session JSONL to markdown using claude-conversation-extractor.
func exportSessionToMarkdown(sessionPath string) (string, error) {
	if _, ok := cmdutil.LookPath("claude-extract"); !ok {
		return "", errors.New("claude-extract not found (install with: uv tool install claude-conversation-extractor)")
	}

	// Create temp directory for output
	tmpDir, err := os.MkdirTemp("", "ccx-session-*")
	if err != nil {
		return "", fmt.Errorf("create temp dir: %w", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Symlink the JSONL file to temp dir
	err = os.Symlink(sessionPath, filepath.Join(tmpDir, filepath.Base(sessionPath)))
	if err != nil {
		return "", fmt.Errorf("symlink session file: %w", err)
	}

	// Call claude-extract to export the session
	_, err = cmdutil.RunStdout(context.Background(), "claude-extract",
		"--extract", "1",
		"--output", tmpDir,
	)
	if err != nil {
		return "", fmt.Errorf("claude-extract: %w", err)
	}

	// Find and read the output file
	entries, err := os.ReadDir(tmpDir)
	if err != nil {
		return "", fmt.Errorf("read output dir: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".md") {
			data, err := os.ReadFile(filepath.Join(tmpDir, entry.Name()))
			if err != nil {
				return "", fmt.Errorf("read output file: %w", err)
			}

			return string(data), nil
		}
	}

	return "", errors.New("no markdown file found in output")
}

// mergeMarkdowns merges multiple session markdowns into one.
func mergeMarkdowns(markdowns []string, chain []ChainRecord) string {
	if len(markdowns) == 1 {
		return markdowns[0]
	}

	var result strings.Builder
	for i, markdown := range markdowns {
		if i > 0 {
			result.WriteString("\n\n---\n\n")
		}

		if len(chain) > i && i > 0 {
			fmt.Fprintf(&result, "## Session %d\n\n", i)
		}

		result.WriteString(markdown)
	}

	return result.String()
}

// generateTitle generates a semantic title from markdown content.
func generateTitle(markdown string) string {
	userMessage := extractFirstUserMessage(markdown)
	if userMessage == "" {
		return time.Now().Format("2006-01-02-15-04-05")
	}

	if len(userMessage) > 60 {
		userMessage = userMessage[:60]
	}

	return sanitizeFilename(userMessage)
}

// extractFirstUserMessage extracts the first user message from markdown.
func extractFirstUserMessage(markdown string) string {
	lines := strings.Split(markdown, "\n")
	inUserBlock := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		if strings.Contains(trimmed, "👤 User") {
			inUserBlock = true

			continue
		}

		if inUserBlock && trimmed != "" && trimmed != "---" {
			if isValidUserMessage(trimmed) {
				return trimmed
			}
		}

		if inUserBlock && trimmed == "---" {
			inUserBlock = false
		}
	}

	return ""
}

// isValidUserMessage checks if the content is a valid user message.
func isValidUserMessage(content string) bool {
	if content == "" || content == ">" {
		return false
	}

	prefixes := []string{
		"**User**",
		"Date:",
		"Model:",
		"Working Directory:",
		"Session:",
		"@",
	}

	for _, prefix := range prefixes {
		if strings.HasPrefix(content, prefix) {
			return false
		}
	}

	return true
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

// classifyContent classifies the content to determine topic path.
func classifyContent(content, wikiRoot string) (string, error) {
	// Create AI config
	aiConfig := &ai.ClientConfig{
		BaseURL: "https://api.lucc.dev/v1",
		Model:   "deepseek-v4-flash",
	}

	// Create classifier
	classifier := wikisvc.NewClassifier(aiConfig, wikiRoot, "https://cdn.lucc.dev/gh.yml")

	// Classify content
	result := classifier.ClassifyURL(context.Background(), "session-export", "Session Export", content)
	if result == nil {
		return "", errors.New("classification returned nil")
	}

	return result.TopicPath, nil
}

// determineOutputPath determines the output path for the exported session.
func determineOutputPath(input ExportInput, topicPath, title string) (string, error) {
	// Generate filename
	date := time.Now().Format("2006-01-02")
	filename := fmt.Sprintf("%s-%s.md", date, title)

	// Determine base directory
	baseDir := input.WikiRoot
	if input.OutputDir != "" {
		baseDir = input.OutputDir
	}

	// Add topic path if available
	if topicPath != "" {
		baseDir = filepath.Join(baseDir, topicPath)
	}

	outputPath := filepath.Join(baseDir, filename)

	return outputPath, nil
}
