package urlutil

import (
	"net/url"
	"strings"
)

// Tracking-parameter removal is driven by the embedded ClearURLs community
// rule set (data/data.minify.json; see clearurls.go for provenance). A URL
// differing only in these parameters is treated as the same resource for dedup
// purposes.
//
// Content-bearing parameters (?v=, ?bvid=, ?q=, ...) are guarded and never
// stripped, per the guardParams set in clearurls.go.

// NormalizeForDedup canonicalizes a URL so that URLs differing only in
// tracking parameters, parameter order, fragment, default ports, scheme/host
// case, or a trailing slash compare equal. Content-bearing query parameters
// are preserved.
//
// It layers the ClearURLs engine (host-scoped tracking-param stripping,
// redirection expansion, raw rewrites) on top of Normalize, which then handles
// scheme/host case folding, default-port removal, dot-segment removal, fragment
// removal, duplicate-slash removal, query sorting, and trailing-slash trimming.
func NormalizeForDedup(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return strings.TrimRight(rawURL, "/")
	}

	// Apply the host-scoped ClearURLs engine first; it may rewrite the URL to
	// a different (real) host, so re-parse before canonicalizing.
	cleaned := cleanURL(rawURL)
	parsed, err := url.Parse(cleaned)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		parsed = u
	}
	return Normalize(parsed.String())
}
