package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	data "github.com/xbpk3t/docs-alfred/internal/gh/domrules"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// writeGhFiles creates a temp directory and writes gh-format YAML files into it.
// Keys in the map are relative file paths (e.g. "tool.yml" or "data/gh/tool.yml").
// Returns the temp directory root.
func writeGhFiles(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, content := range files {
		p := filepath.Join(dir, name)
		require.NoError(t, os.MkdirAll(filepath.Dir(p), 0o755))
		require.NoError(t, os.WriteFile(p, []byte(content), 0o644))
	}
	return dir
}

// validGhYAML is minimal gh-format YAML that passes check validation.
const validGhYAML = `- type: tool
  repo:
    - url: https://github.com/acme/tool
      des: a tool
  record: []
`

// invalidDateGhYAML has a repo-level record with a malformed date.
// Section-level records are not date-validated; repo-level records are.
const invalidDateGhYAML = `- type: tool
  repo:
    - url: https://github.com/acme/tool
      des: a tool
      record:
        - date: not-a-date
          des: invalid date
  record: []
`

// ---------------------------------------------------------------------------
// Execute
// ---------------------------------------------------------------------------

func TestExecuteDoesNotPanic(t *testing.T) {
	require.NotPanics(t, func() {
		_ = Execute()
	})
}

// ---------------------------------------------------------------------------
// newRenderCmd – RunE paths
// ---------------------------------------------------------------------------

func TestNewRenderCmdRequiresDomain(t *testing.T) {
	cmd := newRootCmd()
	cmd.SetArgs([]string{"render"})
	err := cmd.Execute()
	require.Error(t, err)
	require.Contains(t, err.Error(), "accepts 1 arg(s)")
}

func TestNewRenderCmdInvalidDomain(t *testing.T) {
	cmd := newRootCmd()
	cmd.SetArgs([]string{"render", "bogus"})
	err := cmd.Execute()
	require.Error(t, err)
	require.Contains(t, err.Error(), "unknown data domain")
}

func TestNewRenderCmdDefaultPath(t *testing.T) {
	cmd := newRootCmd()
	cmd.SetArgs([]string{"render", "gh"})
	err := cmd.Execute()
	// Fails because data/gh doesn't exist in test env, but validates arg parsing.
	require.Error(t, err)
}

func TestNewRenderCmdWithCustomPath(t *testing.T) {
	ghDir := writeGhFiles(t, map[string]string{"dev/tool.yml": validGhYAML})
	outDir := t.TempDir()

	cmd := newRootCmd()
	cmd.SetArgs([]string{"render", "gh", "--path", ghDir, "--out-dir", outDir, "--format", "yaml"})
	err := cmd.Execute()
	require.NoError(t, err)
}

func TestNewRenderCmdWithJSONFormat(t *testing.T) {
	ghDir := writeGhFiles(t, map[string]string{"dev/tool.yml": validGhYAML})
	outDir := t.TempDir()

	cmd := newRootCmd()
	cmd.SetArgs([]string{"render", "gh", "--path", ghDir, "--out-dir", outDir, "--format", "json"})
	err := cmd.Execute()
	require.NoError(t, err)
}

func TestNewRenderCmdGoodsDomain(t *testing.T) {
	goodsDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(goodsDir, "goods.yml"), []byte(`---
- type: 耳机
  tag: EDC
  score: 3
  item:
    - name: C50
      price: ¥179
`), 0644))
	outDir := t.TempDir()

	cmd := newRootCmd()
	cmd.SetArgs([]string{"render", "goods", "--path", goodsDir, "--out-dir", outDir})
	err := cmd.Execute()
	require.NoError(t, err)
}

// ---------------------------------------------------------------------------
// newCheckCmd – RunE paths
// ---------------------------------------------------------------------------

func TestNewCheckCmdRunEInvalidDomain(t *testing.T) {
	cmd := newRootCmd()
	cmd.SetArgs([]string{"check", "bogus"})
	err := cmd.Execute()
	require.Error(t, err)
	require.Contains(t, err.Error(), "unknown data domain")
}

func TestNewCheckCmdRunEGhValidData(t *testing.T) {
	ghDir := writeGhFiles(t, map[string]string{"tool.yml": validGhYAML})
	cmd := newRootCmd()
	cmd.SetArgs([]string{"check", "gh", "--path", ghDir})
	_ = cmd.Execute()
}

func TestNewCheckCmdRunEGhInvalidData(t *testing.T) {
	ghDir := writeGhFiles(t, map[string]string{"tool.yml": invalidDateGhYAML})
	cmd := newRootCmd()
	cmd.SetArgs([]string{"check", "gh", "--path", ghDir})
	_ = cmd.Execute()
}

func TestNewCheckCmdRunEGhNonexistentPath(t *testing.T) {
	cmd := newRootCmd()
	cmd.SetArgs([]string{"check", "gh", "--path", "/tmp/__no_such_gh_dir__"})
	err := cmd.Execute()
	require.Error(t, err)
}

func TestNewCheckCmdRunEWithMaxLines(t *testing.T) {
	ghDir := writeGhFiles(t, map[string]string{"tool.yml": validGhYAML})
	cmd := newRootCmd()
	cmd.SetArgs([]string{"check", "gh", "--path", ghDir, "--max-lines", "500"})
	_ = cmd.Execute()
}

// ---------------------------------------------------------------------------
// runDomainCheck – direct tests
// ---------------------------------------------------------------------------

func TestRunDomainCheckGhValidData(t *testing.T) {
	ghDir := writeGhFiles(t, map[string]string{"tool.yml": validGhYAML})
	err := runDomainCheck(data.DomainGH, ghDir, "", 0)
	_ = err
}

func TestRunDomainCheckGhInvalidDate(t *testing.T) {
	ghDir := writeGhFiles(t, map[string]string{"tool.yml": invalidDateGhYAML})
	err := runDomainCheck(data.DomainGH, ghDir, "", 0)
	_ = err
}

func TestRunDomainCheckGhNonexistentPath(t *testing.T) {
	err := runDomainCheck(data.DomainGH, "/tmp/__gh_no_such__", "", 0)
	require.Error(t, err)
}

func TestRunDomainCheckGhWithMaxLines(t *testing.T) {
	ghDir := writeGhFiles(t, map[string]string{"tool.yml": validGhYAML})
	err := runDomainCheck(data.DomainGH, ghDir, "", 100)
	_ = err
}

func TestRunDomainCheckGhWithRuleScope(t *testing.T) {
	ghDir := writeGhFiles(t, map[string]string{"tool.yml": validGhYAML})
	err := runDomainCheck(data.DomainGH, ghDir, "auto", 0)
	_ = err
}

// ---------------------------------------------------------------------------
// newDuplicateCmd – RunE paths
// ---------------------------------------------------------------------------

func TestNewDuplicateCmdRunEInvalidDomain(t *testing.T) {
	cmd := newRootCmd()
	cmd.SetArgs([]string{"duplicate", "bogus"})
	err := cmd.Execute()
	require.Error(t, err)
	require.Contains(t, err.Error(), "unknown data domain")
}

func TestNewDuplicateCmdRunEGhNoDuplicates(t *testing.T) {
	ghDir := writeGhFiles(t, map[string]string{"tool.yml": validGhYAML})
	cmd := newRootCmd()
	cmd.SetArgs([]string{"duplicate", "gh", "--path", ghDir})
	_ = cmd.Execute()
}

func TestNewDuplicateCmdRunEGhWithDuplicates(t *testing.T) {
	// GH duplicate check expects YAML files inside subdirectories of the target dir.
	ghDir := writeGhFiles(t, map[string]string{
		"dev/a.yml": "- type: a\n  repo:\n    - url: https://github.com/acme/same\n      des: first\n  record: []\n",
		"ops/b.yml": "- type: b\n  repo:\n    - url: https://github.com/acme/same\n      des: second\n  record: []\n",
	})
	cmd := newRootCmd()
	cmd.SetArgs([]string{"duplicate", "gh", "--path", ghDir})
	_ = cmd.Execute()
}

// ---------------------------------------------------------------------------
// runDomainDuplicate – direct tests
// ---------------------------------------------------------------------------

func TestRunDomainDuplicateGhNoDuplicates(t *testing.T) {
	ghDir := writeGhFiles(t, map[string]string{"tool.yml": validGhYAML})
	err := runDomainDuplicate(data.DomainGH, ghDir)
	_ = err
}

func TestRunDomainDuplicateGhNonexistentPath(t *testing.T) {
	err := runDomainDuplicate(data.DomainGH, "/tmp/__dup_no_such__")
	require.Error(t, err)
}

func TestRunDomainDuplicateGhWithDuplicates(t *testing.T) {
	// GH duplicate check expects YAML files inside subdirectories.
	ghDir := writeGhFiles(t, map[string]string{
		"dev/a.yml": "- type: a\n  repo:\n    - url: https://github.com/acme/same\n      des: first\n  record: []\n",
		"ops/b.yml": "- type: b\n  repo:\n    - url: https://github.com/acme/same\n      des: second\n  record: []\n",
	})
	err := runDomainDuplicate(data.DomainGH, ghDir)
	_ = err
}
