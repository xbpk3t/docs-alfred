package wiki

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xbpk3t/docs-alfred/pkg/urlutil"
)

func TestParseSummaryFrontmatter(t *testing.T) {
	raw := `---
title: demo
date: 2026-06-06
source: wiki-inbox
batch_id: wiki-2026-06-06
total_urls: 2
succeeded: 1
failed: 1
---

## 2026-06-06

body
`

	result := parseSummaryFrontmatter(raw)

	require.NotNil(t, result)
	assert.Equal(t, "demo", result.fm.Title)
	assert.Equal(t, "wiki-2026-06-06", result.fm.BatchID)
	assert.Contains(t, result.body, "## 2026-06-06")
}

func TestParseSummaryFrontmatterBodyCanContainDelimiters(t *testing.T) {
	raw := `---
title: demo
date: 2026-06-06
---

before
---
after
`

	result := parseSummaryFrontmatter(raw)

	require.NotNil(t, result)
	assert.Contains(t, result.body, "before\n---\nafter")
}

func TestParseSummaryFrontmatterRequiresLeadingFrontmatter(t *testing.T) {
	raw := `intro
---
title: demo
---
`

	assert.Nil(t, parseSummaryFrontmatter(raw))
}

func TestWriteSummaryDryRunDoesNotCreateDirectoryOrFile(t *testing.T) {
	root := t.TempDir()
	item := &ClassifyItem{
		URL:       "https://example.com/a",
		Title:     "A",
		TopicPath: "topic/path",
		Type:      TypeDeepDive,
		Summary:   &StructuredSummary{Overview: "summary"},
	}

	path, err := WriteSummary(item, &WriteOptions{WikiRoot: root, DryRun: true})

	require.NoError(t, err)
	assert.Equal(t, filepath.Join(root, "topic", "path", "summary.md"), path)
	_, err = os.Stat(filepath.Join(root, "topic"))
	assert.True(t, os.IsNotExist(err))
}

func TestWriteFailureEntryDryRunDoesNotCreateDirectoryOrFile(t *testing.T) {
	root := t.TempDir()
	item := &ClassifyItem{
		URL:   "https://example.com/a",
		Title: "A",
	}

	path, err := WriteFailureEntry(item, FailureFetch, "fetch failed", &WriteOptions{WikiRoot: root, DryRun: true})

	require.NoError(t, err)
	assert.Equal(t, filepath.Join(root, "digest-fetch-error.jsonl"), path)
	_, err = os.Stat(filepath.Join(root, "digest-fetch-error.jsonl"))
	assert.True(t, os.IsNotExist(err))
}

func TestWriteSummaryConcurrentSameTopicDoesNotLoseEntries(t *testing.T) {
	root := t.TempDir()
	const count = 20
	var wg sync.WaitGroup
	for i := range count {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, err := WriteSummary(&ClassifyItem{
				URL:       fmt.Sprintf("https://example.com/%02d", i),
				Title:     fmt.Sprintf("Item %02d", i),
				TopicPath: "topic/path",
				Type:      TypeDeepDive,
				Summary:   &StructuredSummary{Overview: "summary"},
			}, &WriteOptions{WikiRoot: root})
			require.NoError(t, err)
		}(i)
	}
	wg.Wait()

	data, err := os.ReadFile(filepath.Join(root, "topic", "path", "summary.md"))
	require.NoError(t, err)
	parsed := parseSummaryFrontmatter(string(data))
	require.NotNil(t, parsed)
	require.Equal(t, count, parsed.fm.TotalURLs)
	require.Equal(t, count, parsed.fm.Succeeded)
	for i := range count {
		require.Equal(t, 1, strings.Count(string(data), fmt.Sprintf("https://example.com/%02d", i)))
	}
}

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

func TestBuildFailureEntryTruncatesUTF8Safely(t *testing.T) {
	entry := buildFailureEntry(&ClassifyItem{
		URL:     "https://example.com/a",
		Title:   "A",
		Summary: &StructuredSummary{Overview: strings.Repeat("你好", 400)},
	}, "failed")

	assert.True(t, utf8.ValidString(entry))
	assert.Contains(t, entry, "...")
}

// --- lockPath ---

func TestLockPathReturnsUnlockFunc(t *testing.T) {
	unlock := lockPath(filepath.Join(t.TempDir(), "test.md"))
	assert.NotNil(t, unlock)
	unlock()
}

func TestLockPathConcurrentAccess(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.md")
	var wg sync.WaitGroup
	for range 10 {
		wg.Go(func() {
			unlock := lockPath(path)
			unlock()
		})
	}
	wg.Wait()
}

// --- loadFrontmatter ---

func TestLoadFrontmatterNonFrontmatterContent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "summary.md")
	require.NoError(t, os.WriteFile(path, []byte("just plain content without frontmatter"), 0o600))

	item := &ClassifyItem{TopicPath: "topic/path"}
	fm, body := loadFrontmatter(path, item, "2026-06-20", "batch-1")

	assert.Equal(t, "path", fm.Title)
	assert.Equal(t, "2026-06-20", fm.Date)
	assert.Equal(t, "batch-1", fm.BatchID)
	assert.Equal(t, 0, fm.TotalURLs)
	assert.Contains(t, body, "just plain content")
}

func TestLoadFrontmatterWithExistingFrontmatter(t *testing.T) {
	path := filepath.Join(t.TempDir(), "summary.md")
	require.NoError(t, os.WriteFile(path, []byte(`---
title: existing
date: 2026-06-01
total_urls: 5
succeeded: 3
---

existing body
`), 0o600))

	item := &ClassifyItem{TopicPath: "topic/path"}
	fm, body := loadFrontmatter(path, item, "2026-06-20", "batch-1")

	assert.Equal(t, "existing", fm.Title)
	assert.Equal(t, 5, fm.TotalURLs)
	assert.Equal(t, 3, fm.Succeeded)
	assert.Contains(t, body, "existing body")
}

func TestLoadFrontmatterNewFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nonexistent.md")
	item := &ClassifyItem{TopicPath: "topic/path"}
	fm, body := loadFrontmatter(path, item, "2026-06-20", "batch-1")

	assert.Equal(t, "path", fm.Title)
	assert.Equal(t, "2026-06-20", fm.Date)
	assert.Empty(t, body)
}

// --- appendEntryToExistingDateSection ---

func TestAppendEntryToExistingDateSectionMissingDate(t *testing.T) {
	existing := "## 2026-06-19\n\n### Old Entry\n"
	result := appendEntryToExistingDateSection(existing, "## 2026-06-20", "### New Entry")
	// Date heading not found, returns existing unchanged
	assert.Equal(t, existing, result)
}

func TestAppendEntryToExistingDateSectionWithNextHeading(t *testing.T) {
	existing := "## 2026-06-20\n\n### Old Entry\n\n## 2026-06-19\n\nolder"
	result := appendEntryToExistingDateSection(existing, "## 2026-06-20", "### New Entry")
	assert.Contains(t, result, "### Old Entry")
	assert.Contains(t, result, "### New Entry")
	assert.Contains(t, result, "## 2026-06-19")
}

func TestAppendEntryToExistingDateSectionAtEnd(t *testing.T) {
	existing := "## 2026-06-20\n\n### Old Entry\n"
	result := appendEntryToExistingDateSection(existing, "## 2026-06-20", "### New Entry")
	assert.Contains(t, result, "### Old Entry")
	assert.Contains(t, result, "### New Entry")
}

// --- appendToDateSectionAndWrite ---

func TestAppendToDateSectionAndWriteExistingContent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "uncat.md")
	require.NoError(t, os.WriteFile(path, []byte("## 2026-06-20\n\n### Existing\n"), 0o600))

	err := appendToDateSectionAndWrite(path, "## 2026-06-20", "### New Entry\nContent")
	require.NoError(t, err)

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Contains(t, string(data), "### Existing")
	assert.Contains(t, string(data), "### New Entry")
}

func TestAppendToDateSectionAndWriteNewDateInExistingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "uncat.md")
	require.NoError(t, os.WriteFile(path, []byte("## 2026-06-19\n\n### Old\n"), 0o600))

	err := appendToDateSectionAndWrite(path, "## 2026-06-20", "### New Entry\nContent")
	require.NoError(t, err)

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Contains(t, string(data), "## 2026-06-20")
	assert.Contains(t, string(data), "## 2026-06-19")
}

func TestAppendToDateSectionAndWriteEmptyFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "uncat.md")

	err := appendToDateSectionAndWrite(path, "## 2026-06-20", "### Entry\nContent")
	require.NoError(t, err)

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Contains(t, string(data), "## 2026-06-20")
	assert.Contains(t, string(data), "### Entry")
}

// --- itemURL ---

func TestItemURLNil(t *testing.T) {
	assert.Empty(t, itemURL(nil))
}

func TestItemURLNotNil(t *testing.T) {
	assert.Equal(t, "https://example.com", itemURL(&ClassifyItem{URL: "https://example.com"}))
}

// --- buildFailureEntry ---

func TestBuildFailureEntryWithSummary(t *testing.T) {
	entry := buildFailureEntry(&ClassifyItem{
		URL:     "https://example.com",
		Title:   "Test",
		Summary: &StructuredSummary{Overview: "overview", KeyPoints: []string{"point"}},
	}, "reason")
	assert.Contains(t, entry, "Test")
	assert.Contains(t, entry, "https://example.com")
	assert.Contains(t, entry, "reason")
	assert.Contains(t, entry, "overview")
}

func TestBuildFailureEntryEmptyTitle(t *testing.T) {
	entry := buildFailureEntry(&ClassifyItem{
		URL:     "https://example.com",
		Summary: &StructuredSummary{Overview: "overview"},
	}, "reason")
	assert.Contains(t, entry, "https://example.com")
}

func TestBuildFailureEntryNoSummary(t *testing.T) {
	entry := buildFailureEntry(&ClassifyItem{
		URL:   "https://example.com",
		Title: "Test",
	}, "reason")
	assert.Contains(t, entry, "(无内容)")
}

// --- cleanFlushedInboxLine ---

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
	root := t.TempDir()
	path, err := WriteManualReviewEntry(&ClassifyItem{
		URL:           "https://example.com",
		Title:         "Test Item",
		MetadataBlock: "Type: text\nauthor: Alice",
		Summary:       &StructuredSummary{Overview: "overview", KeyPoints: []string{"point"}},
	}, &WriteOptions{WikiRoot: root})
	require.NoError(t, err)

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Contains(t, string(data), "Type: text")
	assert.Contains(t, string(data), "author: Alice")
}

// --- WriteSummary with MetadataBlock ---

func TestWriteSummaryWithMetadataBlock(t *testing.T) {
	root := t.TempDir()
	path, err := WriteSummary(&ClassifyItem{
		URL:           "https://example.com",
		Title:         "Test",
		TopicPath:     "topic/path",
		Type:          TypeDeepDive,
		MetadataBlock: "Type: text\nquality: high",
		Summary:       &StructuredSummary{Overview: "overview", KeyPoints: []string{"point"}},
	}, &WriteOptions{WikiRoot: root})
	require.NoError(t, err)

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Contains(t, string(data), "Type: text")
	assert.Contains(t, string(data), "quality: high")
}

// --- appendToFile ---

func TestAppendToFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.md")
	require.NoError(t, os.WriteFile(path, []byte("existing"), 0o600))

	err := appendToFile(path, " appended")
	require.NoError(t, err)

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, "existing appended", string(data))
}

func TestAppendToFileCreatesNew(t *testing.T) {
	path := filepath.Join(t.TempDir(), "new.md")

	err := appendToFile(path, "new content")
	require.NoError(t, err)

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, "new content", string(data))
}

// --- FailureKind String ---

func TestFailureKindString(t *testing.T) {
	assert.Equal(t, "fetch", FailureFetch.String())
	assert.Equal(t, "resolve", FailureResolve.String())
	assert.Equal(t, "extract", FailureExtract.String())
	assert.Equal(t, "classify", FailureClassify.String())
	assert.Equal(t, "ai", FailureAI.String())
}

// --- renderContent with YAML marshal error ---

func TestRenderContentFallbackOnMarshalError(t *testing.T) {
	// SummaryFrontmatter with valid fields should marshal fine
	fm := &SummaryFrontmatter{
		Title:  "test",
		Date:   "2026-06-20",
		Source: "rss2nl-wiki",
	}
	content := renderContent(fm, "body")
	assert.True(t, strings.HasPrefix(content, "---"))
	assert.Contains(t, content, "body")
}

// --- WriteSummary with full metadata ---

func TestWriteSummaryFullMetadata(t *testing.T) {
	root := t.TempDir()
	path, err := WriteSummary(&ClassifyItem{
		URL:           "https://example.com",
		Title:         "Full Test",
		TopicPath:     "tech/research/go",
		Type:          TypeDeepDive,
		MetadataBlock: "Type: text\nquality: high\nauthor: Alice\ntags: go, cli, tool",
		Summary: &StructuredSummary{
			Overview:    "This is a comprehensive overview",
			WorthNoting: "Something noteworthy",
			KeyPoints:   []string{"Point 1", "Point 2"},
			Detail:      "Detailed analysis",
		},
	}, &WriteOptions{WikiRoot: root, BatchID: "test-batch"})
	require.NoError(t, err)

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	content := string(data)
	assert.Contains(t, content, "Full Test")
	assert.Contains(t, content, "Type: text")
	assert.Contains(t, content, "quality: high")
	assert.Contains(t, content, "test-batch")
	assert.Contains(t, content, "overview")
	assert.Contains(t, content, "keyPoints")
	assert.Contains(t, content, "detail")
}

// --- WriteFailureEntry with real write ---

func TestWriteFailureEntryRealWrite(t *testing.T) {
	root := t.TempDir()
	item := &ClassifyItem{
		URL:   "https://example.com",
		Title: "Failed Item",
	}

	path, err := WriteFailureEntry(item, FailureExtract, "extraction failed", &WriteOptions{WikiRoot: root})
	require.NoError(t, err)
	assert.NotEmpty(t, path)

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Contains(t, string(data), "https://example.com")
	assert.Contains(t, string(data), "extraction failed")
}

// --- appendEntryBody edge cases ---

func TestAppendEntryBodyExistingContentNoDateHeading(t *testing.T) {
	existing := "Some existing content without date headings"
	result := appendEntryBody(existing, "## 2026-06-20", "### New Entry")
	assert.Contains(t, result, "Some existing content")
	assert.Contains(t, result, "## 2026-06-20")
	assert.Contains(t, result, "### New Entry")
}

func TestAppendEntryBodyExistingContentWithOtherDateHeading(t *testing.T) {
	existing := "## 2026-06-19\n\n### Old Entry\n"
	result := appendEntryBody(existing, "## 2026-06-20", "### New Entry")
	assert.Contains(t, result, "## 2026-06-20")
	assert.Contains(t, result, "## 2026-06-19")
	assert.Contains(t, result, "### New Entry")
}

func TestAppendEntryBodyEmptyExisting(t *testing.T) {
	result := appendEntryBody("", "## 2026-06-20", "### Entry")
	assert.Contains(t, result, "## 2026-06-20")
	assert.Contains(t, result, "### Entry")
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

// --- parseSummaryFrontmatter edge cases ---

func TestParseSummaryFrontmatterNoFrontmatter(t *testing.T) {
	result := parseSummaryFrontmatter("just content without frontmatter")
	assert.Nil(t, result)
}

func TestParseSummaryFrontmatterInvalidYAML(t *testing.T) {
	result := parseSummaryFrontmatter("---\ninvalid: [yaml: content\n---\nbody")
	assert.Nil(t, result)
}

// --- WriteManualReviewEntry full path ---

func TestWriteManualReviewEntryFull(t *testing.T) {
	root := t.TempDir()
	path, err := WriteManualReviewEntry(&ClassifyItem{
		URL:           "https://example.com/article",
		Title:         "Review Item",
		Type:          TypeDeepDive,
		MetadataBlock: "Type: text\nquality: medium",
		Summary: &StructuredSummary{
			Overview:    "Overview text",
			KeyPoints:   []string{"key1", "key2"},
			WorthNoting: "note",
		},
	}, &WriteOptions{WikiRoot: root, BatchID: "batch-1"})
	require.NoError(t, err)

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	content := string(data)
	assert.Contains(t, content, "Review Item")
	assert.Contains(t, content, "Type: text")
	assert.Contains(t, content, "overview")
	assert.Contains(t, content, "Overview text")
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
