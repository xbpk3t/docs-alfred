package urlutil

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestExtractURLRefsMarkdownLinksWithTitles(t *testing.T) {
	refs := ExtractURLRefs(`- [demo](https://example.com/a "title") https://example.com/b`, ExtractOptions{
		Markdown:    true,
		BareURLs:    true,
		HTTPOnly:    true,
		Normalize:   true,
		Deduplicate: true,
	})

	require.Len(t, refs, 2)
	require.Equal(t, "https://example.com/a", refs[0].URL)
	require.Equal(t, 2, refs[0].Start)
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
	require.Equal(t, "", CleanHTTPURL("https://t.co/abc](https://x.com/user/status/1"))
}

func TestNormalizeSetCleansAndNormalizes(t *testing.T) {
	set := NormalizeSet([]string{"<https://example.com/a/>", "ftp://example.com/nope"})
	require.True(t, set["https://example.com/a"], "missing normalized URL")
	require.Len(t, set, 1)
}
