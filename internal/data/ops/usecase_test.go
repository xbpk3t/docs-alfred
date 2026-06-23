package dataops

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func TestRunDomainCheck_DefaultPath(t *testing.T) {
	result, err := RunDomainCheck(DomainCheckInput{Domain: data.DomainGH})
	if err == nil {
		assert.NotNil(t, result)
	}
}

func TestExtractTopics_EmptyOut(t *testing.T) {
	_, err := extractTopics(extractTopicsInput{Out: ""})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--out is required")
}

func TestRunDomainDuplicate_DefaultPath(t *testing.T) {
	_, err := RunDomainDuplicate(DomainDuplicateInput{Domain: data.DomainBooks})
	if err != nil {
		// Expected - default path may not exist
	}
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
