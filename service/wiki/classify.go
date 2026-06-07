package wiki

import (
	"bytes"
	"context"
	"embed"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	"github.com/goccy/go-yaml"
	"github.com/xbpk3t/docs-alfred/pkg/ai"
	"github.com/xbpk3t/docs-alfred/pkg/httputil"
)

//go:embed prompts/*.txt
var promptFS embed.FS

// Content type constants.
const (
	ContentVideo = "video"
	ContentAudio = "audio"
	ContentText  = "text"
)

const noneVal = "none"

// ClassifyType represents the wiki entry type.
type ClassifyType string

const (
	TypeRepoEval ClassifyType = "repo_eval"
	TypeDeepDive ClassifyType = "deep_dive"
	TypeInbox    ClassifyType = "inbox"
)

// ClassifyItem holds the full classification result for a URL.
type ClassifyItem struct {
	URL         string       `json:"url"`
	Title       string       `json:"title"`
	ContentType string       `json:"contentType"`
	TopicPath   string       `json:"topicPath"`
	Type        ClassifyType `json:"type"`
	Summary     string       `json:"summary"`
}

// ClassifyResult is the structured output from classifyItem.
type ClassifyResult struct {
	TopicPath   string       `json:"topicPath"`
	WikiType    ClassifyType `json:"wikiType"`
	ContentType string       `json:"contentType"`
	Summary     string       `json:"summary"`
}

// Classifier handles AI-powered classification of URLs.
type Classifier struct {
	AIConfig    *ai.ClientConfig
	WikiRoot    string
	GhTopicsURL string
}

// NewClassifier creates a new Classifier.
func NewClassifier(aiCfg *ai.ClientConfig, wikiRoot, ghTopicsURL string) *Classifier {
	return &Classifier{
		AIConfig:    aiCfg,
		WikiRoot:    wikiRoot,
		GhTopicsURL: ghTopicsURL,
	}
}

// DetectContentType determines the content type from a URL.
func DetectContentType(urlLower string) string {
	if strings.Contains(urlLower, "bilibili.com") ||
		strings.Contains(urlLower, "youtube.com") ||
		strings.Contains(urlLower, "youtu.be") {
		return ContentVideo
	}
	if strings.Contains(urlLower, "xiaoyuzhou") ||
		strings.Contains(urlLower, "podcast") ||
		strings.Contains(urlLower, "libsyn.com") {
		return ContentAudio
	}

	return ContentText
}

// ClassifyURL performs full classification on a URL with fetched title + content.
// Returns nil, nil if classification is unavailable (graceful degradation).
func (c *Classifier) ClassifyURL(ctx context.Context, urlStr, title, content string) *ClassifyResult {
	contentType := DetectContentType(strings.ToLower(urlStr))

	// Run topic + type classification in parallel
	type topicResult struct {
		err  error
		path string
	}
	type typeResult struct {
		err error
		typ ClassifyType
	}

	topicCh := make(chan topicResult, 1)
	typeCh := make(chan typeResult, 1)

	go func() {
		path, err := c.classifyTopic(ctx, urlStr, title, content)
		topicCh <- topicResult{err, path}
	}()
	go func() {
		typ, err := c.classifyType(ctx, urlStr, title, content)
		typeCh <- typeResult{err, typ}
	}()

	tRes := <-topicCh
	tyRes := <-typeCh

	if tRes.err != nil {
		slog.Warn("Topic classification failed", "url", urlStr, "error", tRes.err)

		return nil
	}
	if tyRes.err != nil {
		slog.Warn("Type classification failed", "url", urlStr, "error", tyRes.err)
		tyRes.typ = TypeInbox
	}

	// Validate topic path
	if err := ValidateRelativeWikiPath(c.WikiRoot, tRes.path); err != nil {
		slog.Warn("Invalid topic path, falling back to inbox", "path", tRes.path, "error", err)

		return &ClassifyResult{
			TopicPath:   "inbox",
			WikiType:    TypeInbox,
			ContentType: contentType,
		}
	}

	// Generate structured summary (non-fatal if it fails)
	summary, _ := c.summarizeText(ctx, urlStr, title, content, string(tyRes.typ))
	if summary == "" {
		slog.Warn("No summary generated", "url", urlStr)
	}

	return &ClassifyResult{
		TopicPath:   tRes.path,
		WikiType:    tyRes.typ,
		ContentType: contentType,
		Summary:     summary,
	}
}

func (c *Classifier) classifyTopic(ctx context.Context, urlStr, title, content string) (string, error) {
	dirTree := scanWikiDirs(c.WikiRoot)
	ghTopicTree := fetchGHTopicsYAML(ctx, c.GhTopicsURL)

	prompt, err := renderPrompt("classify-topic.txt", &promptData{
		DirTree:     dirTree,
		GHTopicTree: ghTopicTree,
		Title:       truncate(title, 200),
		URL:         urlStr,
		Content:     truncate(content, 3000),
	})
	if err != nil {
		return "", err
	}

	result, err := ai.ChatContext(ctx, c.AIConfig, []ai.Message{{Role: "user", Content: prompt}})
	if err != nil {
		return "", err
	}

	rawPath := strings.TrimSpace(result)
	rawPath = strings.Trim(rawPath, "\"'")

	// AI explicitly says nothing matches
	if rawPath == noneVal || rawPath == "" {
		return noneVal, nil
	}

	return rawPath, nil
}

func (c *Classifier) classifyType(ctx context.Context, urlStr, title, content string) (ClassifyType, error) {
	prompt, err := renderPrompt("classify-type.txt", &promptData{
		Title:   truncate(title, 200),
		URL:     urlStr,
		Content: truncate(content, 3000),
	})
	if err != nil {
		return TypeInbox, err
	}

	result, err := ai.ChatContext(ctx, c.AIConfig, []ai.Message{{Role: "user", Content: prompt}})
	if err != nil {
		return TypeInbox, err
	}

	result = strings.TrimSpace(strings.ToLower(result))
	switch result {
	case "repo_eval":
		return TypeRepoEval, nil
	case "deep_dive":
		return TypeDeepDive, nil
	default:
		return TypeInbox, nil
	}
}

// summarizeText generates a structured Chinese summary of the article content.
func (c *Classifier) summarizeText(ctx context.Context, urlStr, title, content, wikiType string) (string, error) {
	if content == "" {
		return "", errors.New("empty content, skipping summary")
	}

	prompt, err := renderPrompt("summarize-text.txt", &promptData{
		Title:   truncate(title, 200),
		URL:     urlStr,
		Type:    wikiType,
		Content: truncate(content, 5000),
	})
	if err != nil {
		return "", err
	}

	result, err := ai.ChatContext(ctx, c.AIConfig, []ai.Message{{Role: "user", Content: prompt}})
	if err != nil {
		return "", fmt.Errorf("summarize: %w", err)
	}

	return strings.TrimSpace(result), nil
}

type promptData struct {
	DirTree     string
	GHTopicTree string
	Title       string
	URL         string
	Type        string
	Content     string
}

func renderPrompt(name string, data *promptData) (string, error) {
	tmpl, err := template.New(name).
		Option("missingkey=error").
		ParseFS(promptFS, "prompts/"+name)
	if err != nil {
		return "", fmt.Errorf("parse prompt %s: %w", name, err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("render prompt %s: %w", name, err)
	}

	return buf.String(), nil
}

// scanWikiDirs scans wikiRoot for existing topic directories.
// Returns indented tree matching TS format: "  folder/type/topic".
func scanWikiDirs(wikiRoot string) string {
	entries, err := os.ReadDir(wikiRoot)
	if err != nil {
		return ""
	}

	var lines []string
	for _, top := range entries {
		lines = scanTopLevelDir(wikiRoot, top, lines)
	}

	return strings.Join(lines, "\n")
}

func scanTopLevelDir(wikiRoot string, top os.DirEntry, lines []string) []string {
	if !top.IsDir() || strings.HasPrefix(top.Name(), ".") || top.Name() == "wiki-prototype" {
		return lines
	}
	topPath := filepath.Join(wikiRoot, top.Name())
	types, err := os.ReadDir(topPath)
	if err != nil {
		return lines
	}
	for _, typ := range types {
		lines = scanTypeDir(topPath, top.Name(), typ, lines)
	}

	return lines
}

func scanTypeDir(topPath, topName string, typ os.DirEntry, lines []string) []string {
	if !typ.IsDir() || strings.HasPrefix(typ.Name(), ".") {
		return lines
	}
	typePath := filepath.Join(topPath, typ.Name())
	topics, err := os.ReadDir(typePath)
	if err != nil {
		return lines
	}
	for _, topic := range topics {
		if !topic.IsDir() || strings.HasPrefix(topic.Name(), ".") {
			continue
		}
		lines = append(lines, fmt.Sprintf("  %s/%s/%s", topName, typ.Name(), topic.Name()))
	}

	return lines
}

// fetchGHTopicsYAML fetches gh.yml from a URL and returns a formatted topic tree.
// The YAML structure is: [{tag, type, topics: [{topic}]}].
// Returns an empty string if the URL is empty or if any step fails (non-blocking).
func fetchGHTopicsYAML(ctx context.Context, url string) string {
	if url == "" {
		return ""
	}

	data, err := httputil.GetBytes(ctx, url, httputil.RequestOptions{Timeout: 10 * time.Second})
	if err != nil {
		slog.Warn("Failed to fetch gh topics YAML", "url", url, "error", err)

		return ""
	}

	var entries []struct {
		Tag    string `yaml:"tag"`
		Type   string `yaml:"type"`
		Topics []struct {
			Topic string `yaml:"topic"`
		} `yaml:"topics"`
	}
	if err := yaml.Unmarshal(data, &entries); err != nil {
		slog.Warn("Failed to parse gh topics YAML", "error", err)

		return ""
	}

	var lines []string
	for _, entry := range entries {
		for _, t := range entry.Topics {
			lines = append(lines, fmt.Sprintf("  %s/%s/%s", entry.Tag, entry.Type, t.Topic))
		}
	}

	return strings.Join(lines, "\n")
}

// ValidateRelativeWikiPath ensures a relative path doesn't escape wikiRoot.
func ValidateRelativeWikiPath(wikiRoot, relativePath string) error {
	if wikiRoot == "" {
		return errors.New("wiki root is empty")
	}
	if relativePath == "" {
		return errors.New("relative path is empty")
	}

	if filepath.IsAbs(relativePath) {
		return fmt.Errorf("absolute path not allowed: %s", relativePath)
	}

	// Check for path traversal segments
	segments := strings.SplitSeq(relativePath, "/")
	for seg := range segments {
		if seg == "" || seg == "." || seg == ".." {
			return fmt.Errorf("invalid segment: %q", seg)
		}
		if strings.ContainsAny(seg, "\\\x00\n\r") {
			return fmt.Errorf("invalid characters in segment: %q", seg)
		}
	}

	// Resolve and check not escaping root
	resolved := filepath.Clean(filepath.Join(wikiRoot, relativePath))
	if !strings.HasPrefix(resolved, filepath.Clean(wikiRoot)) {
		return fmt.Errorf("path traversal detected: %s escapes %s", relativePath, wikiRoot)
	}

	return nil
}

// ErrClassificationUnavailable is returned when AI classification is not available.
var ErrClassificationUnavailable = errors.New("classification unavailable")

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}

	return s[:maxLen] + "..."
}
