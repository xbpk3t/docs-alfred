package transcript

import (
	"context"
	"errors"
	"fmt"
	"html"
	"net/http"
	"net/url"
	"os/exec"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/asticode/go-astisub"
	"github.com/xbpk3t/docs-alfred/pkg/httputil"
	"mvdan.cc/xurls/v2"
)

// TranscriptResult holds the result of a transcript fetch.
type TranscriptResult struct {
	Content      string `json:"content"`
	ContentType  string `json:"contentType"` // plaintext, vtt, srt, json, html
	Source       string `json:"source"`      // provider name
	EpisodeTitle string `json:"episodeTitle"`
	EpisodeURL   string `json:"episodeUrl,omitempty"`
	FeedTitle    string `json:"feedTitle,omitempty"`
	FeedURL      string `json:"feedUrl,omitempty"`
}

// Provider defines the interface for transcript providers.
type Provider interface {
	// Name returns the provider identifier.
	Name() string

	// Fetch retrieves the transcript for the given episode.
	Fetch(ctx context.Context, ep *EpisodeRef) (*TranscriptResult, error)
}

const (
	plaintextContentType = "plaintext"
	vttContentType  = "vtt"
	srtContentType  = "srt"
	jsonContentType = "json"
	htmlContentType = "html"
)

// EpisodeRef identifies a single podcast episode.
type EpisodeRef struct {
	Title           string
	URL             string
	GUID            string
	Description     string
	Content         string
	FeedTitle       string
	FeedURL         string
	EnclosureURL    string
	TranscriptLinks []TranscriptLink
}

// TranscriptLink represents a podcast:transcript link.
type TranscriptLink struct {
	URL  string
	Type string
}

// --- RssTranscriptProvider ---

// RssTranscriptProvider fetches transcripts from podcast:transcript tags.
// TS equivalent: rss-transcript.ts — actually fetches the URL content.
type RssTranscriptProvider struct {
	HTTPClient *http.Client
}

func NewRssTranscriptProvider() *RssTranscriptProvider {
	return &RssTranscriptProvider{
		HTTPClient: httputil.NewClient(30 * time.Second),
	}
}

func (p *RssTranscriptProvider) Name() string {
	return "rss-transcript"
}

func (p *RssTranscriptProvider) Fetch(ctx context.Context, ep *EpisodeRef) (*TranscriptResult, error) {
	if len(ep.TranscriptLinks) == 0 {
		return nil, errors.New("RSS item has no podcast:transcript tag")
	}

	// Pick best link (plaintext > vtt > srt > json)
	best := pickBestTranscriptLink(ep.TranscriptLinks)
	if best == nil {
		return nil, errors.New("no suitable transcript link found")
	}

	// Actually fetch the transcript URL (like TS rss-transcript.ts does)
	client := p.HTTPClient
	if client == nil {
		client = httputil.NewClient(30 * time.Second)
	}

	data, err := httputil.Get(client, best.URL)
	if err != nil {
		return nil, fmt.Errorf("fetch transcript URL: %w", err)
	}

	content := string(data)
	contentType := coerceContentType(best.Type)
	normalized := normalizeTranscriptContent(content, contentType)

	return &TranscriptResult{
		Content:     normalized,
		ContentType: contentType,
		Source:      "rss-transcript",
	}, nil
}

func pickBestTranscriptLink(links []TranscriptLink) *TranscriptLink {
	if len(links) == 0 {
		return nil
	}

	rank := map[string]int{
		"text/plain": 4, "text/vtt": 3, "text/srt": 2,
		"application/json": 1, "text/html": 0,
	}

	best := &links[0]
	bestScore := getLinkScore(best, rank)
	for i := 1; i < len(links); i++ {
		if s := getLinkScore(&links[i], rank); s > bestScore {
			best = &links[i]
			bestScore = s
		}
	}

	return best
}

func getLinkScore(link *TranscriptLink, rank map[string]int) int {
	base := rank[strings.ToLower(link.Type)]
	// Prefer URLs that don't need further redirection
	if strings.Contains(strings.ToLower(link.URL), "transcript") {
		base += 1
	}

	return base
}

// --- DescriptionLinkProvider ---
// TS equivalent: description-link.ts — extracts transcript URLs from description/content
// using href regex + URL regex patterns.

type DescriptionLinkProvider struct {
	HTTPClient *http.Client
}

func NewDescriptionLinkProvider() *DescriptionLinkProvider {
	return &DescriptionLinkProvider{
		HTTPClient: httputil.NewClient(30 * time.Second),
	}
}

func (p *DescriptionLinkProvider) Name() string {
	return "description-link"
}

func (p *DescriptionLinkProvider) Fetch(ctx context.Context, ep *EpisodeRef) (*TranscriptResult, error) {
	source := ep.Content
	if source == "" {
		source = ep.Description
	}
	if source == "" {
		return nil, errors.New("no description or content to search")
	}

	links := extractTranscriptLinksFromText(source, ep.URL)
	if len(links) == 0 {
		return nil, errors.New("description/content has no transcript link")
	}

	// Fetch the first matching URL
	client := p.HTTPClient
	if client == nil {
		client = httputil.NewClient(30 * time.Second)
	}

	data, err := httputil.Get(client, links[0])
	if err != nil {
		return nil, fmt.Errorf("fetch description link: %w", err)
	}

	content := string(data)
	contentType := detectContentTypeFromURL(links[0])
	normalized := normalizeTranscriptContent(content, contentType)

	return &TranscriptResult{
		Content:     normalized,
		ContentType: contentType,
		Source:      "description-link",
	}, nil
}

// extractTranscriptLinksFromText extracts transcript URLs from HTML text.
// Uses goquery for href extraction, xurls for bare URL detection,
// and html.UnescapeString for entity decoding.
func extractTranscriptLinksFromText(text, baseURL string) []string {
	var candidates []string

	// Extract href="..." candidates via goquery
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(text))
	if err == nil {
		doc.Find("a[href]").Each(func(_ int, sel *goquery.Selection) {
			if href, ok := sel.Attr("href"); ok {
				candidate := normalizeCandidateURL(href, baseURL)
				if candidate != "" {
					candidates = append(candidates, candidate)
				}
			}
		})
	}

	// Extract bare URL candidates via xurls (after HTML entity decoding)
	decoded := html.UnescapeString(text)
	rxRelaxed := xurls.Relaxed()
	for _, match := range rxRelaxed.FindAllString(decoded, -1) {
		candidate := normalizeCandidateURL(match, baseURL)
		if candidate != "" {
			candidates = append(candidates, candidate)
		}
	}

	// Deduplicate and filter
	seen := make(map[string]bool)
	var result []string
	for _, c := range candidates {
		if !seen[c] && isTranscriptURL(c) {
			seen[c] = true
			result = append(result, c)
		}
	}

	return result
}

func normalizeCandidateURL(raw, baseURL string) string {
	raw = strings.TrimSpace(raw)
	// Trim trailing punctuation
	raw = strings.TrimRight(raw, "),.;\\]")

	// Decode HTML entities
	raw = html.UnescapeString(raw)

	// Parse as URL (with optional base URL)
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}

	if !u.IsAbs() && baseURL != "" {
		base, err := url.Parse(baseURL)
		if err != nil {
			return ""
		}
		u = base.ResolveReference(u)
	}

	if u.Scheme == "" {
		u.Scheme = "https"
	}

	result := u.String()
	if result == "" {
		return ""
	}

	return result
}

func isTranscriptURL(rawURL string) bool {
	lower := strings.ToLower(rawURL)
	if strings.Contains(lower, "transcript") {
		return true
	}
	// Check file extensions
	for _, ext := range []string{".json", ".srt", ".txt", ".vtt"} {
		if strings.Contains(lower, ext) {
			// Verify it's a file extension (not just part of a word)
			idx := strings.Index(lower, ext)
			if idx > 0 && (idx+len(ext) >= len(lower) || lower[idx+len(ext)] == '?' || lower[idx+len(ext)] == '#') {
				return true
			}
		}
	}

	return false
}

func detectContentTypeFromURL(rawURL string) string {
	lower := strings.ToLower(rawURL)
	switch {
	case strings.Contains(lower, ".vtt"):
		return vttContentType
	case strings.Contains(lower, ".srt"):
		return srtContentType
	case strings.Contains(lower, ".json"):
		return jsonContentType
	case strings.Contains(lower, ".txt"):
		return plaintextContentType
	default:
		return plaintextContentType
	}
}

// --- AudioTranscriptionProvider ---

type AudioTranscriptionProvider struct {
	CLIPath  string
	Language string
}

func NewAudioTranscriptionProvider(cliPath, language string) *AudioTranscriptionProvider {
	if cliPath == "" {
		cliPath = "pt"
	}
	if language == "" {
		language = "en"
	}

	return &AudioTranscriptionProvider{CLIPath: cliPath, Language: language}
}

func (p *AudioTranscriptionProvider) Name() string {
	return "audio-asr"
}

func (p *AudioTranscriptionProvider) Fetch(ctx context.Context, ep *EpisodeRef) (*TranscriptResult, error) {
	if ep.EnclosureURL == "" {
		return nil, errors.New("no audio enclosure URL for ASR")
	}

	cmd := exec.CommandContext(ctx, p.CLIPath,
		"--language", p.Language,
		"--output-format", "txt",
		ep.EnclosureURL,
	)

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("asr failed: %w", err)
	}

	content := strings.TrimSpace(string(output))
	if content == "" {
		return nil, errors.New("asr produced empty transcript")
	}

	return &TranscriptResult{
		Content:     content,
		ContentType: plaintextContentType,
		Source:      "audio-asr",
	}, nil
}

// --- Pipeline ---

type Pipeline struct {
	providers []Provider
}

func NewPipeline(providers ...Provider) *Pipeline {
	return &Pipeline{providers: providers}
}

func (p *Pipeline) Fetch(ctx context.Context, ep *EpisodeRef) (*TranscriptResult, string, error) {
	var lastErr error
	for _, provider := range p.providers {
		result, err := provider.Fetch(ctx, ep)
		if err == nil && result != nil && result.Content != "" {
			return result, provider.Name(), nil
		}
		if err != nil {
			lastErr = err
		}
	}
	if lastErr == nil {
		lastErr = errors.New("all providers failed to produce transcript")
	}

	return nil, "", lastErr
}

// --- Content normalization ---
// TS equivalent: content.ts — normalizeTranscriptContent

func normalizeTranscriptContent(content, contentType string) string {
	switch contentType {
	case "vtt", "srt":
		return cleanSubtitle(content, contentType)
	case "json":
		return content // Pass through JSON as-is
	default:
		return strings.TrimSpace(content)
	}
}

// cleanSubtitle parses VTT/SRT content via go-astisub and extracts plain text.
// Handles non-standard formats (ASS-style tags, encoding issues) that yt-dlp
// may produce from sources like Bilibili.
func cleanSubtitle(content, contentType string) string {
	var sub *astisub.Subtitles
	var err error

	r := strings.NewReader(content)
	switch contentType {
	case "vtt":
		sub, err = astisub.ReadFromWebVTT(r)
	case "srt":
		sub, err = astisub.ReadFromSRT(r)
	}
	if err != nil || sub == nil {
		// Fallback: return content stripped of obvious timestamp lines
		return fallbackCleanSubtitle(content)
	}

	var lines []string
	for _, item := range sub.Items {
		for _, line := range item.Lines {
			text := strings.TrimSpace(line.String())
			if text != "" {
				lines = append(lines, text)
			}
		}
	}

	return strings.Join(lines, "\n")
}

// fallbackCleanSubtitle is a minimal stripper used when go-astisub fails to parse.
func fallbackCleanSubtitle(content string) string {
	var lines []string
	for line := range strings.SplitSeq(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.Contains(trimmed, "-->") {
			continue
		}
		lines = append(lines, trimmed)
	}

	return strings.Join(lines, "\n")
}

var contentTypeMap = map[string]string{
	"text/plain": plaintextContentType,
	"text/vtt":   "vtt",
	"text/srt":   srtContentType,
	"application/json": jsonContentType,
	"text/html":  htmlContentType,
}

func coerceContentType(t string) string {
	if v, ok := contentTypeMap[t]; ok {
		return v
	}
	// Fallback: check by substring
	switch {
	case strings.Contains(t, "plain"):
		return plaintextContentType
	case strings.Contains(t, "vtt"):
		return vttContentType
	case strings.Contains(t, "srt"):
		return srtContentType
	case strings.Contains(t, "json"):
		return jsonContentType
	case strings.Contains(t, "html"):
		return htmlContentType
	}

	return plaintextContentType
}
