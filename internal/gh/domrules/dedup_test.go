package domrules

import (
	"os"
	"path/filepath"
	"strings"
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
	assert.Empty(t, report.URLDuplicates)
	assert.Empty(t, report.NameAuthorDuplicates)
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
	assert.Len(t, report.URLDuplicates, 1, "should find one URL duplicate")
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
	assert.Len(t, report.NameAuthorDuplicates, 1, "should find one name+author duplicate")
}

func TestRunDuplicateCheck_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	report, err := RunDuplicateCheck(dir)
	require.NoError(t, err)
	require.NotNil(t, report)
	assert.Empty(t, report.URLDuplicates)
}

func TestRunGHDuplicateCheck_NoDuplicates(t *testing.T) {
	dir := t.TempDir()
	tagDir := filepath.Join(dir, "dev")
	require.NoError(t, os.MkdirAll(tagDir, 0755))
	yamlContent := `
- topic: lang
  repo:
    - url: "https://github.com/owner/repo-a"
    - url: "https://github.com/owner/repo-b"
`
	require.NoError(t, os.WriteFile(filepath.Join(tagDir, "go.yml"), []byte(yamlContent), 0644))

	report, err := RunGHDuplicateCheck(dir)
	require.NoError(t, err)
	require.NotNil(t, report)
	assert.Empty(t, report.URLDuplicates)
}

func TestRunGHDuplicateCheck_WithDuplicates(t *testing.T) {
	dir := t.TempDir()
	tagDir := filepath.Join(dir, "dev")
	require.NoError(t, os.MkdirAll(tagDir, 0755))
	yamlContent := `
- topic: lang
  kind: type
  repo:
    - url: "https://github.com/owner/repo"
    - url: "https://github.com/owner/repo"
`
	require.NoError(t, os.WriteFile(filepath.Join(tagDir, "go.yml"), []byte(yamlContent), 0644))

	report, err := RunGHDuplicateCheck(dir)
	require.NoError(t, err)
	require.NotNil(t, report)
	assert.Len(t, report.URLDuplicates, 1, "should find duplicate URL")
}

func TestRunGHDuplicateCheck_TrailingSlashVariant(t *testing.T) {
	// Same repo spelled with and without a trailing slash must be flagged as a
	// duplicate — the original gh-alfred bug that surfaced tmc/langchaingo twice.
	dir := t.TempDir()
	tagDir := filepath.Join(dir, "AI")
	require.NoError(t, os.MkdirAll(tagDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(tagDir, "agent.yml"), []byte(`
- topic: agent-fwk
  repo:
    - url: https://github.com/tmc/langchaingo/
- topic: agent-infra
  repo:
    - url: https://github.com/tmc/langchaingo
`), 0644))

	report, err := RunGHDuplicateCheck(dir)
	require.NoError(t, err)
	require.NotNil(t, report)
	require.Len(t, report.URLDuplicates, 1, "trailing-slash variant should be one duplicate")
	assert.Equal(t, "https://github.com/tmc/langchaingo/", report.URLDuplicates[0].URL)
	assert.Len(t, report.URLDuplicates[0].Entries, 2)
}

func TestRunGHDuplicateCheck_CaseFoldedOwnerRepo(t *testing.T) {
	// Owner/name differing only in case must still be recognized as the same repo.
	dir := t.TempDir()
	tagDir := filepath.Join(dir, "dev")
	require.NoError(t, os.MkdirAll(tagDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(tagDir, "go.yml"), []byte(`
- topic: llm
  kind: type
  repo:
    - url: https://github.com/BerriAI/litellm
    - url: https://github.com/berriai/LiteLLM/
`), 0644))

	report, err := RunGHDuplicateCheck(dir)
	require.NoError(t, err)
	require.NotNil(t, report)
	require.Len(t, report.URLDuplicates, 1)
	assert.Len(t, report.URLDuplicates[0].Entries, 2)
}

func TestGhRepoURLKey(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{name: "github no slash", raw: "https://github.com/tmc/langchaingo", want: "tmc/langchaingo"},
		{name: "github trailing slash", raw: "https://github.com/tmc/langchaingo/", want: "tmc/langchaingo"},
		{name: "github case fold", raw: "HTTPS://GITHUB.COM/BerriAI/LiteLLM/", want: "berriai/litellm"},
		{name: "non-github trims slash", raw: "https://example.com/a/b/", want: "https://example.com/a/b"},
		{name: "non-github untouched", raw: "https://example.com/a/b", want: "https://example.com/a/b"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, ghRepoURLKey(tc.raw))
		})
	}
}

func TestRunGHDuplicateCheck_TopicReposAcrossFiles(t *testing.T) {
	// The same topic.repo repeated across two files must be flagged. Repos live
	// on topics now; there is no type-level repo list.
	dir := t.TempDir()
	tagDir := filepath.Join(dir, "AI")
	require.NoError(t, os.MkdirAll(tagDir, 0755))

	require.NoError(t, os.WriteFile(filepath.Join(tagDir, "LLM.yml"), []byte(`
- topic: llm-eval
  kind: mech
  repo:
    - url: https://github.com/BerriAI/litellm
`), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(tagDir, "agent.yml"), []byte(`
- topic: agent-deploy
  kind: mech
  repo:
    - url: https://github.com/BerriAI/litellm
`), 0644))

	report, err := RunGHDuplicateCheck(dir)
	require.NoError(t, err)
	require.Len(t, report.URLDuplicates, 1)
	assert.Equal(t, "https://github.com/BerriAI/litellm", report.URLDuplicates[0].URL)
	assert.Len(t, report.URLDuplicates[0].Entries, 2)

	locs := []string{
		report.URLDuplicates[0].Entries[0].File,
		report.URLDuplicates[0].Entries[1].File,
	}
	assert.True(t, strings.Contains(locs[0], "LLM.yml") || strings.Contains(locs[1], "LLM.yml"))
	assert.True(t, strings.Contains(locs[0], "agent.yml") || strings.Contains(locs[1], "agent.yml"))
	assert.True(t, strings.Contains(locs[0], "topic:agent-deploy") || strings.Contains(locs[1], "topic:agent-deploy"))
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
	assert.Len(t, report.URLDuplicates, 1)
	// Name+author duplicates filtered because they were caught by URL
	assert.Empty(t, report.NameAuthorDuplicates)
}

func TestRunGHDuplicateCheck_EmptyDir(t *testing.T) {
	// A gh dir with no data is a failure (no repos to scan), not a silent pass.
	dir := t.TempDir()
	_, err := RunGHDuplicateCheck(dir)
	require.Error(t, err)
}

func TestRunGHDuplicateCheck_RepoEntry(t *testing.T) {
	dir := t.TempDir()
	tagDir := filepath.Join(dir, "dev")
	require.NoError(t, os.MkdirAll(tagDir, 0755))
	yamlContent := `
- topic: lang
  kind: type
  repo:
    - url: "https://github.com/owner/repo"
    - url: "https://github.com/owner/repo"
`
	require.NoError(t, os.WriteFile(filepath.Join(tagDir, "go.yml"), []byte(yamlContent), 0644))

	report, err := RunGHDuplicateCheck(dir)
	require.NoError(t, err)
	assert.Len(t, report.URLDuplicates, 1)
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
	assert.Len(t, report.NameAuthorDuplicates, 1)
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
	assert.Empty(t, result)
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

func TestRunGHDuplicateCheck_RegressionPytorchTriple(t *testing.T) {
	// Regression for the original bug: data/gh files use a flat "- topic:"
	// layout, and pytorch/pytorch was listed 3x in one file yet dedup reported
	// "passed". Now the loader is the same as index/render, so the triple must
	// surface as a single duplicate with 3 locations.
	dir := t.TempDir()
	tagDir := filepath.Join(dir, "AI")
	require.NoError(t, os.MkdirAll(tagDir, 0755))
	yamlContent := `- topic: llm-train
  kind: mech
  repo:
    - url: https://github.com/huggingface/transformers
    - url: https://github.com/pytorch/pytorch
- topic: llm-arch
  kind: mech
  repo:
    - url: https://github.com/pytorch/pytorch
- topic: llm-serve
  kind: mech
  repo:
    - url: https://github.com/pytorch/pytorch
`
	require.NoError(t, os.WriteFile(filepath.Join(tagDir, "LLM-res.yml"), []byte(yamlContent), 0644))

	report, err := RunGHDuplicateCheck(dir)
	require.NoError(t, err)
	require.NotNil(t, report)
	require.Len(t, report.URLDuplicates, 1, "pytorch listed 3x must be flagged")
	dup := report.URLDuplicates[0]
	assert.Equal(t, "https://github.com/pytorch/pytorch", dup.URL)
	require.Len(t, dup.Entries, 3)
	for _, e := range dup.Entries {
		assert.Contains(t, e.File, "LLM-res.yml", "each duplicate should point at its source file")
	}
}

func TestRunGHDuplicateCheck_InvalidYAMLFailsLoud(t *testing.T) {
	// Unlike the old bespoke collector (which silently skipped parse errors),
	// the shared loader errors on a broken file: broken data must not go
	// undetected, because it means the whole repo set may be missing repos.
	dir := t.TempDir()
	subDir := filepath.Join(dir, "dev")
	require.NoError(t, os.MkdirAll(subDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(subDir, "bad.yml"), []byte("invalid: [yaml:\n"), 0644))

	_, err := RunGHDuplicateCheck(dir)
	require.Error(t, err)
}

func TestParseYAMLDocItems_EmptyName(t *testing.T) {
	doc := []yamlItem{
		{Author: "A"},
		{Author: "B"},
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
	assert.Empty(t, report.URLDuplicates)
	assert.Len(t, report.NameAuthorDuplicates, 1)
}
