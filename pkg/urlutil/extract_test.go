package urlutil

import "testing"

func TestExtractURLRefsMarkdownLinksWithTitles(t *testing.T) {
	refs := ExtractURLRefs(`- [demo](https://example.com/a "title") https://example.com/b`, ExtractOptions{
		Markdown:    true,
		BareURLs:    true,
		HTTPOnly:    true,
		Normalize:   true,
		Deduplicate: true,
	})

	if len(refs) != 2 {
		t.Fatalf("len(refs) = %d, want 2: %#v", len(refs), refs)
	}
	if refs[0].URL != "https://example.com/a" || refs[0].Start != 2 {
		t.Fatalf("markdown ref = %#v", refs[0])
	}
	if refs[1].URL != "https://example.com/b" {
		t.Fatalf("bare ref = %#v", refs[1])
	}
}

func TestExtractURLRefsCleansBareURLPunctuation(t *testing.T) {
	refs := ExtractURLRefs(`See <https://example.com/a>. Also https://example.com/b),`, ExtractOptions{
		BareURLs:  true,
		HTTPOnly:  true,
		Normalize: true,
	})

	if len(refs) != 2 {
		t.Fatalf("len(refs) = %d, want 2: %#v", len(refs), refs)
	}
	if refs[0].URL != "https://example.com/a" {
		t.Fatalf("first URL = %q", refs[0].URL)
	}
	if refs[1].URL != "https://example.com/b" {
		t.Fatalf("second URL = %q", refs[1].URL)
	}
}

func TestExtractURLRefsDeduplicatesNormalizedURLs(t *testing.T) {
	refs := ExtractURLRefs(`https://example.com/a https://example.com/a/`, ExtractOptions{
		BareURLs:    true,
		HTTPOnly:    true,
		Normalize:   true,
		Deduplicate: true,
	})

	if len(refs) != 1 {
		t.Fatalf("len(refs) = %d, want 1: %#v", len(refs), refs)
	}
}

func TestExtractURLRefsResolvesRelativeHTMLAnchors(t *testing.T) {
	refs := ExtractURLRefs(`<p><a href="/transcript.vtt">Transcript</a></p>`, ExtractOptions{
		BaseURL:        "https://example.com/episode",
		HTMLAnchors:    true,
		HTTPOnly:       true,
		TranscriptOnly: true,
		Deduplicate:    true,
	})

	if len(refs) != 1 {
		t.Fatalf("len(refs) = %d, want 1: %#v", len(refs), refs)
	}
	if refs[0].URL != "https://example.com/transcript.vtt" {
		t.Fatalf("URL = %q", refs[0].URL)
	}
}

func TestExtractURLRefsRelaxedBareDomainDoesNotResolveAgainstBase(t *testing.T) {
	refs := ExtractURLRefs(`example.com/transcript.txt`, ExtractOptions{
		BaseURL:        "https://pod.example/episode",
		BareURLs:       true,
		Relaxed:        true,
		HTTPOnly:       true,
		TranscriptOnly: true,
	})

	if len(refs) != 1 {
		t.Fatalf("len(refs) = %d, want 1: %#v", len(refs), refs)
	}
	if refs[0].URL != "https://example.com/transcript.txt" {
		t.Fatalf("URL = %q", refs[0].URL)
	}
}

func TestExtractURLRefsTranscriptOnlyFilters(t *testing.T) {
	refs := ExtractURLRefs(`https://example.com/article.html https://example.com/transcript.html https://example.com/file.srt`, ExtractOptions{
		BareURLs:       true,
		HTTPOnly:       true,
		TranscriptOnly: true,
		Deduplicate:    true,
	})

	if len(refs) != 2 {
		t.Fatalf("len(refs) = %d, want 2: %#v", len(refs), refs)
	}
	if refs[0].URL != "https://example.com/transcript.html" || refs[1].URL != "https://example.com/file.srt" {
		t.Fatalf("refs = %#v", refs)
	}
}

func TestCleanHTTPURLRejectsMalformedMarkdownCapture(t *testing.T) {
	if got := CleanHTTPURL("https://t.co/abc](https://x.com/user/status/1"); got != "" {
		t.Fatalf("CleanHTTPURL returned %q, want empty", got)
	}
}

func TestNormalizeSetCleansAndNormalizes(t *testing.T) {
	set := NormalizeSet([]string{"<https://example.com/a/>", "ftp://example.com/nope"})
	if !set["https://example.com/a"] {
		t.Fatalf("missing normalized URL in %#v", set)
	}
	if len(set) != 1 {
		t.Fatalf("len(set) = %d, want 1: %#v", len(set), set)
	}
}
