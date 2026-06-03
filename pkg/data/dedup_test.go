package data

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
