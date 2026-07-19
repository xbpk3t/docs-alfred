package classify

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/xbpk3t/docs-alfred/internal/docs/wiki/fetch"
	"github.com/xbpk3t/docs-alfred/internal/docs/wiki/prompt"
	"github.com/xbpk3t/docs-alfred/internal/docs/wiki/types"
	"log/slog"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/avast/retry-go/v4"
	"github.com/samber/lo"
	"github.com/xbpk3t/docs-alfred/internal/gh/index"
	"github.com/xbpk3t/docs-alfred/pkg/ai"
	"github.com/xbpk3t/docs-alfred/pkg/textutil"
	"github.com/xbpk3t/docs-alfred/pkg/validator"
)

const noneVal = "none"

// minContentForVideo is the minimum content length (in runes) for video content
// to be classified. Below this threshold the fetched content is likely just
// metadata (title, stats, description) without a transcript, making classification
// unreliable.
const minContentForVideo = 600

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
func WithGHTopicsCachePath(cachePath string) ClassifierOption {
	return func(c *Classifier) { c.GhTopicsCachePath = cachePath }
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
// ClassifyURL performs full classification on a URL with fetched title + content.
// Uses a two-step pipeline: classify (topic+type+metadata) then summarize (detailed summary).
// Returns nil if classification is unavailable (graceful degradation).
func (c *Classifier) ClassifyURL(ctx context.Context, urlStr, title, content string) *types.ClassifyResult {
	contentType := fetch.DetectContentType(strings.ToLower(urlStr))
	if strings.TrimSpace(content) == "" {
		slog.Warn("Classification skipped for empty content", "url", urlStr)

		return nil
	}

	// For video content, require transcript-length content. Short content
	// (e.g., 492 bytes of bilibili metadata without subtitles) can't produce
	// meaningful summaries; treat as extraction failure instead of routing to
	// uncategorized.
	if contentType == types.ContentVideo && len([]rune(content)) < minContentForVideo {
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

// buildClassifyResult processes the AI classification result and builds a types.ClassifyResult.
// It handles manual review, rejection, validation, metadata building, and empty summary checks.
//
// Recall-oriented routing: when the AI marks needsManualReview but still provides a
// candidate-valid topicPath with confidence >= MinConfidence and a non-empty overview,
// the item is treated as a normal topic write (NMR is cleared).
func (c *Classifier) buildClassifyResult(classified *aiClassification, contentType string, candidates []ghindex.TopicCandidate, urlStr string) *types.ClassifyResult {
	if classified == nil {
		return nil
	}

	suggested := suggestedTopicFromAI(classified.TopicPath)

	if classified.Summary == nil || strings.TrimSpace(classified.Summary.Overview) == "" {
		if classified.RejectReason != "" {
			return rejectedClassifyResult(classified, contentType, errors.New(classified.RejectReason))
		}
		slog.Warn("Empty summary from classification", "url", urlStr)

		return nil
	}

	if classified.RejectReason != "" {
		return rejectedClassifyResult(classified, contentType, errors.New(classified.RejectReason))
	}

	// NMR + good summary: try to promote to topic when path is usable; else uncat with reason.
	if classified.NeedsManualReview {
		return c.routeManualReviewWithGoodContent(classified, contentType, candidates, suggested)
	}

	if validationErr := c.validateAIClassificationBasics(classified); validationErr != nil {
		// Low confidence with good summary → uncat (not hard reject), keep suggested topic.
		if classified.Confidence < c.MinConfidence {
			return c.manualReviewRouteResult(classified, contentType, suggested, types.RouteReasonNeedsManualReview)
		}
		slog.Warn("AI classification rejected", "url", urlStr, "error", validationErr)

		return rejectedClassifyResult(classified, contentType, validationErr)
	}

	topicPath, topicOK := c.resolveWritableTopicPath(classified.TopicPath, candidates)
	if !topicOK {
		reason := types.RouteReasonNoTopicMatch
		raw := strings.TrimSpace(classified.TopicPath)
		if raw != "" && raw != noneVal && raw != "inbox" {
			reason = types.RouteReasonInvalidTopicPath
		}

		return c.manualReviewRouteResult(classified, contentType, suggested, reason)
	}

	metaBlock := buildMetaBlock(classified)

	return &types.ClassifyResult{
		TopicPath:         topicPath,
		WikiType:          classified.WikiType,
		ContentType:       contentType,
		Summary:           classified.Summary,
		MetadataBlock:     metaBlock,
		SuggestedTopic:    suggested,
		Confidence:        classified.Confidence,
		NeedsManualReview: false,
	}
}

// routeManualReviewWithGoodContent implements recall-oriented NMR handling.
func (c *Classifier) routeManualReviewWithGoodContent(
	classified *aiClassification,
	contentType string,
	candidates []ghindex.TopicCandidate,
	suggested string,
) *types.ClassifyResult {
	topicPath, topicOK := c.resolveWritableTopicPath(classified.TopicPath, candidates)
	if topicOK && classified.Confidence >= c.MinConfidence {
		metaBlock := buildMetaBlock(classified)
		wikiType := classified.WikiType
		if !isValidClassifyType(wikiType) || wikiType == types.TypeInbox {
			wikiType = types.TypeDeepDive
		}

		return &types.ClassifyResult{
			TopicPath:         topicPath,
			WikiType:          wikiType,
			ContentType:       contentType,
			Summary:           classified.Summary,
			MetadataBlock:     metaBlock,
			SuggestedTopic:    suggested,
			Confidence:        classified.Confidence,
			NeedsManualReview: false,
		}
	}

	reason := types.RouteReasonNeedsManualReview
	if !topicOK {
		raw := strings.TrimSpace(classified.TopicPath)
		if raw == "" || raw == noneVal || raw == "inbox" {
			reason = types.RouteReasonNoTopicMatch
		} else {
			reason = types.RouteReasonInvalidTopicPath
		}
	}

	return c.manualReviewRouteResult(classified, contentType, suggested, reason)
}

// manualReviewRouteResult builds an uncat-bound result with preserved summary + attribution.
func (c *Classifier) manualReviewRouteResult(
	classified *aiClassification,
	contentType string,
	suggested string,
	reason string,
) *types.ClassifyResult {
	wikiType := classified.WikiType
	if !isValidClassifyType(wikiType) {
		wikiType = types.TypeInbox
	}

	return &types.ClassifyResult{
		TopicPath:         "",
		WikiType:          wikiType,
		ContentType:       contentType,
		Summary:           classified.Summary,
		MetadataBlock:     "",
		SuggestedTopic:    suggested,
		RouteReason:       reason,
		Confidence:        classified.Confidence,
		NeedsManualReview: true,
	}
}

// suggestedTopicFromAI keeps AI's original path for observability when non-sentinel.
func suggestedTopicFromAI(topicPath string) string {
	topicPath = strings.TrimSpace(topicPath)
	if topicPath == "" || topicPath == noneVal || topicPath == "inbox" {
		return ""
	}

	return topicPath
}

// resolveWritableTopicPath returns a candidate-valid depth-3 path, or false.
// Exact match first; then fuzzy leaf match (strip ***, parens, wrong parent type).
func (c *Classifier) resolveWritableTopicPath(topicPath string, candidates []ghindex.TopicCandidate) (string, bool) {
	topicPath = strings.TrimSpace(topicPath)
	if topicPath == "" || topicPath == noneVal || topicPath == "inbox" {
		return "", false
	}
	if err := ValidateRelativeWikiPath(c.WikiRoot, topicPath); err != nil {
		// still try fuzzy — AI often invents almost-right paths
		if resolved, ok := fuzzyMatchTopicPath(topicPath, candidates); ok {
			return resolved, true
		}

		return "", false
	}
	if ValidateTopicPathDepth(topicPath) {
		if len(candidates) == 0 || candidatePathSet(candidates)[topicPath] {
			return topicPath, true
		}
	}
	if resolved, ok := fuzzyMatchTopicPath(topicPath, candidates); ok {
		return resolved, true
	}

	return "", false
}

// normalizeTopicLeafKey strips markdown stars, parenthetical notes, and lowercases
// for leaf comparison (e.g. "***golang代码常用写法***（…）" → "golang代码常用写法").
func normalizeTopicLeafKey(s string) string {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, "*", "")
	s = stripParentheticalNotes(s)
	s = strings.Join(strings.Fields(s), "")

	return strings.ToLower(s)
}

func stripParentheticalNotes(s string) string {
	for {
		next := stripOneParenthetical(s, "（", "）")
		next = stripOneParenthetical(next, "(", ")")
		next = strings.TrimSpace(next)
		if next == s {
			return s
		}
		s = next
	}
}

func stripOneParenthetical(s, open, closeDelim string) string {
	i := strings.Index(s, open)
	if i < 0 {
		return s
	}
	j := strings.Index(s[i:], closeDelim)
	if j < 0 {
		return s
	}

	return s[:i] + s[i+j+len(closeDelim):]
}

// leafKeysSimilar reports whether two normalized leaf keys are a confident match.
func leafKeysSimilar(a, b string) bool {
	if a == "" || b == "" {
		return false
	}
	if a == b {
		return true
	}
	if len(a) < 4 && len(b) < 4 {
		return false
	}
	if !strings.Contains(a, b) && !strings.Contains(b, a) {
		return false
	}
	short, long := a, b
	if len(short) > len(long) {
		short, long = long, short
	}

	return len(short) >= 4 && len(short)*2 >= len(long)
}

type fuzzyTopicHit struct {
	path string
	tag  string
	typ  string
}

func candidateMatchesLeaf(c ghindex.TopicCandidate, leafKey string) bool {
	parts := strings.Split(strings.Trim(c.Path, "/"), "/")
	if len(parts) != 3 {
		return false
	}
	if leafKeysSimilar(normalizeTopicLeafKey(parts[2]), leafKey) {
		return true
	}

	return leafKeysSimilar(normalizeTopicLeafKey(c.Display), leafKey)
}

func collectFuzzyTopicHits(leafKey string, candidates []ghindex.TopicCandidate) []fuzzyTopicHit {
	var hits []fuzzyTopicHit
	seen := map[string]bool{}
	for _, c := range candidates {
		if !candidateMatchesLeaf(c, leafKey) || seen[c.Path] {
			continue
		}
		parts := strings.Split(strings.Trim(c.Path, "/"), "/")
		if len(parts) != 3 {
			continue
		}
		seen[c.Path] = true
		hits = append(hits, fuzzyTopicHit{path: c.Path, tag: parts[0], typ: parts[1]})
	}

	return hits
}

func pickUniqueHit(hits []fuzzyTopicHit) (string, bool) {
	if len(hits) == 1 {
		return hits[0].path, true
	}

	return "", false
}

func filterHitsByTagType(hits []fuzzyTopicHit, tag, typ string) []fuzzyTopicHit {
	var out []fuzzyTopicHit
	for _, h := range hits {
		if strings.EqualFold(h.tag, tag) && strings.EqualFold(h.typ, typ) {
			out = append(out, h)
		}
	}

	return out
}

func filterHitsByTag(hits []fuzzyTopicHit, tag string) []fuzzyTopicHit {
	var out []fuzzyTopicHit
	for _, h := range hits {
		if strings.EqualFold(h.tag, tag) {
			out = append(out, h)
		}
	}

	return out
}

// disambiguateFuzzyHits prefers same tag/type, then same tag.
func disambiguateFuzzyHits(parts []string, hits []fuzzyTopicHit) (string, bool) {
	if path, ok := pickUniqueHit(hits); ok {
		return path, true
	}
	switch {
	case len(parts) >= 3:
		if path, ok := pickUniqueHit(filterHitsByTagType(hits, parts[0], parts[1])); ok {
			return path, true
		}
		return pickUniqueHit(filterHitsByTag(hits, parts[0]))
	case len(parts) == 2:
		return pickUniqueHit(filterHitsByTag(hits, parts[0]))
	default:
		return "", false
	}
}

// fuzzyMatchTopicPath maps an AI-invented path onto a unique catalog path by leaf.
// Examples:
//
//	kernel/NP/QUIC → kernel/HTTP/QUIC
//	langs/golang/golang代码常用写法 → langs/golang/***golang代码常用写法***（…）
func fuzzyMatchTopicPath(topicPath string, candidates []ghindex.TopicCandidate) (string, bool) {
	if len(candidates) == 0 {
		return "", false
	}
	parts := strings.Split(strings.Trim(topicPath, "/"), "/")
	if len(parts) == 0 {
		return "", false
	}
	leafKey := normalizeTopicLeafKey(parts[len(parts)-1])
	if leafKey == "" {
		return "", false
	}
	hits := collectFuzzyTopicHits(leafKey, candidates)
	if len(hits) == 0 {
		return "", false
	}

	return disambiguateFuzzyHits(parts, hits)
}

// ResolveTopicPathAmong maps topicPath onto a key in valid (exact or fuzzy leaf).
// Used by the write layer when ValidTopicPaths rejects an almost-correct AI path.
func ResolveTopicPathAmong(topicPath string, valid map[string]bool) (string, bool) {
	topicPath = strings.TrimSpace(topicPath)
	if topicPath == "" || valid == nil {
		return "", false
	}
	if valid[topicPath] {
		return topicPath, true
	}
	cands := make([]ghindex.TopicCandidate, 0, len(valid))
	for p := range valid {
		cands = append(cands, ghindex.TopicCandidate{Path: p, Display: filepath.Base(p)})
	}

	return fuzzyMatchTopicPath(topicPath, cands)
}

// classifyOnlyResult holds the parsed JSON from the classify-only AI call.
type classifyOnlyResult struct {
	Summary           *types.StructuredSummary `json:"summary"`
	Metadata          *types.EntryMetadata     `json:"metadata"`
	TopicPath         string                   `json:"topicPath"`
	WikiType          types.ClassifyType       `json:"wikiType"`
	ContentType       string                   `json:"contentType"`
	RejectReason      string                   `json:"rejectReason,omitempty"`
	Confidence        float64                  `json:"confidence"`
	NeedsManualReview bool                     `json:"needsManualReview"`
}

// classifyOnly runs the AI classification call with retry.
// Retries on AI call failure, JSON parse failure, or validation failure.
func (c *Classifier) classifyOnly(
	ctx context.Context,
	urlStr, title, contentType, content string,
	candidates []ghindex.TopicCandidate,
	maxLen int,
) (*aiClassification, error) {
	promptText, err := prompt.Render("classify-json.txt", &promptData{
		CandidateTree: FormatTopicCandidatesGrouped(candidates),
		Title:         truncate(title, 200),
		URL:           urlStr,
		ContentType:   contentType,
		Content:       truncate(content, maxLen),
	})
	if err != nil {
		return nil, fmt.Errorf("render classify prompt: %w", err)
	}

	var result *aiClassification
	err = retry.Do(
		func() error {
			r, e := ai.ChatContext(ctx, c.AIConfig, []ai.Message{{Role: "user", Content: promptText}})
			if e != nil {
				return fmt.Errorf("AI classify call: %w", e)
			}

			parsed, e := parseClassifyOnlyResult(r)
			if e != nil {
				return fmt.Errorf("parse classify JSON: %w", e)
			}

			if e := validateClassifyResult(parsed); e != nil {
				return fmt.Errorf("validate classify result: %w", e)
			}

			result = &aiClassification{
				TopicPath:         parsed.TopicPath,
				WikiType:          parsed.WikiType,
				ContentType:       parsed.ContentType,
				Summary:           parsed.Summary,
				Metadata:          parsed.Metadata,
				Confidence:        parsed.Confidence,
				NeedsManualReview: parsed.NeedsManualReview,
				RejectReason:      parsed.RejectReason,
			}
			return nil
		},
		retry.Attempts(3),
		retry.Delay(1*time.Second),
		retry.DelayType(retry.BackOffDelay),
	)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func parseClassifyOnlyResult(raw string) (*classifyOnlyResult, error) {
	raw = strings.TrimSpace(raw)
	var result classifyOnlyResult
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
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

func (c *Classifier) classificationCandidates(
	_ context.Context,
	_,
	_,
	_ string,
) ([]ghindex.TopicCandidate, error) {
	remote, err := c.ghTopicCatalog()
	if err != nil {
		return nil, err
	}

	return remote, nil
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
	return ghindex.LocalTopicCatalog(ghindex.LocalGHConfig{})
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
	typeEntries, err := os.ReadDir(topPath)
	if err != nil {
		return candidates
	}
	for _, typ := range typeEntries {
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
		topicPath := strings.Join([]string{topName, typ.Name(), topic.Name()}, "/")
		candidates = append(candidates, ghindex.TopicCandidate{Path: topicPath, Display: topic.Name(), Source: "wiki"})
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

type aiClassification struct {
	Summary           *types.StructuredSummary `json:"summary"`
	Metadata          *types.EntryMetadata     `json:"metadata"`
	TopicPath         string                   `json:"topicPath"`
	WikiType          types.ClassifyType       `json:"wikiType"`
	ContentType       string                   `json:"contentType"`
	RejectReason      string                   `json:"rejectReason,omitempty"`
	Confidence        float64                  `json:"confidence"`
	NeedsManualReview bool                     `json:"needsManualReview"`
}

// RenderStructuredSummary converts a types.StructuredSummary to markdown sections.
// Iterates struct fields in order, using JSON tags as headings.
// Add/remove fields in types.StructuredSummary — rendering adapts automatically.
func RenderStructuredSummary(s *types.StructuredSummary) string {
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
func metadataToMap(m *types.EntryMetadata) map[string]string {
	if m == nil {
		return nil
	}
	kv := make(map[string]string, 2)
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
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		return nil, err
	}

	return &result, nil
}

func (c *Classifier) validateAIClassification(
	result *aiClassification,
	candidates []ghindex.TopicCandidate,
	detectedContentType string,
) (*types.ClassifyResult, error) {
	// Shared path with ClassifyURL: recall-oriented NMR + no fake uncategorized path.
	out := c.buildClassifyResult(result, detectedContentType, candidates, "")
	if out == nil {
		return nil, errors.New("classification result unavailable")
	}
	if out.RejectReason != "" {
		return out, errors.New(out.RejectReason)
	}

	return out, nil
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
	topicPath, ok := c.resolveWritableTopicPath(result.TopicPath, candidates)
	if !ok {
		// No writable topic — caller routes to uncat via empty path + NMR.
		return "", nil
	}

	return topicPath, nil
}

// validateTopicPathDepth ensures topicPath has exactly 3 segments (folder/type/topic).
func ValidateTopicPathDepth(topicPath string) bool {
	return strings.Count(topicPath, "/") == 2
}

// fallbackUncategorized is deprecated: do not invent a fake wiki path.
// Kept as empty-string helper for tests/callers that still reference the name.
func fallbackUncategorized(_ string, _ []ghindex.TopicCandidate) string {
	return ""
}

func validateAIClassificationSummary(result *aiClassification) (*types.StructuredSummary, error) {
	if result.Summary == nil {
		return nil, errors.New("empty summary")
	}
	if strings.TrimSpace(result.Summary.Overview) == "" {
		return nil, errors.New("empty summary")
	}

	return result.Summary, nil
}

func rejectedClassifyResult(result *aiClassification, detectedContentType string, rejectErr error) *types.ClassifyResult {
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

	return &types.ClassifyResult{
		TopicPath:         strings.TrimSpace(result.TopicPath),
		WikiType:          result.WikiType,
		ContentType:       contentType,
		Summary:           result.Summary,
		SuggestedTopic:    suggestedTopicFromAI(result.TopicPath),
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

func isValidClassifyType(typ types.ClassifyType) bool {
	switch typ {
	case types.TypeRepoEval, types.TypeDeepDive, types.TypeInbox:
		return true
	default:
		return false
	}
}

func isValidContentType(contentType string) bool {
	switch contentType {
	case types.ContentText, types.ContentVideo, types.ContentAudio:
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
// Topic candidates are loaded from the local gh.yml (/tmp/gh.yml).
func ClassifyContent(content, wikiRoot string, aiConfig *ai.ClientConfig) (string, error) {
	classifier := NewClassifier(aiConfig, wikiRoot, "")
	classifier.loadGHTopics = func() ([]ghindex.TopicCandidate, error) {
		return ghindex.LocalTopicCatalog(ghindex.LocalGHConfig{})
	}

	// Truncate content for classification (use first 2000 chars for speed)
	if len(content) > 2000 {
		content = content[:2000]
	}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Second)
	defer cancel()

	result := classifier.ClassifyURL(ctx, "session-export", "Session Export", content)
	if result == nil {
		return "", errors.New("classification returned nil")
	}

	return result.TopicPath, nil
}

// FormatTopicCandidates formats topic candidates for prompt injection.
func FormatTopicCandidates(candidates []ghindex.TopicCandidate) string {
	return formatTopicCandidates(candidates)
}

// FormatTopicCandidatesGrouped formats topic candidates grouped by tag for progressive classification.
// Output is hierarchical:
//
//	### AI
//	  LLM: LLM, claude-code, model-routing
//	  agent: agent, agent-fwk, agent-memory
//	### algo
//	  algo: 算法思维, 动态规划
func FormatTopicCandidatesGrouped(candidates []ghindex.TopicCandidate) string {
	_ = formatTopicCandidates // silence lint; template function kept for compatibility
	// Group by tag → type → topics.
	groups := make(map[string]map[string][]string)
	var tagOrder []string
	seenTag := make(map[string]bool)
	for _, c := range candidates {
		parts := strings.SplitN(c.Path, "/", 3)
		if len(parts) < 2 {
			continue
		}
		tag := parts[0]
		typ := parts[1]
		topic := ""
		if len(parts) > 2 {
			topic = parts[2]
		}
		if _, ok := groups[tag]; !ok {
			groups[tag] = make(map[string][]string)
			if !seenTag[tag] {
				seenTag[tag] = true
				tagOrder = append(tagOrder, tag)
			}
		}
		if topic != "" && topic != typ {
			groups[tag][typ] = append(groups[tag][typ], topic)
		}
	}

	var lines []string
	for _, tag := range tagOrder {
		lines = append(lines, fmt.Sprintf("### %s", tag))
		byType := groups[tag]
		var typeOrder []string
		for typ := range byType {
			typeOrder = append(typeOrder, typ)
		}
		sort.Strings(typeOrder)
		for _, typ := range typeOrder {
			topics := byType[typ]
			lines = append(lines, fmt.Sprintf("  %s: %s", typ, strings.Join(topics, ", ")))
		}
		lines = append(lines, "")
	}

	return strings.Join(lines, "\n")
}

// LoadClassificationCandidates loads topic candidates from gh.yml.
// Used by ccx session export for topic path validation.
func LoadClassificationCandidates(wikiRoot string) []ghindex.TopicCandidate {
	remote, err := ghindex.LocalTopicCatalog(ghindex.LocalGHConfig{})
	if err != nil {
		slog.Warn("Local topic catalog unavailable", "error", err)

		return nil
	}

	return remote
}
