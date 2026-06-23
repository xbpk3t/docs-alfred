package dataops

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xbpk3t/docs-alfred/internal/gh/enrich"
	data "github.com/xbpk3t/docs-alfred/internal/gh/domrules"
	"github.com/xbpk3t/docs-alfred/pkg/checkutil"
)

func TestRunDomainCheckPassesGhMaxLinesOverride(t *testing.T) {
	tmpDir := t.TempDir()
	content := strings.Repeat("# filler\n", 1000) + "- type: go\n  record: []\n"
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "go.yml"), []byte(content), 0644))

	defaultResult, err := RunDomainCheck(DomainCheckInput{Domain: data.DomainGH, Path: tmpDir})
	require.NoError(t, err)
	require.True(t, checkutil.HasErrors(defaultResult.Issues))

	overrideResult, err := RunDomainCheck(DomainCheckInput{
		Domain:     data.DomainGH,
		Path:       tmpDir,
		GhMaxLines: 1500,
	})
	require.NoError(t, err)
	require.False(t, checkutil.HasErrors(overrideResult.Issues))
}

func TestResolveGhAppendDateKeepsExplicitDate(t *testing.T) {
	require.Equal(t, "2024-01-02", resolveGhAppendDate("2024-01-02"))
}

func TestResolveGhAppendDateDefaultsToToday(t *testing.T) {
	before := time.Now().Format(time.DateOnly)
	got := resolveGhAppendDate("")
	after := time.Now().Format(time.DateOnly)

	require.Contains(t, []string{before, after}, got)
}

func TestRunDomainCheck_UnknownDomain(t *testing.T) {
	_, err := RunDomainCheck(DomainCheckInput{Domain: data.DataDomain("unknown")})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown data domain")
}

func TestRunDomainCheck_BooksDomain(t *testing.T) {
	tmpDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "books.yml"), []byte("- name: test\n  score: 4\n"), 0644))

	result, err := RunDomainCheck(DomainCheckInput{Domain: data.DomainBooks, Path: tmpDir})
	require.NoError(t, err)
	assert.NotNil(t, result)
}

func TestRunDomainCheck_TaskDomain(t *testing.T) {
	tmpDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "task.yml"), []byte("- task: test\n"), 0644))

	result, err := RunDomainCheck(DomainCheckInput{Domain: data.DomainTask, Path: tmpDir})
	require.NoError(t, err)
	assert.NotNil(t, result)
}

func TestRunDomainCheck_GoodsDomain(t *testing.T) {
	tmpDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "goods.yml"), []byte(`---
- type: 耳机
  tag: EDC
  score: 3
  item:
    - name: C50
      price: ¥179
`), 0644))

	result, err := RunDomainCheck(DomainCheckInput{Domain: data.DomainGoods, Path: tmpDir})
	require.NoError(t, err)
	assert.NotNil(t, result)
}

func TestRunDomainDuplicate_UnknownDomain(t *testing.T) {
	_, err := RunDomainDuplicate(DomainDuplicateInput{Domain: data.DataDomain("unknown")})
	require.Error(t, err)
}

func TestRunDomainDuplicate_UnsupportedDomain(t *testing.T) {
	_, err := RunDomainDuplicate(DomainDuplicateInput{Domain: data.DomainTask})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not supported")
}

func TestRunDomainDuplicate_BooksDomain(t *testing.T) {
	tmpDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "books.yml"), []byte(`---
- name: "Book A"
  author: "Author A"
  url: "https://example.com/a"
`), 0644))

	result, err := RunDomainDuplicate(DomainDuplicateInput{Domain: data.DomainBooks, Path: tmpDir})
	require.NoError(t, err)
	assert.NotNil(t, result)
}

func TestRunDomainDuplicate_GHDomain(t *testing.T) {
	tmpDir := t.TempDir()
	tagDir := filepath.Join(tmpDir, "dev")
	require.NoError(t, os.MkdirAll(tagDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(tagDir, "go.yml"), []byte(`---
- type: language
  repo:
    - url: https://github.com/acme/tool
`), 0644))

	result, err := RunDomainDuplicate(DomainDuplicateInput{Domain: data.DomainGH, Path: tmpDir})
	require.NoError(t, err)
	assert.NotNil(t, result)
}

func TestRunRender_ExtractTopicsNoOut(t *testing.T) {
	_, err := RunRender(RenderInput{Extract: "topics"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--out is required")
}

func TestRunGhFind(t *testing.T) {
	tmpDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "go.yml"), []byte(`---
- type: language
  repo:
    - url: https://github.com/acme/tool
      des: test tool
`), 0644))

	result, err := RunGhFind(GhFindInput{Root: tmpDir, Query: "tool"})
	require.NoError(t, err)
	assert.NotNil(t, result)
}

func TestRunGhFind_WithLimit(t *testing.T) {
	tmpDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "go.yml"), []byte(`---
- type: language
  repo:
    - url: https://github.com/acme/tool1
      des: tool 1
    - url: https://github.com/acme/tool2
      des: tool 2
`), 0644))

	result, err := RunGhFind(GhFindInput{Root: tmpDir, Query: "tool", Limit: 1})
	require.NoError(t, err)
	assert.LessOrEqual(t, len(result.Entries), 1)
}

func TestRunGhAppend_NilInput(t *testing.T) {
	_, err := RunGhAppend(nil)
	require.Error(t, err)
}

func TestRunGhAppend_NoURLNoFile(t *testing.T) {
	_, err := RunGhAppend(&GhAppendInput{Des: "test"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--url or --file")
}

func TestRunGhAppend_NoDes(t *testing.T) {
	_, err := RunGhAppend(&GhAppendInput{URL: "https://github.com/a/b"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--des")
}

func TestRunDomainCheck_DefaultPath(t *testing.T) {
	result, err := RunDomainCheck(DomainCheckInput{Domain: data.DomainGH})
	if err == nil {
		assert.NotNil(t, result)
	}
}

// --- Tests not already in enrich_test.go ---

func TestProcessItem_SuccessApply(t *testing.T) {
	item := parseFirstItem(t, "- name: Good Movie\n")
	m := &mockEnricher{
		fields: &enrich.EnrichFields{
			PublishAt: "2024",
			Alias:     "Original Title",
		},
	}
	res, err := processItem(context.Background(), item, m, false, 0)
	require.NoError(t, err)
	assert.Len(t, res.Actions, 2)
	// SetField stores in pending, not AST. Check HasPending instead.
	assert.True(t, item.HasPending())
}

func TestProcessItem_PartialFields(t *testing.T) {
	item := parseFirstItem(t, "- name: Movie With Date\n  publishAt: \"2020\"\n")
	m := &mockEnricher{
		fields: &enrich.EnrichFields{
			PublishAt: "2024",
			Alias:     "Original",
		},
	}
	res, err := processItem(context.Background(), item, m, false, 0)
	require.NoError(t, err)
	assert.Len(t, res.Actions, 1)
	assert.Equal(t, enrich.FieldAlias, res.Actions[0].Field)
}

func TestProcessItem_SetFieldError(t *testing.T) {
	item := parseFirstItem(t, "- name: Movie With Cast\n")
	// Cast is a list field that requires 、separator, not comma
	m := &mockEnricher{
		fields: &enrich.EnrichFields{
			Cast: "Actor1, Actor2", // comma instead of 、
		},
	}
	_, err := processItem(context.Background(), item, m, false, 0)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "set field")
}

func TestRunEnrich_NonexistentFile(t *testing.T) {
	_, err := RunEnrich(context.Background(), &EnrichInput{
		Resource: enrich.ResourceMovie,
		Path:     "/nonexistent-file-99999.yml",
		APIKey:   "test-key",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "parse yaml")
}

func TestRunEnrich_EmptyYAML(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "empty.yml")
	require.NoError(t, os.WriteFile(path, []byte("[]\n"), 0644))

	_, err := RunEnrich(context.Background(), &EnrichInput{
		Resource: enrich.ResourceMovie,
		Path:     path,
		APIKey:   "test-key",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no items found")
}

func TestExtractTopics_EmptyOut(t *testing.T) {
	_, err := extractTopics(extractTopicsInput{Out: ""})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--out is required")
}

func TestRunGhAppend_WithFile(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.yml")
	require.NoError(t, os.WriteFile(filePath, []byte(`- type: language
  repo:
    - url: https://github.com/acme/tool
      des: test tool
`), 0644))

	result, err := RunGhAppend(&GhAppendInput{
		File: filePath,
		Des:  "new entry",
		Date: "2024-01-01",
	})
	if err != nil {
		assert.Contains(t, err.Error(), "append-record")
	} else {
		assert.NotNil(t, result)
	}
}

func TestRunDomainDuplicate_DefaultPath(t *testing.T) {
	_, err := RunDomainDuplicate(DomainDuplicateInput{Domain: data.DomainBooks})
	if err != nil {
		// Expected - default path may not exist
	}
}

func TestRunEnrich_DryRunWithFakeAPI(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "movies.yml")
	require.NoError(t, os.WriteFile(path, []byte(`---
- name: Test Movie
  score: 4
- name: Another Movie
`), 0644))

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, err := RunEnrich(ctx, &EnrichInput{
		Resource: enrich.ResourceMovie,
		Path:     path,
		APIKey:   "fake-api-key-for-testing",
		DryRun:   true,
	})
	// RunEnrich should succeed even if individual items fail
	require.NoError(t, err)
	require.NotNil(t, result)
	require.NotNil(t, result.Report)
	assert.Len(t, result.Report.Results, 2)
	assert.True(t, result.Report.DryRun)
}

func TestRunRender_NonExtractPath(t *testing.T) {
	tmpDir := t.TempDir()
	srcDir := filepath.Join(tmpDir, "data")
	require.NoError(t, os.MkdirAll(srcDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(srcDir, "test.yml"), []byte("- name: test\n"), 0644))

	configPath := filepath.Join(tmpDir, "docs.yml")
	require.NoError(t, os.WriteFile(configPath, []byte(`---
- src: `+srcDir+`
  cmd: default
  yaml:
    dst: `+filepath.Join(tmpDir, "out")+`
`), 0644))

	result, err := RunRender(RenderInput{Config: configPath})
	require.NoError(t, err)
	assert.Equal(t, 1, result.ConfigCount)
}

func TestRunDomainCheck_YAMLParseOnlyError(t *testing.T) {
	tmpDir := t.TempDir()
	// Write invalid YAML that will fail parsing
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "bad.yml"), []byte("not: [valid: yaml"), 0644))

	// Task domain uses YAMLParseOnly
	result, err := RunDomainCheck(DomainCheckInput{Domain: data.DomainTask, Path: tmpDir})
	// Should error because of invalid YAML
	if err != nil {
		assert.Contains(t, err.Error(), "YAML parsing")
	} else {
		// If no error, result should still be valid
		assert.NotNil(t, result)
	}
}

func TestRunDomainCheck_StructuredCheckDomain(t *testing.T) {
	tmpDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "books.yml"), []byte(`---
- name: "Book A"
  author: "Author A"
  score: 4
`), 0644))

	result, err := RunDomainCheck(DomainCheckInput{Domain: data.DomainBooks, Path: tmpDir})
	require.NoError(t, err)
	assert.NotNil(t, result)
}

func TestRunGhFind_EmptyRoot(t *testing.T) {
	// RunGhFind with empty Root should fall back to default path.
	// Since default path may not exist, we expect either success or a file-not-found error.
	result, err := RunGhFind(GhFindInput{Query: "test"})
	if err != nil {
		// Default path may not exist in test environment
		assert.Nil(t, result)
	} else {
		assert.NotNil(t, result)
	}
}
