package wiki

import (
	"os"
	"path/filepath"
	"testing"

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
		Summary:   "summary",
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
	assert.Equal(t, filepath.Join(root, "failed", "fetch-failed.md"), path)
	_, err = os.Stat(filepath.Join(root, "failed"))
	assert.True(t, os.IsNotExist(err))
}
