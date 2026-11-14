package write

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xbpk3t/docs-alfred/internal/docs/wiki/types"
	"github.com/xbpk3t/docs-alfred/pkg/urlutil"
)

func TestParseInboxMarkdownLinksAndBareURLs(t *testing.T) {
	path := filepath.Join(t.TempDir(), "inbox.md")
	require.NoError(t, os.WriteFile(path, []byte(`- [tweet](https://t.co/abc)
- [X post with quote https://t.co/quoted](https://x.com/user/status/1)
- https://example.com/post
- duplicate https://example.com/post/
`), 0o600))

	entries, err := ParseInbox(path)

	require.NoError(t, err)
	require.Len(t, entries, 4)
	assert.Equal(t, "https://t.co/abc", entries[0].URL)
	assert.Equal(t, 0, entries[0].LineIndex)
	assert.Equal(t, "https://t.co/quoted", entries[1].URL)
	assert.Equal(t, 1, entries[1].LineIndex)
	assert.Equal(t, "https://x.com/user/status/1", entries[2].URL)
	assert.Equal(t, 1, entries[2].LineIndex)
	assert.Equal(t, "https://example.com/post", entries[3].URL)
	assert.Equal(t, 2, entries[3].LineIndex)
}

func TestParseInboxExtractsMultipleURLsFromOneLine(t *testing.T) {
	path := filepath.Join(t.TempDir(), "inbox.md")
	require.NoError(t, os.WriteFile(path, []byte(`- https://example.com/a https://example.com/b
`), 0o600))

	entries, err := ParseInbox(path)

	require.NoError(t, err)
	require.Len(t, entries, 2)
	assert.Equal(t, "https://example.com/a", entries[0].URL)
	assert.Equal(t, 0, entries[0].LineIndex)
	assert.Equal(t, "https://example.com/b", entries[1].URL)
	assert.Equal(t, 0, entries[1].LineIndex)
}

func TestParseInboxExtractsURLsFromMalformedMarkdownURLCapture(t *testing.T) {
	path := filepath.Join(t.TempDir(), "inbox.md")
	require.NoError(t, os.WriteFile(path, []byte(`- https://t.co/abc](https://x.com/user/status/1)
`), 0o600))

	entries, err := ParseInbox(path)

	require.NoError(t, err)
	require.Len(t, entries, 2)
	assert.Equal(t, "https://t.co/abc", entries[0].URL)
	assert.Equal(t, "https://x.com/user/status/1", entries[1].URL)
}

func TestFlushInboxKeepsUnhandledURLOnMultiURLLine(t *testing.T) {
	path := filepath.Join(t.TempDir(), "inbox.md")
	require.NoError(t, os.WriteFile(path, []byte(`- https://example.com/handled https://example.com/unhandled
`), 0o600))

	err := FlushInbox(path, map[int][]string{0: []string{"https://example.com/handled"}})

	require.NoError(t, err)
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.NotContains(t, string(data), "https://example.com/handled")
	assert.Contains(t, string(data), "https://example.com/unhandled")
}

func TestFlushInboxRemovesLineWhenAllURLsHandled(t *testing.T) {
	path := filepath.Join(t.TempDir(), "inbox.md")
	require.NoError(t, os.WriteFile(path, []byte(`- https://example.com/a https://example.com/b
- https://example.com/c
`), 0o600))

	err := FlushInbox(path, map[int][]string{0: []string{"https://example.com/a", "https://example.com/b"}})

	require.NoError(t, err)
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.NotContains(t, string(data), "https://example.com/a")
	assert.NotContains(t, string(data), "https://example.com/b")
	assert.Contains(t, string(data), "https://example.com/c")
}

func TestFlushInboxKeepsUnhandledMarkdownLinkURL(t *testing.T) {
	path := filepath.Join(t.TempDir(), "inbox.md")
	require.NoError(t, os.WriteFile(path, []byte(`- [done](https://example.com/done) [todo](https://example.com/todo)
`), 0o600))

	err := FlushInbox(path, map[int][]string{0: []string{"https://example.com/done"}})

	require.NoError(t, err)
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.NotContains(t, string(data), "https://example.com/done")
	assert.Contains(t, string(data), "https://example.com/todo")
}

// --- appendEntryToExistingSection ---

func TestAppendEntryToExistingSectionMissingSection(t *testing.T) {
	existing := "## 2026-06-19\n\n### Old Entry\n"
	result, ok := appendEntryToExistingSection(existing, "## 2026-06-20", "### New Entry")
	// Section heading not found, returns existing unchanged
	assert.False(t, ok)
	assert.Equal(t, existing, result)
}

func TestAppendEntryToExistingSectionWithNextHeading(t *testing.T) {
	existing := "## 2026-06-20\n\n### Old Entry\n\n## 2026-06-19\n\nolder"
	result, ok := appendEntryToExistingSection(existing, "## 2026-06-20", "### New Entry")
	require.True(t, ok)
	assert.Contains(t, result, "### Old Entry")
	assert.Contains(t, result, "### New Entry")
	assert.Contains(t, result, "## 2026-06-19")
}

func TestAppendEntryToExistingSectionAtEnd(t *testing.T) {
	existing := "## 2026-06-20\n\n### Old Entry\n"
	result, ok := appendEntryToExistingSection(existing, "## 2026-06-20", "### New Entry")
	require.True(t, ok)
	assert.Contains(t, result, "### Old Entry")
	assert.Contains(t, result, "### New Entry")
}

func TestSectionBodyStartRequiresExactHeadingLine(t *testing.T) {
	body := "## AI/agent/agent-fwk\n\n### Entry\n"
	// Nested topic paths share prefixes: the parent heading must not match the
	// child's section.
	assert.Equal(t, -1, sectionBodyStart(body, "## AI/agent"))
	assert.Greater(t, sectionBodyStart(body, "## AI/agent/agent-fwk"), 0)
	assert.Greater(t, sectionBodyStart("## AI/agent\n\n### Entry\n", "## AI/agent"), 0)
}

func TestSectionBodyStartHeadingAtEndWithoutNewline(t *testing.T) {
	// Hand-edited files may lack a trailing newline; the final heading line
	// still identifies its (empty) section.
	assert.Equal(t, len("## AI/agent"), sectionBodyStart("## AI/agent", "## AI/agent"))
	assert.Equal(t, len("body\n## AI/agent"), sectionBodyStart("body\n## AI/agent", "## AI/agent"))
}

// --- mergeDateSection ---

func TestMergeDateSectionExistingDate(t *testing.T) {
	merged := mergeDateSection("## 2026-06-20\n\n### Existing\n", "## 2026-06-20", "### New Entry\nContent")
	assert.Contains(t, merged, "### Existing")
	assert.Contains(t, merged, "### New Entry")
}

func TestMergeDateSectionNewDateInExistingFile(t *testing.T) {
	merged := mergeDateSection("## 2026-06-19\n\n### Old\n", "## 2026-06-20", "### New Entry\nContent")
	assert.Contains(t, merged, "## 2026-06-20")
	assert.Contains(t, merged, "## 2026-06-19")
}

func TestMergeDateSectionEmpty(t *testing.T) {
	merged := mergeDateSection("", "## 2026-06-20", "### Entry\nContent")
	assert.Contains(t, merged, "## 2026-06-20")
	assert.Contains(t, merged, "### Entry")
}

// --- itemURL ---

func TestItemURLNil(t *testing.T) {
	assert.Empty(t, itemURL(nil))
}

func TestItemURLNotNil(t *testing.T) {
	assert.Equal(t, "https://example.com", itemURL(&types.ClassifyItem{URL: "https://example.com"}))
}

// --- buildFailureEntry ---

func TestCleanFlushedInboxLinePunctuationArtifacts(t *testing.T) {
	assert.Equal(t, "- text", cleanFlushedInboxLine("  - , text  "))
	assert.Equal(t, "- text", cleanFlushedInboxLine("  - . text  "))
	assert.Equal(t, "- text", cleanFlushedInboxLine("  - ; text  "))
	assert.Equal(t, "- text", cleanFlushedInboxLine("  - : text  "))
}

func TestCleanFlushedInboxLineDoubleSpaces(t *testing.T) {
	assert.Equal(t, "a b c", cleanFlushedInboxLine("  a  b  c  "))
}

// --- ParseInbox edge cases ---

func TestParseInboxNonexistentFile(t *testing.T) {
	_, err := ParseInbox(filepath.Join(t.TempDir(), "nonexistent.md"))
	require.Error(t, err)
}

func TestParseInboxEmptyFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "inbox.md")
	require.NoError(t, os.WriteFile(path, []byte(""), 0o600))

	entries, err := ParseInbox(path)
	require.NoError(t, err)
	assert.Empty(t, entries)
}

// --- FlushInbox edge cases ---

func TestFlushInboxNonexistentFile(t *testing.T) {
	err := FlushInbox(filepath.Join(t.TempDir(), "nonexistent.md"), map[int][]string{0: {"https://example.com"}})
	require.Error(t, err)
}

func TestFlushInboxEmptyResult(t *testing.T) {
	path := filepath.Join(t.TempDir(), "inbox.md")
	require.NoError(t, os.WriteFile(path, []byte("- https://example.com/a\n"), 0o600))

	err := FlushInbox(path, map[int][]string{0: {"https://example.com/a"}})
	require.NoError(t, err)

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Empty(t, string(data))
}

func TestFlushInboxNoHandledURLs(t *testing.T) {
	path := filepath.Join(t.TempDir(), "inbox.md")
	require.NoError(t, os.WriteFile(path, []byte("- https://example.com/a\n"), 0o600))

	err := FlushInbox(path, map[int][]string{})
	require.NoError(t, err)

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Contains(t, string(data), "https://example.com/a")
}

// --- normalizedURLSet ---

func TestNormalizedURLSet(t *testing.T) {
	set := normalizedURLSet([]string{"https://Example.com/path/", "https://example.com/path"})
	assert.True(t, set[urlutil.Normalize("https://example.com/path")])
}

// --- WriteManualReviewEntry with MetadataBlock ---

func TestWriteManualReviewEntryWithMetadataBlock(t *testing.T) {
	store := newFakeDigestStore()
	_, err := WriteManualReviewEntry(&types.ClassifyItem{
		URL:           "https://example.com",
		Title:         "Test Item",
		MetadataBlock: "Type: text\nauthor: Alice",
		Summary:       &types.StructuredSummary{Overview: "overview", KeyPoints: []string{"point"}},
	}, &WriteOptions{R2Store: store})
	require.NoError(t, err)

	content := string(store.objects["uncat.md"])
	assert.Contains(t, content, "---\ntitle: Test Item\n")
	assert.Contains(t, content, "Type: text")
	assert.Contains(t, content, "author: Alice")
	assert.NotContains(t, content, "```")
}

// --- WriteSummary with MetadataBlock ---

func TestWriteSummaryWithMetadataBlock(t *testing.T) {
	store := newFakeDigestStore()
	_, err := WriteSummary(&types.ClassifyItem{
		URL:           "https://example.com",
		Title:         "Test",
		TopicPath:     "topic/path",
		Type:          types.TypeDeepDive,
		MetadataBlock: "Type: text\nquality: high",
		Summary:       &types.StructuredSummary{Overview: "overview", KeyPoints: []string{"point"}},
	}, &WriteOptions{R2Store: store})
	require.NoError(t, err)

	content := string(store.objects[firstDigestKey(t, store)])
	assert.Contains(t, content, "Type: text")
	assert.Contains(t, content, "quality: high")
}

// --- appendToFile ---

func TestFailureKindString(t *testing.T) {
	assert.Equal(t, "fetch", types.FailureFetch.String())
	assert.Equal(t, "resolve", types.FailureResolve.String())
	assert.Equal(t, "extract", types.FailureExtract.String())
	assert.Equal(t, "classify", types.FailureClassify.String())
	assert.Equal(t, "ai", types.FailureAI.String())
}

// --- WriteSummary with full metadata ---

func TestWriteSummaryFullMetadata(t *testing.T) {
	store := newFakeDigestStore()
	_, err := WriteSummary(&types.ClassifyItem{
		URL:           "https://example.com",
		Title:         "Full Test",
		TopicPath:     "tech/research/go",
		Type:          types.TypeDeepDive,
		MetadataBlock: "Type: text\nquality: high\nauthor: Alice\ntags: go, cli, tool",
		Summary: &types.StructuredSummary{
			Overview:    "This is a comprehensive overview",
			WorthNoting: "Something noteworthy",
			KeyPoints:   []string{"Point 1", "Point 2"},
			Detail:      "Detailed analysis",
		},
	}, &WriteOptions{R2Store: store, BatchID: "test-batch"})
	require.NoError(t, err)

	key := firstDigestKey(t, store)
	assert.Contains(t, key, "test-batch")
	content := string(store.objects[key])
	assert.Contains(t, content, "title: Full Test")
	assert.Contains(t, content, "Type: text")
	assert.Contains(t, content, "quality: high")
	assert.Contains(t, content, "overview")
	assert.Contains(t, content, "keyPoints")
	assert.Contains(t, content, "detail")
}

// firstDigestKey returns the single digest object key a write test uploaded.
func firstDigestKey(t *testing.T, store *fakeDigestStore) string {
	t.Helper()
	for _, key := range store.puts {
		if strings.HasPrefix(key, "digest/") {
			return key
		}
	}
	t.Fatalf("no digest object uploaded; puts=%v", store.puts)

	return ""
}

// --- WriteFailureEntry with real write ---

func TestWriteFailureEntryRealWrite(t *testing.T) {
	store := newFakeDigestStore()
	item := &types.ClassifyItem{
		URL:   "https://example.com",
		Title: "Failed Item",
	}

	path, err := WriteFailureEntry(item, types.FailureExtract, types.FailureReasonPartialExtract, "extraction failed", &WriteOptions{R2Store: store})
	require.NoError(t, err)
	assert.Equal(t, "logs/digest-extract-error.jsonl", path)

	data := string(store.objects[path])
	assert.Contains(t, data, "https://example.com")
	assert.Contains(t, data, "extraction failed")
	assert.Contains(t, data, `"failureReason":"partial_extract"`)
}

// --- FlushInbox with multiple lines ---

func TestFlushInboxMultipleLines(t *testing.T) {
	path := filepath.Join(t.TempDir(), "inbox.md")
	require.NoError(t, os.WriteFile(path, []byte(`- https://example.com/a
- https://example.com/b
- https://example.com/c
`), 0o600))

	err := FlushInbox(path, map[int][]string{
		0: {"https://example.com/a"},
		2: {"https://example.com/c"},
	})
	require.NoError(t, err)

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.NotContains(t, string(data), "https://example.com/a")
	assert.Contains(t, string(data), "https://example.com/b")
	assert.NotContains(t, string(data), "https://example.com/c")
}

// Line indices are a snapshot; editing inbox.md during a run can shift them.
// Flush must still remove handled URLs and preserve every retryable one.
func TestFlushInboxSurvivesLineShift(t *testing.T) {
	path := filepath.Join(t.TempDir(), "inbox.md")
	require.NoError(t, os.WriteFile(path, []byte(`- https://example.com/new-retryable
- https://example.com/handled
- https://example.com/other-retryable
`), 0o600))

	err := FlushInbox(path, map[int][]string{0: {"https://example.com/handled"}})
	require.NoError(t, err)

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.NotContains(t, string(data), "https://example.com/handled")
	assert.Contains(t, string(data), "https://example.com/new-retryable")
	assert.Contains(t, string(data), "https://example.com/other-retryable")
}

// --- appendInboxEntry edge cases ---

func TestAppendInboxEntryEmptyURL(t *testing.T) {
	var entries []InboxEntry
	seen := make(map[string]bool)
	result := appendInboxEntry(entries, seen, "  ", 0)
	assert.Empty(t, result)
}

func TestAppendInboxEntryDuplicateURL(t *testing.T) {
	var entries []InboxEntry
	seen := map[string]bool{urlutil.Normalize("https://example.com"): true}
	result := appendInboxEntry(entries, seen, "https://example.com", 0)
	assert.Empty(t, result)
}

// --- flushInboxLine edge cases ---

func TestFlushInboxLineNoRefs(t *testing.T) {
	line := "just plain text without any URLs"
	result := flushInboxLine(line, map[string]bool{"https://example.com": true})
	assert.Equal(t, line, result)
}

func TestFlushInboxLineFlushedButUnhandled(t *testing.T) {
	line := "- https://example.com/a https://example.com/b"
	handled := map[string]bool{urlutil.Normalize("https://example.com/a"): true}
	result := flushInboxLine(line, handled)
	assert.NotEmpty(t, result)
	assert.NotContains(t, result, "https://example.com/a")
	assert.Contains(t, result, "https://example.com/b")
}

// --- WriteManualReviewEntry full path ---

func TestWriteManualReviewEntryFull(t *testing.T) {
	store := newFakeDigestStore()
	_, err := WriteManualReviewEntry(&types.ClassifyItem{
		URL:           "https://example.com/article",
		Title:         "Review Item",
		Type:          types.TypeDeepDive,
		MetadataBlock: "Type: text\nquality: medium",
		Summary: &types.StructuredSummary{
			Overview:    "Overview text",
			KeyPoints:   []string{"key1", "key2"},
			WorthNoting: "note",
		},
	}, &WriteOptions{R2Store: store, BatchID: "batch-1"})
	require.NoError(t, err)

	content := string(store.objects["uncat.md"])
	assert.Contains(t, content, "### Review Item")
	assert.Contains(t, content, "title: Review Item")
	assert.Contains(t, content, "Type: text")
	assert.Contains(t, content, "overview")
	assert.Contains(t, content, "Overview text")
	assert.NotContains(t, content, "```")
}

// --- extractInboxLineURLRefs ---

func TestExtractInboxLineURLRefsEmpty(t *testing.T) {
	refs := extractInboxLineURLRefs("just text, no url")
	assert.Empty(t, refs)
}

func TestExtractInboxLineURLRefsBareURL(t *testing.T) {
	refs := extractInboxLineURLRefs("https://example.com/article")
	require.Len(t, refs, 1)
	assert.Equal(t, "https://example.com/article", refs[0].URL)
}

func TestExtractInboxLineURLRefsMarkdownLink(t *testing.T) {
	refs := extractInboxLineURLRefs("[click here](https://example.com)")
	require.Len(t, refs, 1)
	assert.Equal(t, "https://example.com", refs[0].URL)
}

func TestExtractInboxLineURLRefsMixedBareAndMarkdown(t *testing.T) {
	refs := extractInboxLineURLRefs("- [tweet](https://t.co/abc) and https://example.com/post")
	require.Len(t, refs, 2)
	assert.Equal(t, "https://t.co/abc", refs[0].URL)
	assert.Equal(t, "https://example.com/post", refs[1].URL)
}

func TestExtractInboxLineURLRefsMalformedMarkdownExtractsURLs(t *testing.T) {
	refs := extractInboxLineURLRefs("https://t.co/abc](https://x.com/user/status/1)")
	require.Len(t, refs, 2)
	assert.Equal(t, "https://t.co/abc", refs[0].URL)
	assert.Equal(t, "https://x.com/user/status/1", refs[1].URL)
}

func TestExtractInboxLineURLRefsDedupFalsePreservesDuplicates(t *testing.T) {
	refs := extractInboxLineURLRefs("https://example.com/a https://example.com/a")
	require.Len(t, refs, 2)
	assert.Equal(t, "https://example.com/a", refs[0].URL)
	assert.Equal(t, "https://example.com/a", refs[1].URL)
}

func TestExtractInboxLineURLRefsBareAndMarkdownOrder(t *testing.T) {
	refs := extractInboxLineURLRefs("https://example.com/a [link](https://example.com/b)")
	require.Len(t, refs, 2)
	assert.Equal(t, "https://example.com/a", refs[0].URL)
	assert.Equal(t, "https://example.com/b", refs[1].URL)
}

// --- inbox tracking-param stripping ---

func TestParseInboxStripsTrackingParams(t *testing.T) {
	path := filepath.Join(t.TempDir(), "inbox.md")
	require.NoError(t, os.WriteFile(path, []byte(`- [video](https://www.bilibili.com/video/BV1BMdzB1EVX?spm_id_from=333.1387.favlist.content.click)
- https://example.com/post?utm_source=feed&bvid=BV1xx411c7mD
`), 0o600))

	entries, err := ParseInbox(path)

	require.NoError(t, err)
	require.Len(t, entries, 2)
	assert.Equal(t, "https://www.bilibili.com/video/BV1BMdzB1EVX", entries[0].URL)
	assert.Equal(t, "https://example.com/post?bvid=BV1xx411c7mD", entries[1].URL)
}

func TestFlushInboxMatchesStrippedHandledURL(t *testing.T) {
	path := filepath.Join(t.TempDir(), "inbox.md")
	require.NoError(t, os.WriteFile(path, []byte("- https://example.com/post?utm_source=feed\n"), 0o600))

	// Handled URLs come from parsed entries, i.e. already stripped; the inbox
	// line still carries the raw URL.
	require.NoError(t, FlushInbox(path, map[int][]string{0: {"https://example.com/post"}}))

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Empty(t, string(data))
}

func TestManualReviewContainsURL(t *testing.T) {
	content := "### A\n\n```markdown\nURL: https://example.com/post?utm_source=x\nType: inbox\n```\n"
	assert.True(t, manualReviewContainsURL(content, "https://example.com/post"))
	assert.False(t, manualReviewContainsURL(content, "https://example.com/other"))

	// Current format is YAML frontmatter. The encoder quotes URLs containing
	// '#', and the quoted value must still normalize to the same dedup key.
	frontmatter := "### B\n\n---\ntitle: B\nURL: \"https://example.com/post#section\"\n---\n\nbody\n"
	assert.True(t, manualReviewContainsURL(frontmatter, "https://example.com/post#section"))
}
