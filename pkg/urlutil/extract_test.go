package urlutil

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestExtractURLRefsMarkdownLinksWithTitles(t *testing.T) {
	refs := ExtractURLRefs(`- [demo](https://example.com/a "title") https://example.com/b`, ExtractOptions{
		BareURLs:    true,
		HTTPOnly:    true,
		Normalize:   true,
		Deduplicate: true,
	})

	require.Len(t, refs, 2)
	require.Equal(t, "https://example.com/a", refs[0].URL)
	require.Equal(t, "https://example.com/b", refs[1].URL)
}

func TestExtractURLRefsCleansBareURLPunctuation(t *testing.T) {
	refs := ExtractURLRefs(`See <https://example.com/a>. Also https://example.com/b),`, ExtractOptions{
		BareURLs:  true,
		HTTPOnly:  true,
		Normalize: true,
	})

	require.Len(t, refs, 2)
	require.Equal(t, "https://example.com/a", refs[0].URL)
	require.Equal(t, "https://example.com/b", refs[1].URL)
}

func TestExtractURLRefsDeduplicatesNormalizedURLs(t *testing.T) {
	refs := ExtractURLRefs(`https://example.com/a https://example.com/a/`, ExtractOptions{
		BareURLs:    true,
		HTTPOnly:    true,
		Normalize:   true,
		Deduplicate: true,
	})

	require.Len(t, refs, 1)
}

func TestExtractURLRefsResolvesRelativeHTMLAnchors(t *testing.T) {
	refs := ExtractURLRefs(`<p><a href="/transcript.vtt">Transcript</a></p>`, ExtractOptions{
		BaseURL:        "https://example.com/episode",
		HTMLAnchors:    true,
		HTTPOnly:       true,
		TranscriptOnly: true,
		Deduplicate:    true,
	})

	require.Len(t, refs, 1)
	require.Equal(t, "https://example.com/transcript.vtt", refs[0].URL)
}

func TestExtractURLRefsRelaxedBareDomainDoesNotResolveAgainstBase(t *testing.T) {
	refs := ExtractURLRefs(`example.com/transcript.txt`, ExtractOptions{
		BaseURL:        "https://pod.example/episode",
		BareURLs:       true,
		Relaxed:        true,
		HTTPOnly:       true,
		TranscriptOnly: true,
	})

	require.Len(t, refs, 1)
	require.Equal(t, "https://example.com/transcript.txt", refs[0].URL)
}

func TestExtractURLRefsTranscriptOnlyFilters(t *testing.T) {
	refs := ExtractURLRefs(`https://example.com/article.html https://example.com/transcript.html https://example.com/file.srt`, ExtractOptions{
		BareURLs:       true,
		HTTPOnly:       true,
		TranscriptOnly: true,
		Deduplicate:    true,
	})

	require.Len(t, refs, 2)
	require.Equal(t, "https://example.com/transcript.html", refs[0].URL)
	require.Equal(t, "https://example.com/file.srt", refs[1].URL)
}

func TestCleanHTTPURLRejectsMalformedMarkdownCapture(t *testing.T) {
	require.Empty(t, CleanHTTPURL("https://t.co/abc](https://x.com/user/status/1"))
}

func TestNormalizeSetCleansAndNormalizes(t *testing.T) {
	set := NormalizeSet([]string{"<https://example.com/a/>", "ftp://example.com/nope"})
	require.True(t, set["https://example.com/a"], "missing normalized URL")
	require.Len(t, set, 1)
}

func TestExtractURLRefs_EmptyText(t *testing.T) {
	refs := ExtractURLRefs("", ExtractOptions{BareURLs: true, HTTPOnly: true})
	require.Empty(t, refs)
}

func TestExtractURLRefs_MarkdownNoMatch(t *testing.T) {
	// A markdown-like text that doesn't match the URL pattern
	refs := ExtractURLRefs(`[text](not-a-url)`, ExtractOptions{BareURLs: true})
	require.Empty(t, refs)
}

func TestExtractURLRefs_HTMLAnchorsError(t *testing.T) {
	// goquery can handle most HTML; test with valid HTML but no href
	refs := ExtractURLRefs(`<p>no links</p>`, ExtractOptions{HTMLAnchors: true})
	require.Empty(t, refs)
}

func TestExtractURLRefs_HTMLAnchorNoHref(t *testing.T) {
	refs := ExtractURLRefs(`<a>no href</a>`, ExtractOptions{HTMLAnchors: true})
	require.Empty(t, refs)
}

func TestExtractURLRefs_HTMLAnchorFilteredByKeep(t *testing.T) {
	// non-http href with HTTPOnly
	refs := ExtractURLRefs(`<a href="ftp://example.com/file">link</a>`, ExtractOptions{
		HTMLAnchors: true,
		HTTPOnly:    true,
	})
	require.Empty(t, refs)
}

func TestExtractURLRefs_BareURLNoMatch(t *testing.T) {
	// No URLs in text
	refs := ExtractURLRefs(`just plain text`, ExtractOptions{BareURLs: true, HTTPOnly: true})
	require.Empty(t, refs)
}

func TestExtractURLRefs_BareURLRelaxed(t *testing.T) {
	refs := ExtractURLRefs(`check example.com/path`, ExtractOptions{
		BareURLs: true,
		Relaxed:  true,
		HTTPOnly: true,
	})
	require.Len(t, refs, 1)
	require.Equal(t, "https://example.com/path", refs[0].URL)
}

func TestKeepExtractedURL_Empty(t *testing.T) {
	require.False(t, keepExtractedURL("", ExtractOptions{}))
}

func TestKeepExtractedURL_TranscriptOnly_Match(t *testing.T) {
	require.True(t, keepExtractedURL("https://example.com/transcript.json", ExtractOptions{
		TranscriptOnly: true,
	}))
}

func TestKeepExtractedURL_TranscriptOnly_NoMatch(t *testing.T) {
	require.False(t, keepExtractedURL("https://example.com/page.html", ExtractOptions{
		TranscriptOnly: true,
	}))
}

func TestCleanURLWithTrim_AllTrimmed(t *testing.T) {
	// All characters are trim chars - result should be empty
	result := CleanURLWithTrim("<<<>>>", CleanOptions{})
	require.Empty(t, result.URL)
}

func TestCleanURLWithTrim_BracketsAndParens(t *testing.T) {
	result := CleanURLWithTrim(`<https://example.com/path>.`, CleanOptions{HTTPOnly: true})
	require.Equal(t, "https://example.com/path", result.URL)
}

func TestCleanHTTPURL_Basic(t *testing.T) {
	require.Equal(t, "https://example.com", CleanHTTPURL("https://example.com"))
}

func TestCleanHTTPURL_FTPRejected(t *testing.T) {
	require.Empty(t, CleanHTTPURL("ftp://example.com"))
}

func TestCleanURL_WithBaseURL(t *testing.T) {
	result := CleanURL("/relative/path", CleanOptions{BaseURL: "https://example.com"})
	require.Equal(t, "https://example.com/relative/path", result)
}

func TestCleanURL_InvalidURL(t *testing.T) {
	result := CleanURL("%zz", CleanOptions{})
	require.Empty(t, result)
}

func TestCleanURL_WithMarkdownSyntax(t *testing.T) {
	// URL containing ]( should be rejected
	result := CleanURL("https://t.co/abc](https://x.com/user/status/1", CleanOptions{HTTPOnly: true})
	require.Empty(t, result)
}

func TestParseURLCandidate_Empty(t *testing.T) {
	_, ok := parseURLCandidate("", false)
	require.False(t, ok)
}

func TestParseURLCandidate_WithMarkdownSyntax(t *testing.T) {
	_, ok := parseURLCandidate("abc](def", false)
	require.False(t, ok)
}

func TestParseURLCandidate_AssumeHTTPS_LooksLikeDomain(t *testing.T) {
	parsed, ok := parseURLCandidate("example.com/path", true)
	require.True(t, ok)
	require.Equal(t, "https", parsed.Scheme)
	require.Equal(t, "example.com", parsed.Host)
}

func TestParseURLCandidate_AssumeHTTPS_RelativePath(t *testing.T) {
	// "/path/page" doesn't look like a domain URL
	_, ok := parseURLCandidate("/path/page", true)
	require.True(t, ok) // still parses, just without scheme
}

func TestResolveURLReference_InvalidBaseURL(t *testing.T) {
	_, ok := resolveURLReference(&url.URL{Path: "/relative"}, "%zz")
	require.False(t, ok)
}

func TestResolveURLReference_AbsoluteURL(t *testing.T) {
	parsed := &url.URL{Scheme: "https", Host: "example.com", Path: "/page"}
	result, ok := resolveURLReference(parsed, "https://base.com")
	require.True(t, ok)
	require.Equal(t, "https://example.com/page", result.String())
}

func TestLooksLikeDomainURL(t *testing.T) {
	require.True(t, looksLikeDomainURL("example.com/path"))
	require.True(t, looksLikeDomainURL("sub.example.com"))
	require.False(t, looksLikeDomainURL("/relative/path"))
	require.False(t, looksLikeDomainURL("./relative"))
	require.False(t, looksLikeDomainURL("../parent"))
	require.False(t, looksLikeDomainURL(""))
}

func TestIsTranscriptURL(t *testing.T) {
	require.True(t, IsTranscriptURL("https://example.com/transcript"))
	require.True(t, IsTranscriptURL("https://example.com/file.json"))
	require.True(t, IsTranscriptURL("https://example.com/file.srt"))
	require.True(t, IsTranscriptURL("https://example.com/file.txt"))
	require.True(t, IsTranscriptURL("https://example.com/file.vtt"))
	require.False(t, IsTranscriptURL("https://example.com/file.html"))
	require.False(t, IsTranscriptURL("https://example.com/file.mp4"))
}

func TestURLExtension_ParseError(t *testing.T) {
	// When url.Parse fails, fall back to path.Ext on the raw string
	ext := urlExtension("%zz.json")
	require.Equal(t, ".json", ext)
}

func TestReplaceRangeWithSpaces_InvalidRange(t *testing.T) {
	// start < 0
	result := replaceRangeWithSpaces("hello", -1, 3)
	require.Equal(t, "hello", result)

	// end < start
	result = replaceRangeWithSpaces("hello", 3, 1)
	require.Equal(t, "hello", result)

	// start > len
	result = replaceRangeWithSpaces("hello", 10, 15)
	require.Equal(t, "hello", result)

	// end > len
	result = replaceRangeWithSpaces("hello", 0, 10)
	require.Equal(t, "hello", result)
}

func TestReplaceRangeWithSpaces_Valid(t *testing.T) {
	result := replaceRangeWithSpaces("hello world", 5, 6)
	require.Equal(t, "hello world", result)
}

func TestMaskRanges_WithRefs(t *testing.T) {
	refs := []URLRef{
		{URL: "https://a.com", Start: 6, End: 20},
	}
	masked := maskRanges("visit https://a.com today", refs)
	require.NotContains(t, masked, "https://a.com")
}

func TestMaskRanges_OutOfOrder(t *testing.T) {
	refs := []URLRef{
		{URL: "c", Start: 20, End: 22},
		{URL: "a", Start: 0, End: 2},
		{URL: "b", Start: 10, End: 12},
	}
	masked := maskRanges("xx yy zz ww qq rr", refs)
	require.Len(t, masked, len("xx yy zz ww qq rr"))
}

func TestMaskRanges_EqualStart(t *testing.T) {
	// Two refs with the same Start to exercise the equal comparison path
	refs := []URLRef{
		{URL: "a", Start: 5, End: 8},
		{URL: "b", Start: 5, End: 10},
	}
	masked := maskRanges("hello world", refs)
	require.Len(t, masked, len("hello world"))
}

func TestExtractURLRefs_MarkdownTranscriptOnlyFiltered(t *testing.T) {
	// Markdown link to a non-transcript URL with TranscriptOnly
	refs := ExtractURLRefs(`[link](https://example.com/page.html)`, ExtractOptions{
		BareURLs:       true,
		TranscriptOnly: true,
		HTTPOnly:       true,
	})
	require.Empty(t, refs)
}

func TestExtractURLRefs_MarkdownTranscriptMatch(t *testing.T) {
	refs := ExtractURLRefs(`[transcript](https://example.com/transcript.json)`, ExtractOptions{
		BareURLs:       true,
		TranscriptOnly: true,
		HTTPOnly:       true,
	})
	require.Len(t, refs, 1)
	require.Contains(t, refs[0].URL, "transcript")
}

func TestCleanURL_ResolveReferenceError(t *testing.T) {
	// A relative URL with an invalid base URL should return ""
	result := CleanURL("/relative/path", CleanOptions{BaseURL: "%zz"})
	require.Empty(t, result)
}

func TestCleanURL_AssumeHTTPS_RelativePath(t *testing.T) {
	// A relative path with AssumeHTTPS: parseURLCandidate returns a relative URL,
	// resolveURLReference returns it as-is (no baseURL), then scheme is set to "https".
	// But validCleanURL rejects it because Host is empty.
	result := CleanURL("/path/page", CleanOptions{AssumeHTTPS: true})
	// The URL will have scheme "https" but no host, so validCleanURL returns false
	require.Empty(t, result)
}

func TestCleanURL_WithMarkdownBracketSyntax(t *testing.T) {
	// URL containing ]( should be rejected by parseURLCandidate
	result := CleanURL("abc](def", CleanOptions{})
	require.Empty(t, result)
}

func TestParseURLCandidate_AssumeHTTPS_DomainWithPercent(t *testing.T) {
	// looksLikeDomainURL("example.com%") is true (contains "."),
	// but url.Parse("https://example.com%") should fail
	_, ok := parseURLCandidate("example.com%", true)
	require.False(t, ok)
}

func TestExtractBareURLRefs_KeepFiltered(t *testing.T) {
	// Test that bare URLs are filtered by keepExtractedURL
	refs := ExtractURLRefs(`ftp://files.example.com/data`, ExtractOptions{
		BareURLs: true,
		HTTPOnly: true,
	})
	require.Empty(t, refs)
}

func TestIsTranscriptURL_Extensions(t *testing.T) {
	require.True(t, IsTranscriptURL("https://example.com/subtitles.vtt"))
	require.True(t, IsTranscriptURL("https://example.com/subtitles.srt"))
	require.True(t, IsTranscriptURL("https://example.com/data.json"))
	require.True(t, IsTranscriptURL("https://example.com/notes.txt"))
	require.True(t, IsTranscriptURL("https://example.com/TRANSCRIPT.html"))
	require.False(t, IsTranscriptURL("https://example.com/video.mp4"))
}

func TestURLExtension_ValidURL(t *testing.T) {
	require.Equal(t, ".json", urlExtension("https://example.com/data.json"))
	require.Equal(t, ".vtt", urlExtension("https://example.com/subs.vtt"))
	require.Empty(t, urlExtension("https://example.com/noext"))
}
