package wiki

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xbpk3t/docs-alfred/internal/gh/index"
)

// --- DetectContentType edge cases ---

func TestDetectContentTypePodcast(t *testing.T) {
	assert.Equal(t, ContentAudio, DetectContentType("https://xiaoyuzhoufm.com/ep/1"))
	assert.Equal(t, ContentAudio, DetectContentType("https://example.com/podcast/feed"))
	assert.Equal(t, ContentAudio, DetectContentType("https://example.libsyn.com/episode"))
}

func TestDetectContentTypeB23(t *testing.T) {
	assert.Equal(t, ContentVideo, DetectContentType("https://b23.tv/abc"))
}

func TestDetectContentTypeMobileYouTube(t *testing.T) {
	assert.Equal(t, ContentVideo, DetectContentType("https://m.youtube.com/watch?v=abc"))
	assert.Equal(t, ContentVideo, DetectContentType("https://music.youtube.com/watch?v=abc"))
}

func TestDetectContentTypeMobileBilibili(t *testing.T) {
	assert.Equal(t, ContentVideo, DetectContentType("https://m.bilibili.com/video/BV1xx"))
}

// --- URL classification helpers ---

func TestIsVideoURL(t *testing.T) {
	tests := []struct {
		url  string
		want bool
	}{
		{"https://www.youtube.com/watch?v=abc", true},
		{"https://youtube.com/shorts/abc", true},
		{"https://www.youtube.com/embed/abc", true},
		{"https://youtu.be/abc", true},
		{"https://www.bilibili.com/video/BV1xx", true},
		{"https://b23.tv/abc", true},
		{"https://www.youtube.com/@user/videos", false},
		{"https://example.com/page", false},
		{"", false},
	}
	for _, tt := range tests {
		t.Run(tt.url, func(t *testing.T) {
			assert.Equal(t, tt.want, isVideoURL(tt.url))
		})
	}
}

func TestHasKnownVideoURLText(t *testing.T) {
	assert.True(t, hasKnownVideoURLText("https://youtu.be/abc"))
	assert.True(t, hasKnownVideoURLText("https://b23.tv/abc"))
	assert.True(t, hasKnownVideoURLText("https://bilibili.com/video/BV1"))
	assert.False(t, hasKnownVideoURLText("https://example.com"))
}

func TestIsPodcastLikeURL(t *testing.T) {
	assert.True(t, isPodcastLikeURL("https://xiaoyuzhoufm.com/ep/1"))
	assert.True(t, isPodcastLikeURL("https://example.com/podcast/feed"))
	assert.True(t, isPodcastLikeURL("https://example.libsyn.com/ep"))
	assert.True(t, isPodcastLikeURL("https://anchor.fm/show"))
	assert.False(t, isPodcastLikeURL("https://example.com/article"))
}

func TestIsRSSFeedLike(t *testing.T) {
	assert.True(t, isRSSFeedLike("https://example.com/feed.xml"))
	assert.True(t, isRSSFeedLike("https://example.com/feed.rss"))
	assert.True(t, isRSSFeedLike("https://example.com/feed.atom"))
	assert.True(t, isRSSFeedLike("https://example.com/feed"))
	assert.True(t, isRSSFeedLike("https://example.com/feed/items"))
	assert.True(t, isRSSFeedLike("https://example.com/rss"))
	assert.False(t, isRSSFeedLike("https://example.com/article"))
}

func TestIsDirectAudioURL(t *testing.T) {
	assert.True(t, isDirectAudioURL("https://example.com/audio.mp3"))
	assert.True(t, isDirectAudioURL("https://example.com/audio.m4a"))
	assert.True(t, isDirectAudioURL("https://example.com/audio.aac"))
	assert.True(t, isDirectAudioURL("https://example.com/audio.wav"))
	assert.True(t, isDirectAudioURL("https://example.com/audio.flac"))
	assert.True(t, isDirectAudioURL("https://example.com/audio.ogg"))
	assert.True(t, isDirectAudioURL("https://example.com/audio.opus"))
	assert.False(t, isDirectAudioURL("https://example.com/audio.txt"))
}

// --- extractTitleFromHTML ---

func TestExtractTitleFromHTML(t *testing.T) {
	assert.Equal(t, "Hello", extractTitleFromHTML(`<html><head><title>Hello</title></head><body></body></html>`))
	assert.Empty(t, extractTitleFromHTML(`<html><body>No title</body></html>`))
	assert.Empty(t, extractTitleFromHTML(`not html`))
}

// --- isHTTPBlockError ---

func TestIsHTTPBlockError(t *testing.T) {
	assert.True(t, isHTTPBlockError(errors.New("HTTP 403 forbidden")))
	assert.False(t, isHTTPBlockError(errors.New("connection timeout")))
}

// --- extractWeixinSavedPath ---

func TestExtractWeixinSavedPath(t *testing.T) {
	tests := []struct {
		name   string
		output string
		want   string
	}{
		{
			name:   "found",
			output: "title: test\nsaved: /tmp/weixin/file.md\nother: val",
			want:   "/tmp/weixin/file.md",
		},
		{
			name:   "not found",
			output: "title: test\nother: val",
			want:   "",
		},
		{
			name:   "empty output",
			output: "",
			want:   "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, extractWeixinSavedPath(tt.output))
		})
	}
}

func TestExtractSavedPathFromLine(t *testing.T) {
	assert.Equal(t, "/tmp/file.md", extractSavedPathFromLine("  saved: /tmp/file.md"))
	assert.Empty(t, extractSavedPathFromLine("no saved field"))
	assert.Empty(t, extractSavedPathFromLine("saved:"))
}

// --- cleanExtractReason / isMissingTranscriptReason ---

func TestCleanExtractReason(t *testing.T) {
	assert.Equal(t, "low quality", cleanExtractReason("extract: low quality"))
	assert.Equal(t, "low quality", cleanExtractReason("  extract: low quality  "))
	assert.Equal(t, "fetch error", cleanExtractReason("fetch error"))
}

func TestIsMissingTranscriptReason(t *testing.T) {
	assert.True(t, isMissingTranscriptReason("RSS item has no podcast:transcript tag"))
	assert.True(t, isMissingTranscriptReason("description/content has no transcript link"))
	assert.True(t, isMissingTranscriptReason("no description or content to search"))
	assert.True(t, isMissingTranscriptReason("all providers failed to produce transcript"))
	assert.False(t, isMissingTranscriptReason("network timeout"))
}

// --- extractFailure ---

func TestExtractFailure(t *testing.T) {
	r := extractFailure("https://example.com", "low quality")
	assert.Equal(t, "extract: low quality", r.Error)
	assert.Equal(t, FailureExtract, r.FailureKind)
	assert.Equal(t, "https://example.com", r.SourceURL)
}

func TestExtractFailureEmptyReason(t *testing.T) {
	r := extractFailure("https://example.com", "")
	assert.Contains(t, r.Error, "content extraction failed")
}

func TestExtractFailureAlreadyPrefixed(t *testing.T) {
	r := extractFailure("https://example.com", "extract: already prefixed")
	assert.Equal(t, "extract: already prefixed", r.Error)
}

// --- cloudflareChallengeReason ---

func TestCloudflareChallengeReason(t *testing.T) {
	assert.NotEmpty(t, cloudflareChallengeReason("just a moment checking your browser"))
	assert.NotEmpty(t, cloudflareChallengeReason("just a moment cf-browser-verification"))
	assert.NotEmpty(t, cloudflareChallengeReason("just a moment cloudflare ray id:"))
	assert.Empty(t, cloudflareChallengeReason("normal page content"))
}

// --- socialShellLike / hasRealSentences ---

func TestSocialShellLikeShort(t *testing.T) {
	assert.True(t, socialShellLike("short content", "short"))
}

func TestSocialShellLikePatterns(t *testing.T) {
	assert.True(t, socialShellLike("please log in to continue", strings.Repeat("x", 500)))
}

func TestSocialShellLikeFalse(t *testing.T) {
	long := strings.Repeat("This is a sentence with real content. ", 100)
	assert.False(t, socialShellLike(long, long))
}

func TestHasRealSentences(t *testing.T) {
	assert.True(t, hasRealSentences("one. two. three. four."))
	assert.False(t, hasRealSentences("no sentences here"))
}

// --- isLinkHeavy ---

func TestIsLinkHeavy(t *testing.T) {
	// Heavy links: 25 links, each 20 chars, 500 text -> 25*80=2000 > 500
	heavy := strings.Repeat("[a](https://x.com) ", 25)
	assert.True(t, isLinkHeavy(heavy))

	assert.False(t, isLinkHeavy("just some text with no links"))
	assert.False(t, isLinkHeavy(strings.Repeat("[a](https://x.com) ", 5)))
}

func TestIsLinkHeavyEmptyBody(t *testing.T) {
	// Empty body has 0 links, which is < 20, so not link-heavy
	assert.False(t, isLinkHeavy(""))
}

// --- isSocialShellDomain ---

func TestIsSocialShellDomain(t *testing.T) {
	assert.True(t, isSocialShellDomain("x.com"))
	assert.True(t, isSocialShellDomain("twitter.com"))
	assert.True(t, isSocialShellDomain("instagram.com"))
	assert.False(t, isSocialShellDomain("example.com"))
}

// --- NewDriver ---

func TestNewDriverOpencli(t *testing.T) {
	d, err := NewDriver("opencli", DriverOptions{MaxBodySize: 1000})
	require.NoError(t, err)
	assert.Equal(t, "opencli", d.Name())
}

func TestNewDriverHTTP(t *testing.T) {
	d, err := NewDriver("http-readability", DriverOptions{MaxBodySize: 1000})
	require.NoError(t, err)
	assert.Equal(t, "http-readability", d.Name())
}

func TestNewDriverUnknown(t *testing.T) {
	_, err := NewDriver("unknown", DriverOptions{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown driver")
}

// --- metadataToMap ---

func TestMetadataToMapNil(t *testing.T) {
	assert.Nil(t, metadataToMap(nil))
}

func TestMetadataToMapAllFields(t *testing.T) {
	m := metadataToMap(&EntryMetadata{
		ContentType:       "text",
		Quality:           "high",
		Author:            "John",
		Uncertainties:     "none",
		Duration:          "10min",
		TranscriptQuality: "good",
		Verdict:           "watch",
		Language:          "en",
		Tags:              []string{"go", "cli", "tool"},
		Stars:             4,
	})
	assert.Equal(t, "text", m["Type"])
	assert.Equal(t, "high", m["quality"])
	assert.Equal(t, "John", m["author"])
	assert.Equal(t, "go, cli, tool", m["tags"])
	assert.Equal(t, "4", m["stars"])
}

func TestMetadataToMapMinimal(t *testing.T) {
	m := metadataToMap(&EntryMetadata{ContentType: "media"})
	assert.Equal(t, "media", m["Type"])
	assert.Len(t, m, 1)
}

// --- buildMetaBlock ---

func TestBuildMetaBlockNilMetadata(t *testing.T) {
	assert.Empty(t, buildMetaBlock(&aiClassification{}))
}

func TestBuildMetaBlockWithMetadata(t *testing.T) {
	block := buildMetaBlock(&aiClassification{
		Metadata: &EntryMetadata{ContentType: "text", Author: "Alice"},
	})
	assert.Contains(t, block, "Type: text")
	assert.Contains(t, block, "author: Alice")
}

// --- RenderStructuredSummary ---

func TestRenderStructuredSummaryNil(t *testing.T) {
	assert.Empty(t, RenderStructuredSummary(nil))
}

func TestRenderStructuredSummaryFull(t *testing.T) {
	s := &StructuredSummary{
		Overview:    "This is an overview",
		WorthNoting: "Something noteworthy",
		KeyPoints:   []string{"Point 1", "Point 2"},
	}
	rendered := RenderStructuredSummary(s)
	assert.Contains(t, rendered, "overview")
	assert.Contains(t, rendered, "This is an overview")
	assert.Contains(t, rendered, "keyPoints")
	assert.Contains(t, rendered, "- Point 1")
	assert.Contains(t, rendered, "- Point 2")
}

func TestRenderStructuredSummaryEmpty(t *testing.T) {
	s := &StructuredSummary{}
	assert.Empty(t, RenderStructuredSummary(s))
}

// --- ValidateRelativeWikiPath edge cases ---

func TestValidateRelativeWikiPathInputErrors(t *testing.T) {
	tests := []struct {
		name    string
		root    string
		path    string
		wantErr string
	}{
		{"empty root", "", "topic/path", "wiki root is empty"},
		{"empty path", "/wiki", "", "relative path is empty"},
		{"absolute path", "/wiki", "/etc/passwd", "absolute path not allowed"},
		{"path traversal", "/wiki", "../etc/passwd", "invalid segment"},
		{"dot segment", "/wiki", "topic/./path", "invalid segment"},
		{"double dot segment", "/wiki", "topic/../path", "invalid segment"},
		{"null byte", "/wiki", "topic/\x00/path", "invalid characters"},
		{"backslash", "/wiki", `topic\path`, "invalid characters"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateRelativeWikiPath(tt.root, tt.path)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestValidateRelativeWikiPathValid(t *testing.T) {
	root := t.TempDir()
	assert.NoError(t, ValidateRelativeWikiPath(root, "topic/path"))
}

// --- digestFilename ---

func TestDigestFilename(t *testing.T) {
	tests := []struct {
		entry *DigestEntry
		want  string
	}{
		{&DigestEntry{Status: DigestSuccess}, "digest-success.jsonl"},
		{&DigestEntry{Status: DigestFailure, FailureKind: string(FailureFetch)}, "digest-fetch-error.jsonl"},
		{&DigestEntry{Status: DigestFailure, FailureKind: string(FailureResolve)}, "digest-fetch-error.jsonl"},
		{&DigestEntry{Status: DigestFailure, FailureKind: string(FailureExtract)}, "digest-extract-error.jsonl"},
		{&DigestEntry{Status: DigestFailure, FailureKind: string(FailureClassify)}, "digest-classify-rejected.jsonl"},
		{&DigestEntry{Status: DigestFailure, FailureKind: string(FailureAI)}, "digest-ai-error.jsonl"},
		{&DigestEntry{Status: DigestFailure, FailureKind: "unknown"}, "digest-ai-error.jsonl"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			assert.Equal(t, tt.want, digestFilename(tt.entry))
		})
	}
}

// --- digestStageForFailure ---

func TestDigestStageForFailure(t *testing.T) {
	assert.Equal(t, StageFetch, digestStageForFailure(FailureFetch))
	assert.Equal(t, StageFetch, digestStageForFailure(FailureResolve))
	assert.Equal(t, StageExtract, digestStageForFailure(FailureExtract))
	assert.Equal(t, StageClassify, digestStageForFailure(FailureClassify))
	assert.Equal(t, StageClassify, digestStageForFailure(FailureAI))
}

// --- LogDigestEntry ---

func TestLogDigestEntryNilOpts(t *testing.T) {
	path, err := LogDigestEntry(&DigestEntry{}, nil)
	assert.NoError(t, err)
	assert.Empty(t, path)
}

func TestLogDigestEntryEmptyWikiRoot(t *testing.T) {
	path, err := LogDigestEntry(&DigestEntry{}, &WriteOptions{})
	assert.NoError(t, err)
	assert.Empty(t, path)
}

func TestLogDigestEntryDryRun(t *testing.T) {
	path, err := LogDigestEntry(&DigestEntry{
		URL:    "https://example.com",
		Status: DigestSuccess,
	}, &WriteOptions{WikiRoot: t.TempDir(), DryRun: true})
	assert.NoError(t, err)
	assert.NotEmpty(t, path)
}

func TestLogDigestEntrySuccess(t *testing.T) {
	root := t.TempDir()
	entry := &DigestEntry{
		URL:    "https://example.com",
		Status: DigestSuccess,
	}
	path, err := LogDigestEntry(entry, &WriteOptions{WikiRoot: root})
	require.NoError(t, err)
	assert.NotEmpty(t, path)

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Contains(t, string(data), "https://example.com")
	assert.Contains(t, string(data), "success")
}

// --- WriteManualReviewEntry ---

func TestWriteManualReviewEntryNilOpts(t *testing.T) {
	path, err := WriteManualReviewEntry(&ClassifyItem{}, nil)
	assert.NoError(t, err)
	assert.Empty(t, path)
}

func TestWriteManualReviewEntryNilItem(t *testing.T) {
	path, err := WriteManualReviewEntry(nil, &WriteOptions{WikiRoot: t.TempDir()})
	assert.NoError(t, err)
	assert.Empty(t, path)
}

func TestWriteManualReviewEntryDryRun(t *testing.T) {
	path, err := WriteManualReviewEntry(&ClassifyItem{
		URL:     "https://example.com",
		Title:   "Test",
		Summary: &StructuredSummary{Overview: "overview"},
	}, &WriteOptions{WikiRoot: t.TempDir(), DryRun: true})
	require.NoError(t, err)
	assert.Contains(t, path, "uncat.md")
}

func TestWriteManualReviewEntrySuccess(t *testing.T) {
	root := t.TempDir()
	path, err := WriteManualReviewEntry(&ClassifyItem{
		URL:     "https://example.com",
		Title:   "Test Item",
		Summary: &StructuredSummary{Overview: "overview", KeyPoints: []string{"point"}},
	}, &WriteOptions{WikiRoot: root})
	require.NoError(t, err)
	assert.Contains(t, path, "uncat.md")

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Contains(t, string(data), "Test Item")
}

// --- WriteSummary empty TopicPath ---

func TestWriteSummaryEmptyTopicPathRoutesToClassifyFailure(t *testing.T) {
	root := t.TempDir()
	path, err := WriteSummary(&ClassifyItem{
		URL:       "https://example.com",
		TopicPath: "",
		Summary:   &StructuredSummary{Overview: "overview"},
	}, &WriteOptions{WikiRoot: root})
	assert.Empty(t, path)
	require.NoError(t, err) // classify failure is logged, not an error

	// Verify a classify-rejected digest entry was written
	digestPath := filepath.Join(root, "digest-classify-rejected.jsonl")
	data, err := os.ReadFile(digestPath)
	require.NoError(t, err)
	assert.Contains(t, string(data), "empty topic path")
}

// --- buildEntry ---

func TestBuildEntryWithMetadataBlock(t *testing.T) {
	entry := buildEntry(&ClassifyItem{
		URL:           "https://example.com",
		Title:         "Test",
		Type:          TypeDeepDive,
		MetadataBlock: "Type: text\nauthor: Alice",
		Summary:       &StructuredSummary{Overview: "overview"},
	})
	assert.Contains(t, entry, "Test")
	assert.Contains(t, entry, "URL: https://example.com")
	assert.Contains(t, entry, "Type: text")
}

func TestBuildEntryFallbackTitle(t *testing.T) {
	entry := buildEntry(&ClassifyItem{
		URL:     "https://example.com",
		Title:   "",
		Type:    TypeDeepDive,
		Summary: &StructuredSummary{Overview: "overview"},
	})
	assert.Contains(t, entry, "https://example.com")
}

func TestBuildEntryFallbackMetadataBlock(t *testing.T) {
	entry := buildEntry(&ClassifyItem{
		URL:     "https://example.com",
		Title:   "Test",
		Type:    TypeDeepDive,
		Summary: &StructuredSummary{Overview: "overview"},
	})
	assert.Contains(t, entry, "Type: research")
}

// --- appendEntryBody ---

func TestAppendEntryBodyNewDateSection(t *testing.T) {
	result := appendEntryBody("", "## 2026-06-20", "### Entry\nContent")
	assert.Contains(t, result, "## 2026-06-20")
	assert.Contains(t, result, "### Entry")
}

func TestAppendEntryBodyExistingDateSection(t *testing.T) {
	existing := "## 2026-06-20\n\n### Old Entry\n"
	result := appendEntryBody(existing, "## 2026-06-20", "### New Entry")
	assert.Contains(t, result, "### Old Entry")
	assert.Contains(t, result, "### New Entry")
}

func TestAppendEntryBodyInsertBeforeOtherDate(t *testing.T) {
	existing := "## 2026-06-19\n\n### Old\n"
	result := appendEntryBody(existing, "## 2026-06-20", "### New")
	assert.Contains(t, result, "## 2026-06-20")
	assert.Contains(t, result, "## 2026-06-19")
}

// --- renderContent ---

func TestRenderContent(t *testing.T) {
	fm := &SummaryFrontmatter{
		Title:     "test",
		Date:      "2026-06-20",
		Source:    "rss2nl-wiki",
		Type:      "research",
		TotalURLs: 1,
		Succeeded: 1,
	}
	content := renderContent(fm, "body content")
	assert.True(t, strings.HasPrefix(content, "---"))
	assert.Contains(t, content, "title: test")
	assert.Contains(t, content, "body content")
}

// --- ScanWikiCandidates ---

func TestScanWikiCandidatesEmpty(t *testing.T) {
	candidates := scanWikiCandidates(t.TempDir())
	assert.Empty(t, candidates)
}

func TestScanWikiCandidatesWithDirs(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "tech", "research", "go-cli"), 0o700))
	require.NoError(t, os.MkdirAll(filepath.Join(root, "hidden", "type", "topic"), 0o700))
	require.NoError(t, os.MkdirAll(filepath.Join(root, ".hidden", "type", "topic"), 0o700))

	candidates := scanWikiCandidates(root)
	// Should find tech/research/go-cli, skip .hidden
	found := false
	for _, c := range candidates {
		if c.Path == "tech/research/go-cli" {
			found = true
		}
		assert.NotContains(t, c.Path, ".hidden")
	}
	assert.True(t, found)
}

// --- rankTopicCandidates ---

func TestRankTopicCandidatesNoLimit(t *testing.T) {
	candidates := []ghindex.TopicCandidate{
		{Path: "a/b/c", Display: "c"},
		{Path: "d/e/f", Display: "f"},
	}
	result := rankTopicCandidates(candidates, "c", 0)
	assert.Len(t, result, 2)
}

func TestRankTopicCandidatesWithLimit(t *testing.T) {
	candidates := []ghindex.TopicCandidate{
		{Path: "ai/tool/demo", Display: "demo"},
		{Path: "dev/cli/tool", Display: "tool"},
	}
	result := rankTopicCandidates(candidates, "ai tool demo", 1)
	assert.Len(t, result, 1)
	assert.Equal(t, "ai/tool/demo", result[0].Path)
}

func TestRankTopicCandidatesLowScoreFallback(t *testing.T) {
	// Need more candidates than limit, with zero score, to trigger the 40-cap fallback
	candidates := make([]ghindex.TopicCandidate, 50)
	for i := range candidates {
		candidates[i] = ghindex.TopicCandidate{Path: "xx/yy/zz", Display: "zz"}
	}
	// limit < len(candidates) so ranking runs; query won't match any tokens
	result := rankTopicCandidates(candidates, "qqqqqqqqqqqqq", 45)
	assert.LessOrEqual(t, len(result), 40) // low score caps at 40
}

// --- topicTokens ---

func TestTopicTokens(t *testing.T) {
	tokens := topicTokens("ai/tool/demo, test-value")
	assert.Contains(t, tokens, "ai")
	assert.Contains(t, tokens, "tool")
	assert.Contains(t, tokens, "demo")
	assert.Contains(t, tokens, "test")
	assert.Contains(t, tokens, "value")
}

// --- formatTopicCandidates ---

func TestFormatTopicCandidates(t *testing.T) {
	candidates := []ghindex.TopicCandidate{
		{Path: "ai/tool/demo", Display: "Demo Tool", Source: "wiki"},
		{Path: "dev/cli/tool", Display: "tool", Source: "gh:config"},
	}
	result := formatTopicCandidates(candidates)
	assert.Contains(t, result, "path: ai/tool/demo | title: Demo Tool | source: wiki")
	// display "tool" != path "dev/cli/tool", so title IS shown
	assert.Contains(t, result, "title: tool")
}

func TestFormatTopicCandidatesSameDisplayAsPath(t *testing.T) {
	candidates := []ghindex.TopicCandidate{
		{Path: "dev/cli/tool", Display: "dev/cli/tool", Source: "gh:config"},
	}
	result := formatTopicCandidates(candidates)
	// When display == path, no title is shown
	assert.Contains(t, result, "path: dev/cli/tool | source: gh:config")
	assert.NotContains(t, result, "title:")
}

// --- Classifier options ---

func TestClassifierOptions(t *testing.T) {
	c := NewClassifier(nil, "/wiki", "https://example.com/gh.yml",
		WithGHTopicsCachePath("/tmp/cache"),
		WithGHTopicsMaxAge(2*time.Hour),
		WithCandidateLimit(50),
		WithMaxContentSize(10000),
	)
	assert.Equal(t, "/tmp/cache", c.GhTopicsCachePath)
	assert.Equal(t, 2*time.Hour, c.GhTopicsMaxAge)
	assert.Equal(t, 50, c.CandidateLimit)
	assert.Equal(t, 10000, c.MaxContentSize)
}

func TestNewClassifierDefaults(t *testing.T) {
	c := NewClassifier(nil, "/wiki", "")
	assert.Equal(t, 120, c.CandidateLimit)
	assert.Equal(t, 0.30, c.MinConfidence)
	assert.Equal(t, 20000, c.MaxContentSize)
}

// --- isValidClassifyType / isValidContentType ---

func TestIsValidClassifyType(t *testing.T) {
	assert.True(t, isValidClassifyType(TypeRepoEval))
	assert.True(t, isValidClassifyType(TypeDeepDive))
	assert.True(t, isValidClassifyType(TypeInbox))
	assert.False(t, isValidClassifyType("invalid"))
}

func TestIsValidContentType(t *testing.T) {
	assert.True(t, isValidContentType(ContentText))
	assert.True(t, isValidContentType(ContentVideo))
	assert.True(t, isValidContentType(ContentAudio))
	assert.False(t, isValidContentType("invalid"))
}

// --- validateClassifyResult ---

func TestValidateClassifyResultNil(t *testing.T) {
	err := validateClassifyResult(nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "nil")
}

func TestParseClassifyOnlyResult(t *testing.T) {
	raw := `{"topicPath":"ai/tool","wikiType":"research","contentType":"text","confidence":0.9}`
	result, err := parseClassifyOnlyResult(raw)
	require.NoError(t, err)
	assert.Equal(t, "ai/tool", result.TopicPath)
}

func TestParseClassifyOnlyResultInvalid(t *testing.T) {
	_, err := parseClassifyOnlyResult("not json")
	require.Error(t, err)
}

// --- isManualReviewWithGoodContent ---

func TestIsManualReviewWithGoodContent(t *testing.T) {
	c := &Classifier{}
	assert.True(t, c.isManualReviewWithGoodContent(&aiClassification{
		NeedsManualReview: true,
		Summary:           &StructuredSummary{Overview: "good content"},
	}))
	assert.False(t, c.isManualReviewWithGoodContent(nil))
	assert.False(t, c.isManualReviewWithGoodContent(&aiClassification{NeedsManualReview: false}))
	assert.False(t, c.isManualReviewWithGoodContent(&aiClassification{
		NeedsManualReview: true,
		Summary:           &StructuredSummary{Overview: ""},
	}))
}

// --- rejectedClassifyResult nil ---

func TestRejectedClassifyResultNil(t *testing.T) {
	assert.Nil(t, rejectedClassifyResult(nil, "", nil))
}

// --- buildReviewEntryMeta ---

func TestBuildReviewEntryMeta(t *testing.T) {
	title, meta := buildReviewEntryMeta(&ClassifyItem{
		URL:           "https://example.com",
		Title:         "Test",
		MetadataBlock: "Type: text",
	})
	assert.Equal(t, "Test", title)
	assert.Contains(t, meta, "URL: https://example.com")
	assert.Contains(t, meta, "Type: text")
}

func TestBuildReviewEntryMetaFallback(t *testing.T) {
	title, meta := buildReviewEntryMeta(&ClassifyItem{
		URL:  "https://example.com",
		Type: TypeDeepDive,
	})
	assert.Equal(t, "https://example.com", title)
	assert.Contains(t, meta, "Type: research")
}

// --- assessContentQuality ---

func TestAssessContentQualityEmptyBody(t *testing.T) {
	q := assessContentQuality("Title", "", "https://example.com")
	assert.False(t, q.OK)
	assert.Equal(t, "empty body", q.Reason)
}

func TestAssessContentQualityCloudflare(t *testing.T) {
	q := assessContentQuality("Title", "just a moment checking your browser please wait", "https://example.com")
	assert.False(t, q.OK)
	assert.Contains(t, q.Reason, "cloudflare")
}

func TestAssessContentQualityLowQuality(t *testing.T) {
	q := assessContentQuality("Title", "this page requires javascript to display", "https://example.com")
	assert.False(t, q.OK)
}

func TestAssessContentQualityShort(t *testing.T) {
	q := assessContentQuality("Title", "too short", "https://example.com")
	assert.False(t, q.OK)
	assert.Equal(t, "too short", q.Reason)
}

func TestAssessContentQualityGood(t *testing.T) {
	body := strings.Repeat("This is a long article with enough content to pass quality checks. ", 10)
	q := assessContentQuality("Good Article", body, "https://example.com/article")
	assert.True(t, q.OK)
}

// --- contentQuality struct ---

func TestContentQualityStruct(t *testing.T) {
	q := contentQuality{OK: true}
	assert.True(t, q.OK)
	q2 := contentQuality{Reason: "bad"}
	assert.False(t, q2.OK)
	assert.Equal(t, "bad", q2.Reason)
}

// --- WriteFailureEntry nil opts ---

func TestWriteFailureEntryNilOpts(t *testing.T) {
	_, err := WriteFailureEntry(&ClassifyItem{}, FailureFetch, "info", nil)
	require.Error(t, err)
}

// --- LogSuccessEntry nil opts ---

func TestLogSuccessEntryNilOpts(t *testing.T) {
	path, err := LogSuccessEntry(&ClassifyItem{}, "", nil)
	assert.NoError(t, err)
	assert.Empty(t, path)
}

// --- LogSuccessEntry with opts ---

func TestLogSuccessEntryDryRun(t *testing.T) {
	path, err := LogSuccessEntry(&ClassifyItem{URL: "https://example.com"}, "output.md", &WriteOptions{
		WikiRoot: t.TempDir(), DryRun: true,
	})
	require.NoError(t, err)
	assert.NotEmpty(t, path)
}

// --- fetch failure context ---

func TestNewFetcherDefaults(t *testing.T) {
	f := NewFetcher()
	assert.NotNil(t, f.driver)
	assert.Equal(t, 5000, f.MaxBodySize)
	assert.True(t, f.MediaEnabled)
}

func TestNewFetcherWithOptions(t *testing.T) {
	f := NewFetcher(WithMediaEnabled(false))
	assert.False(t, f.MediaEnabled)
}

func TestFetcherMediaMaxBodySize(t *testing.T) {
	f := NewFetcher()
	f.MaxBodySize = 5000
	assert.Equal(t, 20000, f.mediaMaxBodySize())

	f.MaxBodySize = 0
	assert.Equal(t, 20000, f.mediaMaxBodySize())
}

func TestMediaContentResult(t *testing.T) {
	r := mediaContentResult("Title", "https://example.com", "source", "content", 1000)
	assert.Equal(t, "Title", r.Title)
	assert.Contains(t, r.Body, "Transcript source: source")
	assert.Contains(t, r.Body, "content")
}

func TestMediaContentResultTruncates(t *testing.T) {
	content := strings.Repeat("x", 2000)
	r := mediaContentResult("Title", "https://example.com", "source", content, 100)
	assert.LessOrEqual(t, len(r.Body), 500) // contains prefix + truncated content
}

// --- Fetcher direct audio ---

func TestFetcherDirectAudioMediaDisabled(t *testing.T) {
	f := NewFetcher(WithMediaEnabled(false))
	result := f.FetchContent(context.Background(), "https://example.com/audio.mp3", ContentAudio)
	require.NotNil(t, result)
	assert.Contains(t, result.Error, "media extraction disabled")
}
