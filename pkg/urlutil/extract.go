package urlutil

import (
	"html"
	"net/url"
	"path"
	"regexp"
	"slices"
	"sort"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"mvdan.cc/xurls/v2"
)

// URLRef is a URL found in source text. Start and End are byte offsets in the
// original text when the extractor can preserve them.
type URLRef struct {
	URL        string
	Normalized string
	Start      int
	End        int
}

// ExtractOptions configures URL extraction behavior.
type ExtractOptions struct {
	BaseURL        string
	Markdown       bool
	BareURLs       bool
	HTMLAnchors    bool
	Relaxed        bool
	HTTPOnly       bool
	Normalize      bool
	Deduplicate    bool
	TranscriptOnly bool
}

var markdownLinkURLPattern = regexp.MustCompile(`\[[^\]]+\]\((https?://[^)\s]+)(?:\s+"[^"]*")?\)`)

// ExtractURLRefs extracts URL references from text according to opts.
func ExtractURLRefs(text string, opts ExtractOptions) []URLRef {
	var refs []URLRef
	if opts.Markdown {
		refs = append(refs, extractMarkdownURLRefs(text, opts)...)
	}
	if opts.HTMLAnchors {
		refs = append(refs, extractHTMLAnchorURLRefs(text, opts)...)
	}
	if opts.BareURLs {
		refs = append(refs, extractBareURLRefs(text, refs, opts)...)
	}
	sort.SliceStable(refs, func(i, j int) bool { return refs[i].Start < refs[j].Start })
	if opts.Deduplicate {
		refs = dedupeURLRefs(refs)
	}

	return refs
}

func extractMarkdownURLRefs(text string, opts ExtractOptions) []URLRef {
	matches := markdownLinkURLPattern.FindAllStringSubmatchIndex(text, -1)
	refs := make([]URLRef, 0, len(matches))
	for _, match := range matches {
		if len(match) < 4 || match[2] < 0 || match[3] < match[2] {
			continue
		}
		cleaned := CleanURL(text[match[2]:match[3]], CleanOptions{BaseURL: opts.BaseURL, HTTPOnly: opts.HTTPOnly})
		if !keepExtractedURL(cleaned, opts) {
			continue
		}
		refs = append(refs, newURLRef(cleaned, match[0], match[1], opts.Normalize))
	}

	return refs
}

func extractHTMLAnchorURLRefs(text string, opts ExtractOptions) []URLRef {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(text))
	if err != nil {
		return nil
	}

	var refs []URLRef
	doc.Find("a[href]").Each(func(_ int, sel *goquery.Selection) {
		href, ok := sel.Attr("href")
		if !ok {
			return
		}
		cleaned := CleanURL(href, CleanOptions{BaseURL: opts.BaseURL, HTTPOnly: opts.HTTPOnly, AssumeHTTPS: true})
		if !keepExtractedURL(cleaned, opts) {
			return
		}
		refs = append(refs, newURLRef(cleaned, -1, -1, opts.Normalize))
	})

	return refs
}

func extractBareURLRefs(text string, masked []URLRef, opts ExtractOptions) []URLRef {
	searchText := text
	if opts.Relaxed {
		searchText = html.UnescapeString(text)
	} else {
		searchText = maskRanges(searchText, masked)
	}

	rx := xurls.Strict()
	if opts.Relaxed {
		rx = xurls.Relaxed()
	}

	var refs []URLRef
	for _, match := range rx.FindAllString(searchText, -1) {
		start := strings.Index(searchText, match)
		if start < 0 {
			continue
		}
		cleaned := CleanURLWithTrim(match, CleanOptions{BaseURL: opts.BaseURL, HTTPOnly: opts.HTTPOnly, AssumeHTTPS: opts.Relaxed})
		if !keepExtractedURL(cleaned.URL, opts) {
			searchText = replaceRangeWithSpaces(searchText, start, start+len(match))

			continue
		}
		refs = append(refs, newURLRef(
			cleaned.URL,
			start+cleaned.LeftTrim,
			start+len(match)-cleaned.RightTrim,
			opts.Normalize,
		))
		searchText = replaceRangeWithSpaces(searchText, start, start+len(match))
	}

	return refs
}

func keepExtractedURL(candidate string, opts ExtractOptions) bool {
	if candidate == "" {
		return false
	}
	if opts.TranscriptOnly && !IsTranscriptURL(candidate) {
		return false
	}

	return true
}

func newURLRef(rawURL string, start, end int, normalize bool) URLRef {
	ref := URLRef{URL: rawURL, Start: start, End: end}
	if normalize {
		ref.Normalized = Normalize(rawURL)
	}

	return ref
}

func dedupeURLRefs(refs []URLRef) []URLRef {
	seen := make(map[string]bool, len(refs))
	result := refs[:0]
	for _, ref := range refs {
		key := ref.Normalized
		if key == "" {
			key = Normalize(ref.URL)
		}
		if seen[key] {
			continue
		}
		seen[key] = true
		result = append(result, ref)
	}

	return result
}

// CleanOptions configures URL cleanup.
type CleanOptions struct {
	BaseURL     string
	HTTPOnly    bool
	AssumeHTTPS bool
}

// CleanedURL contains a cleaned URL and the number of bytes trimmed from both sides.
type CleanedURL struct {
	URL       string
	LeftTrim  int
	RightTrim int
}

// CleanHTTPURL trims common surrounding punctuation and accepts only absolute HTTP(S) URLs.
func CleanHTTPURL(raw string) string {
	return CleanURL(raw, CleanOptions{HTTPOnly: true})
}

// CleanURL cleans and validates a URL.
func CleanURL(raw string, opts CleanOptions) string {
	return CleanURLWithTrim(raw, opts).URL
}

// CleanURLWithTrim cleans and validates a URL while preserving trim counts.
func CleanURLWithTrim(raw string, opts CleanOptions) CleanedURL {
	raw = strings.TrimSpace(raw)
	trim := urlTrimBounds(raw)
	if len(raw)-trim.right < trim.left {
		return CleanedURL{LeftTrim: trim.left, RightTrim: trim.right}
	}
	candidate := raw[trim.left : len(raw)-trim.right]
	candidate = html.UnescapeString(candidate)
	cleaned := parseCleanURL(candidate, opts)

	return CleanedURL{URL: cleaned, LeftTrim: trim.left, RightTrim: trim.right}
}

type trimBounds struct {
	left  int
	right int
}

func urlTrimBounds(raw string) trimBounds {
	leftTrim := 0
	for leftTrim < len(raw) && raw[leftTrim] == '<' {
		leftTrim++
	}
	rightTrim := 0
	for len(raw)-rightTrim > leftTrim {
		ch := raw[len(raw)-rightTrim-1]
		if !strings.ContainsRune("'\".,;:!?)\\]>", rune(ch)) {
			break
		}
		rightTrim++
	}

	return trimBounds{left: leftTrim, right: rightTrim}
}

func parseCleanURL(candidate string, opts CleanOptions) string {
	parsed, ok := parseURLCandidate(candidate, opts.AssumeHTTPS)
	if !ok {
		return ""
	}
	parsed, ok = resolveURLReference(parsed, opts.BaseURL)
	if !ok {
		return ""
	}
	if opts.AssumeHTTPS && parsed.Scheme == "" {
		parsed.Scheme = "https"
	}
	if !validCleanURL(parsed, opts.HTTPOnly) {
		return ""
	}

	return parsed.String()
}

func parseURLCandidate(candidate string, assumeHTTPS bool) (*url.URL, bool) {
	if candidate == "" || strings.Contains(candidate, "](") {
		return nil, false
	}
	parsed, err := url.Parse(candidate)
	if err != nil {
		return nil, false
	}
	if assumeHTTPS && parsed.Scheme == "" && parsed.Host == "" && looksLikeDomainURL(candidate) {
		parsed, err = url.Parse("https://" + candidate)
		if err != nil {
			return nil, false
		}
	}

	return parsed, true
}

func resolveURLReference(parsed *url.URL, baseURL string) (*url.URL, bool) {
	if parsed.IsAbs() || baseURL == "" {
		return parsed, true
	}
	base, err := url.Parse(baseURL)
	if err != nil {
		return nil, false
	}

	return base.ResolveReference(parsed), true
}

func validCleanURL(parsed *url.URL, httpOnly bool) bool {
	if httpOnly && parsed.Scheme != "http" && parsed.Scheme != "https" {
		return false
	}

	return parsed.Scheme != "" && parsed.Host != ""
}

func looksLikeDomainURL(candidate string) bool {
	if candidate == "" || strings.HasPrefix(candidate, "/") || strings.HasPrefix(candidate, "./") || strings.HasPrefix(candidate, "../") {
		return false
	}
	first, _, _ := strings.Cut(candidate, "/")

	return strings.Contains(first, ".")
}

// NormalizeSet returns a set of cleaned, normalized HTTP(S) URLs.
func NormalizeSet(urls []string) map[string]bool {
	set := make(map[string]bool, len(urls))
	for _, raw := range urls {
		cleaned := CleanHTTPURL(raw)
		if cleaned == "" {
			continue
		}
		set[Normalize(cleaned)] = true
	}

	return set
}

// IsTranscriptURL reports whether a URL points to or names transcript content.
func IsTranscriptURL(rawURL string) bool {
	lower := strings.ToLower(rawURL)
	if strings.Contains(lower, "transcript") {
		return true
	}

	switch urlExtension(lower) {
	case ".json", ".srt", ".txt", ".vtt":
		return true
	default:
		return false
	}
}

func urlExtension(rawURL string) string {
	if parsed, err := url.Parse(rawURL); err == nil {
		return strings.ToLower(path.Ext(parsed.Path))
	}

	return strings.ToLower(path.Ext(rawURL))
}

// CountURLs counts URL-looking references in text.
func CountURLs(text string) int {
	return len(ExtractURLRefs(text, ExtractOptions{BareURLs: true, HTTPOnly: true}))
}

func maskRanges(s string, refs []URLRef) string {
	masked := s
	ordered := append([]URLRef(nil), refs...)
	slices.SortStableFunc(ordered, func(a, b URLRef) int {
		if a.Start < b.Start {
			return -1
		}
		if a.Start > b.Start {
			return 1
		}

		return 0
	})
	for _, ref := range ordered {
		masked = replaceRangeWithSpaces(masked, ref.Start, ref.End)
	}

	return masked
}

func replaceRangeWithSpaces(s string, start, end int) string {
	if start < 0 || end < start || start > len(s) || end > len(s) {
		return s
	}

	return s[:start] + strings.Repeat(" ", end-start) + s[end:]
}
