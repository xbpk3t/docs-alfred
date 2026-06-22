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
	"reflect"
	"sort"
	"strconv"
	"strings"
	"sync"
	"text/template"
	"time"

	"github.com/samber/lo"
	"github.com/xbpk3t/docs-alfred/pkg/ai"
	"github.com/xbpk3t/docs-alfred/pkg/textutil"
	"github.com/xbpk3t/docs-alfred/pkg/validator"
	"github.com/xbpk3t/docs-alfred/service/ghindex"
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

// minContentForVideo is the minimum content length (in runes) for video content
// to be classified. Below this threshold the fetched content is likely just
// metadata (title, stats, description) without a transcript, making classification
// unreliable.
const minContentForVideo = 600

// ClassifyType represents the wiki entry type.
type ClassifyType string

const (
	TypeRepoEval ClassifyType = "review"
	TypeDeepDive ClassifyType = "research"
	TypeInbox    ClassifyType = "inbox"
)

// ClassifyItem holds the full classification result for a URL.
type ClassifyItem struct {
	URL               string             `json:"url"`
	Title             string             `json:"title"`
	ContentType       string             `json:"contentType"`
	TopicPath         string             `json:"topicPath"`
	Type              ClassifyType       `json:"type"`
	Summary           *StructuredSummary `json:"summary"`
	MetadataBlock     string             `json:"metadataBlock,omitempty"`
	NeedsManualReview bool               `json:"needsManualReview,omitempty"`
}

// ClassifyResult is the structured output from classifyItem.
type ClassifyResult struct {
	TopicPath         string             `json:"topicPath"`
	WikiType          ClassifyType       `json:"wikiType"`
	ContentType       string             `json:"contentType"`
	Summary           *StructuredSummary `json:"summary"`
	MetadataBlock     string             `json:"metadataBlock,omitempty"`
	RejectReason      string             `json:"rejectReason,omitempty"`
	Confidence        float64            `json:"confidence,omitempty"`
	NeedsManualReview bool               `json:"needsManualReview,omitempty"`
}

// Classifier handles AI-powered classification of URLs.
type Classifier struct {
	catalogErr        error
	AIConfig          *ai.ClientConfig
	loadGHTopics      func() ([]ghindex.TopicCandidate, error)
	WikiRoot          string
	GhTopicsURL       string
	GhTopicsCachePath string
	catalog           []ghindex.TopicCandidate
	GhTopicsMaxAge    time.Duration
	CandidateLimit    int
	MinConfidence     float64
	MaxContentSize    int // max chars sent to AI; 0 defaults to 20000
	catalogMu         sync.Mutex
	catalogLoaded     bool
}

// ClassifierOption customizes a classifier.
type ClassifierOption func(*Classifier)

// WithGHTopicsCachePath sets the cache path for remote gh.yml.
func WithGHTopicsCachePath(path string) ClassifierOption {
	return func(c *Classifier) { c.GhTopicsCachePath = path }
}

// WithGHTopicsMaxAge sets the remote gh.yml cache TTL.
func WithGHTopicsMaxAge(maxAge time.Duration) ClassifierOption {
	return func(c *Classifier) { c.GhTopicsMaxAge = maxAge }
}

// WithCandidateLimit sets the maximum remote topic candidates sent to AI.
func WithCandidateLimit(limit int) ClassifierOption {
	return func(c *Classifier) { c.CandidateLimit = limit }
}

// WithMaxContentSize sets the max character count sent to AI for content.
func WithMaxContentSize(n int) ClassifierOption {
	return func(c *Classifier) { c.MaxContentSize = n }
}

// NewClassifier creates a new Classifier.
func NewClassifier(aiCfg *ai.ClientConfig, wikiRoot, ghTopicsURL string, opts ...ClassifierOption) *Classifier {
	c := &Classifier{
		AIConfig:       aiCfg,
		WikiRoot:       wikiRoot,
		GhTopicsURL:    ghTopicsURL,
		GhTopicsMaxAge: ghindex.DefaultMaxAge,
		CandidateLimit: 120,
		MinConfidence:  0.30,
		MaxContentSize: 20000,
	}
	for _, opt := range opts {
		opt(c)
	}
	if c.CandidateLimit <= 0 {
		c.CandidateLimit = 120
	}
	if c.MinConfidence <= 0 {
		c.MinConfidence = 0.45
	}

	return c
}

// DetectContentType determines the content type from a URL.
func DetectContentType(urlLower string) string {
	urlLower = strings.ToLower(strings.TrimSpace(urlLower))
	if isVideoURL(urlLower) {
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
// Uses a two-step pipeline: classify (topic+type+metadata) then summarize (detailed summary).
// Returns nil if classification is unavailable (graceful degradation).
func (c *Classifier) ClassifyURL(ctx context.Context, urlStr, title, content string) *ClassifyResult {
	contentType := DetectContentType(strings.ToLower(urlStr))
	if strings.TrimSpace(content) == "" {
		slog.Warn("Classification skipped for empty content", "url", urlStr)

		return nil
	}

	// For video content, require transcript-length content. Short content
	// (e.g., 492 bytes of bilibili metadata without subtitles) can't produce
	// meaningful summaries; treat as extraction failure instead of routing to
	// uncategorized.
	if contentType == ContentVideo && len([]rune(content)) < minContentForVideo {
		slog.Warn("Video content too short (likely no transcript)", "url", urlStr, "len", len(content))

		return nil
	}

	candidates, err := c.classificationCandidates(ctx, urlStr, title, content)
	if err != nil {
		slog.Warn("Classification candidates unavailable", "url", urlStr, "error", err)
	}
	if len(candidates) == 0 {
		slog.Warn("Classification skipped with no topic candidates", "url", urlStr)

		return nil
	}

	maxLen := c.MaxContentSize
	if maxLen <= 0 {
		maxLen = 20000
	}

	// Single-step classification using classify-json.txt which returns both
	// classification (topic, type, metadata) and structured summary.
	classified, err := c.classifyOnly(ctx, urlStr, title, contentType, content, candidates, maxLen)
	if err != nil {
		slog.Warn("AI classification failed", "url", urlStr, "error", err)

		return nil
	}

	return c.buildClassifyResult(classified, contentType, candidates, urlStr)
}

// buildClassifyResult processes the AI classification result and builds a ClassifyResult.
// It handles manual review, rejection, validation, metadata building, and empty summary checks.
func (c *Classifier) buildClassifyResult(classified *aiClassification, contentType string, candidates []ghindex.TopicCandidate, urlStr string) *ClassifyResult {
	if c.isManualReviewWithGoodContent(classified) {
		return &ClassifyResult{
			TopicPath:         "",
			WikiType:          classified.WikiType,
			ContentType:       contentType,
			Summary:           classified.Summary,
			Confidence:        classified.Confidence,
			NeedsManualReview: true,
		}
	}

	if classified.RejectReason != "" {
		return rejectedClassifyResult(classified, contentType, errors.New(classified.RejectReason))
	}

	if validationErr := c.validateAIClassificationBasics(classified); validationErr != nil {
		slog.Warn("AI classification rejected", "url", urlStr, "error", validationErr)

		return rejectedClassifyResult(classified, contentType, validationErr)
	}

	topicPath, err := c.validateAIClassificationTopic(classified, candidates)
	if err != nil {
		slog.Warn("AI topic validation failed", "url", urlStr, "error", err)

		return nil
	}

	metaBlock := buildMetaBlock(classified)

	if classified.Summary == nil || strings.TrimSpace(classified.Summary.Overview) == "" {
		slog.Warn("Empty summary from classification", "url", urlStr)

		return nil
	}

	return &ClassifyResult{
		TopicPath:         topicPath,
		WikiType:          classified.WikiType,
		ContentType:       contentType,
		Summary:           classified.Summary,
		MetadataBlock:     metaBlock,
		Confidence:        classified.Confidence,
		NeedsManualReview: classified.NeedsManualReview,
	}
}

// classifyOnlyResult holds the parsed JSON from the classify-only AI call.
type classifyOnlyResult struct {
	Summary           *StructuredSummary `json:"summary"`
	Metadata          *EntryMetadata     `json:"metadata"`
	TopicPath         string             `json:"topicPath"`
	WikiType          ClassifyType       `json:"wikiType"`
	ContentType       string             `json:"contentType"`
	RejectReason      string             `json:"rejectReason,omitempty"`
	Confidence        float64            `json:"confidence"`
	NeedsManualReview bool               `json:"needsManualReview"`
}

// classifyOnly runs the first AI call: classify topic, type, and metadata.
func (c *Classifier) classifyOnly(
	ctx context.Context,
	urlStr, title, contentType, content string,
	candidates []ghindex.TopicCandidate,
	maxLen int,
) (*aiClassification, error) {
	prompt, err := renderPrompt("classify-json.txt", &promptData{
		CandidateTree: formatTopicCandidates(candidates),
		Title:         truncate(title, 200),
		URL:           urlStr,
		ContentType:   contentType,
		Content:       truncate(content, maxLen),
	})
	if err != nil {
		return nil, fmt.Errorf("render classify prompt: %w", err)
	}

	result, err := ai.ChatContext(ctx, c.AIConfig, []ai.Message{{Role: "user", Content: prompt}})
	if err != nil {
		return nil, fmt.Errorf("AI classify call: %w", err)
	}

	parsed, err := parseClassifyOnlyResult(result)
	if err != nil {
		return nil, fmt.Errorf("parse classify JSON: %w", err)
	}

	if err := validateClassifyResult(parsed); err != nil {
		return nil, fmt.Errorf("validate classify result: %w", err)
	}

	// Convert to aiClassification for reuse of validation methods.
	return &aiClassification{
		TopicPath:         parsed.TopicPath,
		WikiType:          parsed.WikiType,
		ContentType:       parsed.ContentType,
		Summary:           parsed.Summary,
		Metadata:          parsed.Metadata,
		Confidence:        parsed.Confidence,
		NeedsManualReview: parsed.NeedsManualReview,
		RejectReason:      parsed.RejectReason,
	}, nil
}

func parseClassifyOnlyResult(raw string) (*classifyOnlyResult, error) {
	var result classifyOnlyResult
	err := ai.UnmarshalStrictJSON(raw, &result)
	if err != nil {
		repaired := repairInvalidJSONStringEscapes(raw)
		if repaired != raw {
			if retryErr := ai.UnmarshalStrictJSON(repaired, &result); retryErr != nil {
				return nil, fmt.Errorf("%w; repaired JSON parse failed: %w", err, retryErr)
			}

			return &result, nil
		}

		return nil, err
	}

	return &result, nil
}

// validateClassifyResult validates the parsed classification result using struct tags.
// Returns an error if any field violates its constraints (e.g., tags is not an array,
// quality format is invalid, required fields are missing).
func validateClassifyResult(result *classifyOnlyResult) error {
	if result == nil {
		return errors.New("nil classify result")
	}

	if result.Summary != nil {
		if err := validator.Struct(result.Summary); err != nil {
			return fmt.Errorf("summary: %w", err)
		}
	}

	if result.Metadata != nil {
		if err := validator.Struct(result.Metadata); err != nil {
			return fmt.Errorf("metadata: %w", err)
		}
	}

	return nil
}

type promptData struct {
	DirTree       string
	GHTopicTree   string
	CandidateTree string
	Title         string
	URL           string
	Type          string
	ContentType   string
	Content       string
	TopicPath     string
	WikiType      string
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

func (c *Classifier) classificationCandidates(
	_ context.Context,
	urlStr,
	title,
	content string,
) ([]ghindex.TopicCandidate, error) {
	seen := make(map[string]bool)
	candidates := appendUniqueTopicCandidates(nil, seen, scanWikiCandidates(c.WikiRoot))

	remote, err := c.ghTopicCatalog()
	if err != nil {
		return candidates, err
	}
	ranked := rankTopicCandidates(remote, title+"\n"+urlStr+"\n"+truncate(content, 3000), c.CandidateLimit)
	candidates = appendUniqueTopicCandidates(candidates, seen, ranked)

	return candidates, nil
}

func (c *Classifier) ghTopicCatalog() ([]ghindex.TopicCandidate, error) {
	c.catalogMu.Lock()
	defer c.catalogMu.Unlock()

	if c.catalogLoaded && c.catalogErr == nil {
		return c.catalog, nil
	}

	loader := c.loadGHTopics
	if loader == nil {
		loader = c.defaultGHTopicsLoader
	}
	catalog, err := loader()
	if err != nil {
		c.catalogErr = err
		slog.Warn("Remote wiki topic catalog unavailable; using local candidates only", "error", err)

		return nil, err
	}

	c.catalog = catalog
	c.catalogErr = nil
	c.catalogLoaded = true

	return c.catalog, nil
}

func (c *Classifier) defaultGHTopicsLoader() ([]ghindex.TopicCandidate, error) {
	manager := ghindex.NewManager(c.GhTopicsCachePath, c.GhTopicsURL)
	if c.GhTopicsMaxAge > 0 {
		manager.SetTTL(c.GhTopicsMaxAge)
	}
	if err := manager.LoadWithCacheTTL(); err != nil {
		return nil, err
	}

	return manager.ConfigRepos().TopicCatalog(), nil
}

func scanWikiCandidates(wikiRoot string) []ghindex.TopicCandidate {
	entries, err := os.ReadDir(wikiRoot)
	if err != nil {
		return nil
	}

	var candidates []ghindex.TopicCandidate
	for _, top := range entries {
		candidates = scanTopLevelCandidates(wikiRoot, top, candidates)
	}

	return candidates
}

func scanTopLevelCandidates(
	wikiRoot string,
	top os.DirEntry,
	candidates []ghindex.TopicCandidate,
) []ghindex.TopicCandidate {
	if !top.IsDir() || strings.HasPrefix(top.Name(), ".") || top.Name() == "wiki-prototype" || top.Name() == "failed" {
		return candidates
	}
	topPath := filepath.Join(wikiRoot, top.Name())
	types, err := os.ReadDir(topPath)
	if err != nil {
		return candidates
	}
	for _, typ := range types {
		candidates = scanTypeCandidates(topPath, top.Name(), typ, candidates)
	}

	return candidates
}

func scanTypeCandidates(
	topPath,
	topName string,
	typ os.DirEntry,
	candidates []ghindex.TopicCandidate,
) []ghindex.TopicCandidate {
	if !typ.IsDir() || strings.HasPrefix(typ.Name(), ".") {
		return candidates
	}
	typePath := filepath.Join(topPath, typ.Name())
	topics, err := os.ReadDir(typePath)
	if err != nil {
		return candidates
	}
	for _, topic := range topics {
		if !topic.IsDir() || strings.HasPrefix(topic.Name(), ".") {
			continue
		}
		path := strings.Join([]string{topName, typ.Name(), topic.Name()}, "/")
		candidates = append(candidates, ghindex.TopicCandidate{Path: path, Display: topic.Name(), Source: "wiki"})
	}

	return candidates
}

func appendUniqueTopicCandidates(
	candidates []ghindex.TopicCandidate,
	seen map[string]bool,
	items []ghindex.TopicCandidate,
) []ghindex.TopicCandidate {
	for _, item := range items {
		item.Path = strings.TrimSpace(item.Path)
		if item.Path == "" || seen[item.Path] {
			continue
		}
		if err := ValidateRelativeWikiPath(string(filepath.Separator), item.Path); err != nil {
			continue
		}
		seen[item.Path] = true
		candidates = append(candidates, item)
	}

	return candidates
}

type candidateRank struct {
	candidate ghindex.TopicCandidate
	score     int
	index     int
}

func rankTopicCandidates(
	candidates []ghindex.TopicCandidate,
	query string,
	limit int,
) []ghindex.TopicCandidate {
	if limit <= 0 || len(candidates) <= limit {
		return candidates
	}
	query = strings.ToLower(query)
	ranks := make([]candidateRank, 0, len(candidates))
	for i, candidate := range candidates {
		score := scoreTopicCandidate(candidate, query)
		ranks = append(ranks, candidateRank{candidate: candidate, score: score, index: i})
	}
	sort.SliceStable(ranks, func(i, j int) bool {
		if ranks[i].score != ranks[j].score {
			return ranks[i].score > ranks[j].score
		}

		return ranks[i].index < ranks[j].index
	})

	if ranks[0].score <= 0 && limit > 40 {
		limit = 40
	}
	if len(ranks) < limit {
		limit = len(ranks)
	}

	result := make([]ghindex.TopicCandidate, 0, limit)
	for i := range limit {
		result = append(result, ranks[i].candidate)
	}

	return result
}

func scoreTopicCandidate(candidate ghindex.TopicCandidate, query string) int {
	target := strings.ToLower(candidate.Path + " " + candidate.Display)
	var score int
	for _, token := range topicTokens(target) {
		if len(token) < 2 {
			continue
		}
		if strings.Contains(query, token) {
			score += len(token)
		}
	}
	for _, token := range topicTokens(query) {
		if len(token) < 3 {
			continue
		}
		if strings.Contains(target, token) {
			score += len(token)
		}
	}

	return score
}

func topicTokens(s string) []string {
	return strings.FieldsFunc(s, func(r rune) bool {
		switch r {
		case '/', '-', '_', '.', ',', ':', ';', '(', ')', '[', ']', '{', '}', ' ', '\t', '\n', '\r':
			return true
		default:
			return false
		}
	})
}

func formatTopicCandidates(candidates []ghindex.TopicCandidate) string {
	var lines []string
	for _, candidate := range candidates {
		display := strings.TrimSpace(candidate.Display)
		if display != "" && display != candidate.Path {
			lines = append(lines, fmt.Sprintf("- path: %s | title: %s | source: %s", candidate.Path, display, candidate.Source))

			continue
		}
		lines = append(lines, fmt.Sprintf("- path: %s | source: %s", candidate.Path, candidate.Source))
	}

	return strings.Join(lines, "\n")
}

// StructuredSummary holds the AI-generated summary broken into sections.
type StructuredSummary struct {
	Overview         string   `json:"overview"                   validate:"required"`
	WorthNoting      string   `json:"worthNoting"`
	Detail           string   `json:"detail,omitempty"`
	KeyPoints        []string `json:"keyPoints"                  validate:"required|min_len:1"`
	ActionableAdvice []string `json:"actionableAdvice,omitempty"`
}

// EntryMetadata holds additional metadata fields from AI classification.
type EntryMetadata struct {
	ContentType       string   `json:"contentType"                 validate:"required|in:text,media,repo"`
	Quality           string   `json:"quality,omitempty"           validate:"quality"`
	Author            string   `json:"author,omitempty"`
	Uncertainties     string   `json:"uncertainties,omitempty"`
	Duration          string   `json:"duration,omitempty"          validate:"duration"`
	TranscriptQuality string   `json:"transcriptQuality,omitempty" validate:"in:good,fair,poor"`
	Verdict           string   `json:"verdict,omitempty"           validate:"in:watch,skip,try"`
	Language          string   `json:"language,omitempty"`
	Tags              []string `json:"tags,omitempty"              validate:"required|min_len:3|max_len:8"`
	Stars             int      `json:"stars,omitempty"`
}

// ContentTypeDisplay maps content types to display-friendly labels.
const (
	DisplayTypeText  = "text"
	DisplayTypeMedia = "media"
	DisplayTypeRepo  = "repo"
)

type aiClassification struct {
	Summary           *StructuredSummary `json:"summary"`
	Metadata          *EntryMetadata     `json:"metadata"`
	TopicPath         string             `json:"topicPath"`
	WikiType          ClassifyType       `json:"wikiType"`
	ContentType       string             `json:"contentType"`
	RejectReason      string             `json:"rejectReason,omitempty"`
	Confidence        float64            `json:"confidence"`
	NeedsManualReview bool               `json:"needsManualReview"`
}

// RenderStructuredSummary converts a StructuredSummary to markdown sections.
// Iterates struct fields in order, using JSON tags as headings.
// Add/remove fields in StructuredSummary — rendering adapts automatically.
func RenderStructuredSummary(s *StructuredSummary) string {
	if s == nil {
		return ""
	}

	var b strings.Builder
	v := reflect.ValueOf(*s)
	t := v.Type()
	for i := range t.NumField() {
		field := t.Field(i)
		key := jsonKey(&field)
		if key == "" {
			continue
		}
		fv := v.Field(i)
		switch fv.Kind() {
		case reflect.String:
			s := fv.String()
			if s == "" {
				continue
			}
			fmt.Fprintf(&b, "#### %s\n%s\n\n", key, s)
		case reflect.Slice:
			if fv.Len() == 0 {
				continue
			}
			fmt.Fprintf(&b, "#### %s\n", key)
			for j := range fv.Len() {
				fmt.Fprintf(&b, "- %s\n", fv.Index(j).String())
			}
			b.WriteString("\n")
		}
	}

	return strings.TrimSpace(b.String())
}

func jsonKey(f *reflect.StructField) string {
	tag := f.Tag.Get("json")
	if tag == "" || tag == "-" {
		return ""
	}
	if comma := strings.IndexByte(tag, ','); comma >= 0 {
		tag = tag[:comma]
	}

	return tag
}

// renderMetadataCodeblock formats metadata into a markdown codeblock body.

// buildMetaBlock builds the metadata codeblock body from an aiClassification.
func buildMetaBlock(result *aiClassification) string {
	if result.Metadata != nil {
		kv := metadataToMap(result.Metadata)
		if len(kv) > 0 {
			var pairs []string
			// Deterministic order: Type first, then alphabetical.
			if v, ok := kv["Type"]; ok {
				pairs = append(pairs, "Type: "+v)
				delete(kv, "Type")
			}
			for k, v := range kv {
				pairs = append(pairs, k+": "+v)
			}

			return strings.Join(pairs, "\n")
		}
	}

	return ""
}
func metadataToMap(m *EntryMetadata) map[string]string {
	if m == nil {
		return nil
	}
	kv := make(map[string]string)
	for _, f := range [...][2]string{
		{"Type", m.ContentType},
		{"quality", m.Quality},
		{"author", m.Author},
		{"uncertainties", m.Uncertainties},
		{"duration", m.Duration},
		{"transcriptQuality", m.TranscriptQuality},
		{"verdict", m.Verdict},
		{"language", m.Language},
	} {
		if f[1] != "" {
			kv[f[0]] = f[1]
		}
	}
	if len(m.Tags) > 0 {
		kv["tags"] = strings.Join(m.Tags, ", ")
	}
	if m.Stars > 0 {
		kv["stars"] = strconv.Itoa(m.Stars)
	}

	return kv
}

func parseAIClassification(raw string) (*aiClassification, error) {
	var result aiClassification
	err := ai.UnmarshalStrictJSON(raw, &result)
	if err != nil {
		// Try repair for common AI JSON escape issues.
		repaired := repairInvalidJSONStringEscapes(raw)
		if repaired == raw {
			return nil, err
		}
		if retryErr := ai.UnmarshalStrictJSON(repaired, &result); retryErr != nil {
			return nil, fmt.Errorf("%w; repaired JSON parse failed: %w", err, retryErr)
		}
	}

	return &result, nil
}

// repairInvalidJSONStringEscapes fixes AI-generated JSON escape issues that sonic
// doesn't handle natively. Sonic already tolerates unescaped \n, \r, \t in strings,
// so this only repairs invalid backslash escapes (e.g. \k) and stray control chars.
func repairInvalidJSONStringEscapes(raw string) string {
	var b strings.Builder
	b.Grow(len(raw))
	inString := false

	for i := 0; i < len(raw); i++ {
		ch := raw[i]
		if !inString {
			b.WriteByte(ch)
			if ch == '"' {
				inString = true
			}

			continue
		}
		switch ch {
		case '"':
			b.WriteByte(ch)
			inString = false
		case '\\':
			i = repairBackslash(raw, i, &b)
		default:
			if ch < 0x20 {
				fmt.Fprintf(&b, `\u%04x`, ch)
			} else {
				b.WriteByte(ch)
			}
		}
	}

	return b.String()
}

// repairBackslash handles a backslash escape sequence starting at position i.
// Valid JSON escapes are passed through; invalid ones are double-escaped.
func repairBackslash(raw string, i int, b *strings.Builder) int {
	if i+1 >= len(raw) {
		b.WriteString(`\\`)

		return i
	}
	next := raw[i+1]
	if isValidJSONEscape(next) {
		b.WriteByte('\\')
		b.WriteByte(next)

		return i + 1
	}
	if next == 'u' && i+5 < len(raw) && isHex4(raw[i+2:i+6]) {
		b.WriteString(raw[i : i+6])

		return i + 5
	}
	b.WriteString(`\\`)

	return i
}

func isValidJSONEscape(ch byte) bool {
	switch ch {
	case '"', '\\', '/', 'b', 'f', 'n', 'r', 't':
		return true
	}

	return false
}

func isHex4(s string) bool {
	if len(s) != 4 {
		return false
	}
	for i := 0; i < len(s); i++ {
		ch := s[i]
		if (ch >= '0' && ch <= '9') || (ch >= 'a' && ch <= 'f') || (ch >= 'A' && ch <= 'F') {
			continue
		}

		return false
	}

	return true
}

func (c *Classifier) validateAIClassification(
	result *aiClassification,
	candidates []ghindex.TopicCandidate,
	detectedContentType string,
) (*ClassifyResult, error) {
	if c.isManualReviewWithGoodContent(result) {
		return c.uncategorizedClassifyResult(result, detectedContentType), nil
	}

	if err := c.validateAIClassificationBasics(result); err != nil {
		return nil, err
	}
	topicPath, err := c.validateAIClassificationTopic(result, candidates)
	if err != nil {
		return nil, err
	}
	summary, err := validateAIClassificationSummary(result)
	if err != nil {
		return nil, err
	}

	metaBlock := buildMetaBlock(result)

	return &ClassifyResult{
		TopicPath:         topicPath,
		WikiType:          result.WikiType,
		ContentType:       detectedContentType,
		Summary:           summary,
		MetadataBlock:     metaBlock,
		Confidence:        result.Confidence,
		NeedsManualReview: result.NeedsManualReview,
	}, nil
}

// uncategorizedClassifyResult produces a ClassifyResult with NeedsManualReview=true
// and TopicPath=uncategorized, preserving the AI-generated summary. Used when the
// AI has good content but no matching topic — the write layer will route to uncat.md.
func (c *Classifier) uncategorizedClassifyResult(result *aiClassification, detectedContentType string) *ClassifyResult {
	return &ClassifyResult{
		TopicPath:         fallbackUncategorized(c.WikiRoot, nil),
		WikiType:          result.WikiType,
		ContentType:       detectedContentType,
		Summary:           result.Summary,
		MetadataBlock:     "",
		Confidence:        result.Confidence,
		NeedsManualReview: true,
	}
}

// isManualReviewWithGoodContent returns true when the AI explicitly marked
// NeedsManualReview but still produced a valid summary — meaning the content was
// good, only the topic match failed. In this case we route to uncategorized
// rather than rejecting outright.
func (c *Classifier) isManualReviewWithGoodContent(result *aiClassification) bool {
	return result != nil && result.NeedsManualReview &&
		result.Summary != nil && strings.TrimSpace(result.Summary.Overview) != ""
}

func (c *Classifier) validateAIClassificationBasics(result *aiClassification) error {
	if result == nil {
		return errors.New("classification result is nil")
	}
	if result.NeedsManualReview {
		return errors.New("AI marked result for manual review")
	}
	if result.Confidence < c.MinConfidence {
		return fmt.Errorf("confidence %.2f below %.2f", result.Confidence, c.MinConfidence)
	}
	if !isValidClassifyType(result.WikiType) {
		return fmt.Errorf("invalid wiki type: %s", result.WikiType)
	}
	if result.ContentType != "" && !isValidContentType(result.ContentType) {
		return fmt.Errorf("invalid content type: %s", result.ContentType)
	}

	return nil
}

func (c *Classifier) validateAIClassificationTopic(
	result *aiClassification,
	candidates []ghindex.TopicCandidate,
) (string, error) {
	topicPath := strings.TrimSpace(result.TopicPath)
	if topicPath == "" || topicPath == noneVal || topicPath == "inbox" {
		return fallbackUncategorized(c.WikiRoot, candidates), nil
	}
	//nolint: nilerr // validate path only for candidate lookup; fallback on any error.
	if err := ValidateRelativeWikiPath(c.WikiRoot, topicPath); err != nil {
		return fallbackUncategorized(c.WikiRoot, candidates), nil
	}
	if !candidatePathSet(candidates)[topicPath] {
		return fallbackUncategorized(c.WikiRoot, candidates), nil
	}

	return topicPath, nil
}

// fallbackUncategorized returns "zzz/ss/uncategorized" if it exists in the
// candidate set, otherwise creates and returns it as a hardcoded fallback.
func fallbackUncategorized(wikiRoot string, candidates []ghindex.TopicCandidate) string {
	if candidatePathSet(candidates)["zzz/ss/uncategorized"] {
		return "zzz/ss/uncategorized"
	}
	// Even if not in candidates, accept it — WriteSummary will create the dir.
	return "zzz/ss/uncategorized"
}

func validateAIClassificationSummary(result *aiClassification) (*StructuredSummary, error) {
	if result.Summary == nil {
		return nil, errors.New("empty summary")
	}
	if strings.TrimSpace(result.Summary.Overview) == "" {
		return nil, errors.New("empty summary")
	}

	return result.Summary, nil
}

func rejectedClassifyResult(result *aiClassification, detectedContentType string, rejectErr error) *ClassifyResult {
	if result == nil {
		return nil
	}
	reason := "classification rejected"
	if rejectErr != nil {
		reason = rejectErr.Error()
	}
	contentType := detectedContentType
	if contentType == "" {
		contentType = result.ContentType
	}

	return &ClassifyResult{
		TopicPath:         strings.TrimSpace(result.TopicPath),
		WikiType:          result.WikiType,
		ContentType:       contentType,
		Summary:           result.Summary,
		Confidence:        result.Confidence,
		NeedsManualReview: result.NeedsManualReview,
		RejectReason:      reason,
	}
}

func candidatePathSet(candidates []ghindex.TopicCandidate) map[string]bool {
	return lo.SliceToMap(candidates, func(c ghindex.TopicCandidate) (string, bool) {
		return c.Path, true
	})
}

func isValidClassifyType(typ ClassifyType) bool {
	switch typ {
	case TypeRepoEval, TypeDeepDive, TypeInbox:
		return true
	default:
		return false
	}
}

func isValidContentType(contentType string) bool {
	switch contentType {
	case ContentText, ContentVideo, ContentAudio:
		return true
	default:
		return false
	}
}

// ValidateRelativeWikiPath ensures a relative path doesn't escape wikiRoot.
func ValidateRelativeWikiPath(wikiRoot, relativePath string) error {
	if err := validateWikiPathInput(wikiRoot, relativePath); err != nil {
		return err
	}
	if err := validateWikiPathSegments(relativePath); err != nil {
		return err
	}

	return ensureWithinWikiRoot(wikiRoot, relativePath)
}

func validateWikiPathInput(wikiRoot, relativePath string) error {
	if wikiRoot == "" {
		return errors.New("wiki root is empty")
	}
	if relativePath == "" {
		return errors.New("relative path is empty")
	}
	if filepath.IsAbs(relativePath) {
		return fmt.Errorf("absolute path not allowed: %s", relativePath)
	}

	return nil
}

func validateWikiPathSegments(relativePath string) error {
	segments := strings.SplitSeq(relativePath, "/")
	for seg := range segments {
		if seg == "" || seg == "." || seg == ".." {
			return fmt.Errorf("invalid segment: %q", seg)
		}
		if strings.ContainsAny(seg, "\\\x00\n\r") {
			return fmt.Errorf("invalid characters in segment: %q", seg)
		}
	}

	return nil
}

func ensureWithinWikiRoot(wikiRoot, relativePath string) error {
	root := filepath.Clean(wikiRoot)
	resolved := filepath.Clean(filepath.Join(root, relativePath))
	rel, err := filepath.Rel(root, resolved)
	if err != nil {
		return fmt.Errorf("resolve relative path: %w", err)
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return fmt.Errorf("path traversal detected: %s escapes %s", relativePath, wikiRoot)
	}

	return nil
}

// ErrClassificationUnavailable is returned when AI classification is not available.
var ErrClassificationUnavailable = errors.New("classification unavailable")

func truncate(s string, maxLen int) string {
	return textutil.TruncateUTF8(s, maxLen)
}

// ClassifyContent classifies content to determine topic path.
// This is a shared function that can be used by both wiki and ccx.
func ClassifyContent(content, wikiRoot string, aiConfig *ai.ClientConfig) (string, error) {
	classifier := NewClassifier(aiConfig, wikiRoot, "https://cdn.lucc.dev/gh.yml")

	// Truncate content for classification (use first 2000 chars for speed)
	if len(content) > 2000 {
		content = content[:2000]
	}

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	result := classifier.ClassifyURL(ctx, "session-export", "Session Export", content)
	if result == nil {
		return "", errors.New("classification returned nil")
	}

	return result.TopicPath, nil
}
