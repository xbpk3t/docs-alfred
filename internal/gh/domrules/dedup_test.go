package domrules

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunDuplicateCheck_NoDuplicates(t *testing.T) {
	dir := t.TempDir()
	yamlContent := `
- name: "Book A"
  author: "Author A"
  url: "https://example.com/a"
  score: 4
- name: "Book B"
  author: "Author B"
  url: "https://example.com/b"
  score: 3
`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "books.yml"), []byte(yamlContent), 0644))

	report, err := RunDuplicateCheck(dir)
	require.NoError(t, err)
	require.NotNil(t, report)
	assert.Equal(t, 0, len(report.URLDuplicates))
	assert.Equal(t, 0, len(report.NameAuthorDuplicates))
}

func TestRunDuplicateCheck_URLDuplicates(t *testing.T) {
	dir := t.TempDir()
	yamlContent := `
- name: "Book A"
  author: "Author A"
  url: "https://example.com/same"
  score: 4
- name: "Book B"
  author: "Author B"
  url: "https://example.com/same"
  score: 3
`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "books.yml"), []byte(yamlContent), 0644))

	report, err := RunDuplicateCheck(dir)
	require.NoError(t, err)
	require.NotNil(t, report)
	assert.Equal(t, 1, len(report.URLDuplicates), "should find one URL duplicate")
	assert.Equal(t, "https://example.com/same", report.URLDuplicates[0].URL)
}

func TestRunDuplicateCheck_NameAuthorDuplicates(t *testing.T) {
	dir := t.TempDir()
	yamlContent := `
- name: "Same Name"
  author: "Same Author"
  url: "https://example.com/different"
  score: 4
- name: "Same Name"
  author: "Same Author"
  url: "https://example.com/other"
  score: 3
`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "books.yml"), []byte(yamlContent), 0644))

	report, err := RunDuplicateCheck(dir)
	require.NoError(t, err)
	require.NotNil(t, report)
	assert.Equal(t, 1, len(report.NameAuthorDuplicates), "should find one name+author duplicate")
}

func TestRunDuplicateCheck_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	report, err := RunDuplicateCheck(dir)
	require.NoError(t, err)
	require.NotNil(t, report)
	assert.Equal(t, 0, len(report.URLDuplicates))
}

func TestRunGHDuplicateCheck_NoDuplicates(t *testing.T) {
	dir := t.TempDir()
	tagDir := filepath.Join(dir, "dev")
	require.NoError(t, os.MkdirAll(tagDir, 0755))
	yamlContent := `
- type: "language"
  repo:
    - url: "https://github.com/owner/repo-a"
    - url: "https://github.com/owner/repo-b"
`
	require.NoError(t, os.WriteFile(filepath.Join(tagDir, "go.yml"), []byte(yamlContent), 0644))

	report, err := RunGHDuplicateCheck(dir)
	require.NoError(t, err)
	require.NotNil(t, report)
	assert.Equal(t, 0, len(report.URLDuplicates))
}

func TestRunGHDuplicateCheck_WithDuplicates(t *testing.T) {
	dir := t.TempDir()
	tagDir := filepath.Join(dir, "dev")
	require.NoError(t, os.MkdirAll(tagDir, 0755))
	yamlContent := `
- type: "language"
  repo:
    - url: "https://github.com/owner/repo"
    - url: "https://github.com/owner/repo"
`
	require.NoError(t, os.WriteFile(filepath.Join(tagDir, "go.yml"), []byte(yamlContent), 0644))

	report, err := RunGHDuplicateCheck(dir)
	require.NoError(t, err)
	require.NotNil(t, report)
	assert.Equal(t, 1, len(report.URLDuplicates), "should find duplicate URL")
}

func TestFormatDuplicateReport(t *testing.T) {
	report := &DuplicateReport{
		URLDuplicates: []URLDupEntry{
			{
				URL: "https://example.com/dup",
				Entries: []ItemBrief{
					{File: "a.yml", Name: "A", Author: "Author A"},
					{File: "b.yml", Name: "B", Author: "Author B"},
				},
			},
		},
	}
	output := FormatDuplicateReport(report)
	assert.Contains(t, output, "重复 URL")
	assert.Contains(t, output, "https://example.com/dup")
}

func TestFormatGHDuplicateReport(t *testing.T) {
	report := &DuplicateReport{
		URLDuplicates: []URLDupEntry{
			{
				URL: "https://github.com/owner/repo",
				Entries: []ItemBrief{
					{File: "dev/go.yml: language (repo)"},
				},
			},
		},
	}
	output := FormatGHDuplicateReport(report)
	assert.Contains(t, output, "重复 URL")
	assert.Contains(t, output, "https://github.com/owner/repo")
}

func TestFormatDuplicateReport_NilReport(t *testing.T) {
	output := FormatDuplicateReport(nil)
	assert.NotEmpty(t, output) // should not panic
}

func TestFormatGHDuplicateReport_NilReport(t *testing.T) {
	output := FormatGHDuplicateReport(nil)
	assert.NotEmpty(t, output) // should not panic
}

func TestRunDuplicateCheck_NameAuthorDuplicatesFiltered(t *testing.T) {
	// When name+author duplicates also have URL duplicates, they should be filtered
	dir := t.TempDir()
	yamlContent := `
- name: "Same Name"
  author: "Same Author"
  url: "https://example.com/same"
  score: 4
- name: "Same Name"
  author: "Same Author"
  url: "https://example.com/same"
  score: 3
`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "books.yml"), []byte(yamlContent), 0644))

	report, err := RunDuplicateCheck(dir)
	require.NoError(t, err)
	// URL duplicate found
	assert.Equal(t, 1, len(report.URLDuplicates))
	// Name+author duplicates filtered because they were caught by URL
	assert.Equal(t, 0, len(report.NameAuthorDuplicates))
}

func TestRunGHDuplicateCheck_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	report, err := RunGHDuplicateCheck(dir)
	require.NoError(t, err)
	assert.NotNil(t, report)
	assert.Equal(t, 0, len(report.URLDuplicates))
}

func TestRunGHDuplicateCheck_UsingEntry(t *testing.T) {
	dir := t.TempDir()
	tagDir := filepath.Join(dir, "dev")
	require.NoError(t, os.MkdirAll(tagDir, 0755))
	yamlContent := `
- type: "language"
  using:
    url: "https://github.com/owner/repo"
  repo:
    - url: "https://github.com/owner/repo"
`
	require.NoError(t, os.WriteFile(filepath.Join(tagDir, "go.yml"), []byte(yamlContent), 0644))

	report, err := RunGHDuplicateCheck(dir)
	require.NoError(t, err)
	assert.Equal(t, 1, len(report.URLDuplicates))
}

func TestRunDuplicateCheck_MultiDoc(t *testing.T) {
	dir := t.TempDir()
	yamlContent := `---
- name: "Book A"
  author: "Author A"
  url: "https://example.com/a"
---
- name: "Book A"
  author: "Author A"
  url: "https://example.com/b"
`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "books.yml"), []byte(yamlContent), 0644))

	report, err := RunDuplicateCheck(dir)
	require.NoError(t, err)
	assert.Equal(t, 1, len(report.NameAuthorDuplicates))
}

func TestRunDuplicateCheck_InvalidYAML(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "bad.yml"), []byte("invalid: [yaml:\n"), 0644))

	// Should not error, just skip invalid files
	report, err := RunDuplicateCheck(dir)
	require.NoError(t, err)
	assert.NotNil(t, report)
}

func TestFormatDuplicateEntries_GhOnlyEmpty(t *testing.T) {
	// ghOnly=true with entries that have empty File
	entries := []ItemBrief{
		{File: "dev/go.yml: language (repo)"},
	}
	result := formatDuplicateEntries(entries, true)
	assert.Contains(t, result, "dev/go.yml: language (repo)")
}

func TestFormatDuplicateEntries_Empty(t *testing.T) {
	result := formatDuplicateEntries(nil, false)
	assert.Equal(t, "", result)
}

func TestDuplicateReport_IssuesNilReport(t *testing.T) {
	var r *DuplicateReport
	issues := r.issues(false)
	assert.Nil(t, issues)
}

func TestDuplicateReport_IssuesGhOnly(t *testing.T) {
	r := &DuplicateReport{
		URLDuplicates: []URLDupEntry{
			{URL: "https://example.com", Entries: []ItemBrief{{File: "a.yml"}}},
		},
		NameAuthorDuplicates: []NameAuthorDupEntry{
			{Key: "name|author", Entries: []ItemBrief{{File: "b.yml"}}},
		},
	}
	// ghOnly=true should skip NameAuthorDuplicates
	issues := r.issues(true)
	assert.Len(t, issues, 1)
	assert.Contains(t, issues[0].Message, "重复 URL")
}

func TestCollectGhRepoEntries_SkipsHiddenDirs(t *testing.T) {
	dir := t.TempDir()
	hiddenDir := filepath.Join(dir, ".hidden")
	require.NoError(t, os.MkdirAll(hiddenDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(hiddenDir, "go.yml"), []byte(`- type: lang`), 0644))

	entries, err := collectGhRepoEntries(dir)
	require.NoError(t, err)
	assert.Empty(t, entries)
}

func TestCollectGhRepoEntries_SkipsFiles(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "readme.txt"), []byte("text"), 0644))

	entries, err := collectGhRepoEntries(dir)
	require.NoError(t, err)
	assert.Empty(t, entries)
}

func TestCollectGhRepoEntries_InvalidYAMLInSubdir(t *testing.T) {
	dir := t.TempDir()
	subDir := filepath.Join(dir, "dev")
	require.NoError(t, os.MkdirAll(subDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(subDir, "bad.yml"), []byte("invalid: [yaml:\n"), 0644))

	entries, err := collectGhRepoEntries(dir)
	require.NoError(t, err)
	assert.Empty(t, entries) // parse errors are silently skipped
}

func TestParseGhYAMLEntries_EmptyType(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "test.yml"), []byte(`- type: ""
  repo:
    - url: https://github.com/a/b
`), 0644))

	entries, err := parseGhYAMLEntries(filepath.Join(dir, "test.yml"), dir)
	require.NoError(t, err)
	assert.Empty(t, entries) // empty type is skipped
}

func TestParseGhYAMLEntries_NoType(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "test.yml"), []byte(`- repo:
    - url: https://github.com/a/b
`), 0644))

	entries, err := parseGhYAMLEntries(filepath.Join(dir, "test.yml"), dir)
	require.NoError(t, err)
	assert.Empty(t, entries)
}

func TestParseGhYAMLEntries_InvalidYAML(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "bad.yml"), []byte("invalid: [yaml:\n"), 0644))

	_, err := parseGhYAMLEntries(filepath.Join(dir, "bad.yml"), dir)
	require.Error(t, err)
}

func TestParseYAMLDocItems_EmptyName(t *testing.T) {
	doc := []map[string]any{
		{"name": "", "author": "A"},
		{"author": "B"}, // no name key
	}
	items := parseYAMLDocItems(doc, "test.yml")
	assert.Empty(t, items)
}

func TestRunGHDuplicateCheck_SubDirReadError(t *testing.T) {
	// Non-existent dir should error
	_, err := RunGHDuplicateCheck("/tmp/nonexistent-gh-dup-check-dir-99999")
	require.Error(t, err)
}

func TestRunDuplicateCheck_NameAuthorOnly(t *testing.T) {
	// Name+author duplicates with no URL overlap
	dir := t.TempDir()
	yamlContent := `
- name: "Same"
  author: "Author"
  url: "https://example.com/a"
- name: "Same"
  author: "Author"
  url: "https://example.com/b"
- name: "Same"
  author: "Author"
  url: "https://example.com/c"
`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "books.yml"), []byte(yamlContent), 0644))

	report, err := RunDuplicateCheck(dir)
	require.NoError(t, err)
	assert.Equal(t, 0, len(report.URLDuplicates))
	assert.Equal(t, 1, len(report.NameAuthorDuplicates))
}
