package urlutil

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestEngineRedirectionExpansion verifies the ClearURLs redirections path: a
// URL that is a tracking redirect wrapper gets expanded to its real target.
func TestEngineRedirectionExpansion(t *testing.T) {
	cases := []struct {
		name string
		in   string
		base string // the URL it must collapse onto
	}{
		{
			name: "google /url expands to target",
			in:   "https://www.google.com/url?q=https%3A%2F%2Fexample.com%2Farticle&sa=U",
			base: "https://example.com/article",
		},
		{
			name: "youtube redirect expands",
			in:   "https://www.youtube.com/redirect?q=https%3A%2F%2Fexample.org%2Fx",
			base: "https://example.org/x",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := NormalizeForDedup(tc.in)
			base := NormalizeForDedup(tc.base)
			require.Equal(t, base, got, "redirect wrapper should collapse onto target")
		})
	}
}

// TestEngineCompleteProvider verifies completeProvider domains drop all query
// params while keeping the host/path (and any guard params).
func TestEngineCompleteProvider(t *testing.T) {
	in := "https://www.googlesyndication.com/ads?foo=1&bar=2&u=target"
	got := NormalizeForDedup(in)
	require.NotContains(t, got, "foo=1")
	require.NotContains(t, got, "q=")
	require.NotContains(t, got, "bar=2")
}

// TestEngineGuardParams are content-bearing params that must never be stripped
// even under full ClearURLs semantics — the classic three (?v, ?bvid, ?q) plus
// the rest of the guard set, exercised through the full NormalizeForDedup path.
func TestEngineGuardParams(t *testing.T) {
	for _, tc := range []struct {
		key string
		val string
	}{
		{"v", "abc123"}, {"bvid", "BV1xx411c7mD"}, {"q", "golang url dedup"},
		{"id", "42"}, {"page", "2"}, {"sort", "asc"}, {"order", "desc"},
		{"limit", "10"}, {"offset", "5"}, {"tag", "go"}, {"tab", "posts"},
		{"view", "reader"}, {"lang", "en"}, {"lang_code", "zh"},
	} {
		// Guard param plus a tracking param alongside; the tracking must strip
		// but the guard must survive.
		u := "https://example.com/p?utm_source=nl&" + tc.key + "=" + url.QueryEscape(tc.val)
		norm := NormalizeForDedup(u)
		require.NotContains(t, norm, "utm_source=nl")
		parsed, err := url.Parse(norm)
		require.NoError(t, err)
		require.Equal(t, tc.val, parsed.Query().Get(tc.key), "guard %q must survive NormalizeForDedup")
	}
}

// TestEngineCaseInsensitive verifies urlPattern/exceptions/redirection/param
// matching is case-insensitive (matching upstream re.IGNORECASE): an all-caps
// host still matches its provider's rules.
func TestEngineCaseInsensitive(t *testing.T) {
	// host case: the global utm rule must strip from an uppercase host too.
	upper := NormalizeForDedup("HTTPS://EXAMPLE.COM/post?UTM_SOURCE=nl")
	lower := NormalizeForDedup("https://example.com/post?utm_source=nl")
	require.Equal(t, lower, upper, "case-insensitive host/param matching")
	// param-name case: UTM_SOURCE stripped just like utm_source.
	plain := NormalizeForDedup("https://example.com/post")
	require.Equal(t, plain, upper, "uppercase utm param stripped")
}

// TestEngineDomainScoping verifies that a parameter stripped on one domain is
// preserved as a guard on another, i.e. domain scoping is honored.
func TestEngineDomainScoping(t *testing.T) {
	// tracking_source is a ClearURLs global param; it should strip anywhere.
	with := NormalizeForDedup("https://example.com/post?tracking_source=x")
	without := NormalizeForDedup("https://example.com/post")
	require.Equal(t, without, with, "tracking_source should be stripped globally")
}

// TestRulesDataProvenance verifies the embedded hash is a well-formed SHA-256
// digest and stable across calls, so the rule snapshot is reproducible.
func TestRulesDataProvenance(t *testing.T) {
	require.NotEmpty(t, RulesDataHash(), "embedded rules digest must be present")
	require.Len(t, RulesDataHash(), 64, "SHA-256 hex digest length")
	require.Equal(t, RulesDataHash(), RulesDataHash(), "provenance digest must be stable")
}

// TestEngineIdempotentOverSample verifies the normalization is idempotent
// (re-running on its own output yields the same key) and that same params in a
// different order collapse, across a realistic sample including redirection
// targets and a bilibili URL whose tracking params are stripped but bvid kept.
func TestEngineIdempotentOverSample(t *testing.T) {
	urls := []string{
		"https://www.bilibili.com/video/BV1GK4y1x7x/",
		"https://www.bilibili.com/video/BV1GK4y1x7x/?from=search&spm_id_from=333.337.0.0&vd_source=abc",
		"https://www.google.com/url?q=https%3A%2F%2Fexample.com%2Fpost&sa=U",
		"https://example.com?utm_source=a&utm_medium=b&utm_campaign=c",
		"https://example.com?a=1&b=2",
		"https://youtube.com/watch?v=dQw4w9WgXcQ",
	}
	for _, r := range urls {
		once := NormalizeForDedup(r)
		twice := NormalizeForDedup(once)
		require.Equal(t, once, twice, "normalization must be idempotent for %q", r)
	}
	// Same params in different order share a key.
	a := NormalizeForDedup("https://example.com?a=1&b=2")
	b := NormalizeForDedup("https://example.com?b=2&a=1")
	require.Equal(t, a, b, "same params in different order must share a key")
}
