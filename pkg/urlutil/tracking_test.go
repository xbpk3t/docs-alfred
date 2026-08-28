package urlutil

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNormalizeForDedupStripsTrackingAndSorts(t *testing.T) {
	tests := []struct {
		name   string
		first  string
		second string
		same   bool
	}{
		{
			name:   "tracking params ignored",
			first:  "https://example.com/post?a=1&utm_source=x",
			second: "https://example.com/post?a=1",
			same:   true,
		},
		{
			name:   "param order ignored",
			first:  "https://example.com/post?a=1&b=2",
			second: "https://example.com/post?b=2&a=1",
			same:   true,
		},
		{
			name:   "full utm family ignored",
			first:  "https://example.com/post?utm_source=nl&utm_medium=email&utm_campaign=summer",
			second: "https://example.com/post",
			same:   true,
		},
		{
			name:   "content identifier preserved (youtube v)",
			first:  "https://youtube.com/watch?v=abc123",
			second: "https://youtube.com/watch?v=def456",
			same:   false,
		},
		{
			name:   "content identifier preserved (bilibili bvid)",
			first:  "https://bilibili.com/video/BV1xx411c7mD",
			second: "https://bilibili.com/video/BV1Kw411c7mE",
			same:   false,
		},
		{
			name:   "search query preserved",
			first:  "https://example.com/search?q=go",
			second: "https://example.com/search?q=rust",
			same:   false,
		},
		{
			name:   "same search query with tracking strips to same",
			first:  "https://example.com/search?q=go&utm_source=nl",
			second: "https://example.com/search?q=go",
			same:   true,
		},
		{
			name:   "fragment ignored",
			first:  "https://example.com/post#section-1",
			second: "https://example.com/post#section-2",
			same:   true,
		},
		{
			name:   "trailing slash ignored",
			first:  "https://example.com/post/",
			second: "https://example.com/post",
			same:   true,
		},
		{
			name:   "host case and default port ignored",
			first:  "https://Example.com:443/post",
			second: "https://example.com/post",
			same:   true,
		},
		{
			name:   "different path not merged",
			first:  "https://example.com/a",
			second: "https://example.com/b",
			same:   false,
		},
		{
			// ClearURLs' global utm rule `(?:%3F)?utm(?:_[a-z_]*)?` is
			// unanchored, so any param starting with the literal "utm" is
			// stripped, not just utm_* with an underscore.
			name:   "utm prefix stripped by ClearURLs global rule",
			first:  "https://example.com/post?utmx=1",
			second: "https://example.com/post",
			same:   true,
		},
		{
			name:   "prefix family matomo kept detection",
			first:  "https://example.com/post?mtm_source=nl",
			second: "https://example.com/post",
			same:   true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			first := NormalizeForDedup(tc.first)
			second := NormalizeForDedup(tc.second)
			if tc.same {
				require.Equal(t, first, second, "expected equal keys")
			} else {
				require.NotEqual(t, first, second, "expected distinct keys")
			}
		})
	}
}

func TestNormalizeForDedupSpecificFilterCases(t *testing.T) {
	// Each of these must be considered the same URL as the URL without
	// the parameter (i.e. the parameter is a tracking param), under the
	// embedded ClearURLs global (catch-all) rules.
	//
	// Note: params only in the old hand-maintained list but absent from
	// ClearURLs rules (e.g. _hsmi, campaign_source) are NOT asserted here —
	// the engine now strips exactly what the rules strip, nothing more.
	for _, params := range []string{
		"?fbclid=abc",
		"?gclid=xyz",
		"?mc_cid=1&mc_eid=2",
		"?mkt_tok=token",
		"?twclid=abc",
		"?msclkid=def",
		"?yclid=123",
		"?_openstat=abc",
		"?_hsenc=1",
		"?srsltid=xyz",
		"?vero_id=abc",
		"?rb_clickid=x",
	} {
		with := "https://example.com/post" + params
		base := "https://example.com/post"
		require.Equal(t, NormalizeForDedup(with), NormalizeForDedup(base), "expected %q to be deduped to base", params)
	}
}

func TestNormalizeForDedupInvalidInputFallback(t *testing.T) {
	for _, raw := range []string{"", "not a url", "https://"} {
		var got string
		require.NotPanics(t, func() { got = NormalizeForDedup(raw) })
		// Fallback keeps the raw input without panic; it must not produce an
		// empty string for non-empty input.
		if raw != "" {
			require.NotEmpty(t, got)
		}
	}
}
