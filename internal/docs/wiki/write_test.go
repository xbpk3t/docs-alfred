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
	require.Len(t, entries, 3)
	assert.Equal(t, "https://t.co/abc", entries[0].URL)
	assert.Equal(t, 0, entries[0].LineIndex)
	assert.Equal(t, "https://x.com/user/status/1", entries[1].URL)
	assert.Equal(t, 1, entries[1].LineIndex)
	assert.Equal(t, "https://example.com/post", entries[2].URL)
	assert.Equal(t, 2, entries[2].LineIndex)
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

func TestParseInboxRejectsMalformedMarkdownURLCapture(t *testing.T) {
	path := filepath.Join(t.TempDir(), "inbox.md")
	require.NoError(t, os.WriteFile(path, []byte(`- https://t.co/abc](https://x.com/user/status/1)
`), 0o600))

	entries, err := ParseInbox(path)

	require.NoError(t, err)
	require.Empty(t, entries)
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
