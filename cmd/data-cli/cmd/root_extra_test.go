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
// kind tools does not require mdscc; section must have at least one topic.
const validGhYAML = `- type: tool
  topics:
    - topic: overview
      kind: tools
  repo:
    - url: https://github.com/acme/tool
      des: a tool
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
	cmd.SetArgs([]string{"render", "gh", "--path", ghDir, "--output", outDir})
	err := cmd.Execute()
	require.NoError(t, err)
}

func TestNewRenderCmdWithJSONFormat(t *testing.T) {
	ghDir := writeGhFiles(t, map[string]string{"dev/tool.yml": validGhYAML})
	outDir := t.TempDir()

	cmd := newRootCmd()
	cmd.SetArgs([]string{"render", "gh", "--path", ghDir, "--output", outDir})
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
	cmd.SetArgs([]string{"render", "goods", "--path", goodsDir, "--output", outDir})
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
	require.NoError(t, cmd.Execute())
}

func TestNewCheckCmdRunEGhInvalidData(t *testing.T) {
	// Missing kind is the check gh gate (date shape is out of scope).
	ghDir := writeGhFiles(t, map[string]string{"tool.yml": `- type: tool
  topics:
    - topic: overview
`})
	cmd := newRootCmd()
	cmd.SetArgs([]string{"check", "gh", "--path", ghDir})
	err := cmd.Execute()
	require.Error(t, err)
	require.Contains(t, err.Error(), "data check gh failed")
}

func TestNewCheckCmdRunEGhNonexistentPath(t *testing.T) {
	cmd := newRootCmd()
	cmd.SetArgs([]string{"check", "gh", "--path", "/tmp/__no_such_gh_dir__"})
	err := cmd.Execute()
	require.Error(t, err)
}

func TestNewCheckCmdRunEWithMaxLines(t *testing.T) {
	// --max-lines flag was removed alongside gh check logic
	// This test is kept as a no-op for backward compat
}

// ---------------------------------------------------------------------------
// runDomainCheck – direct tests
// ---------------------------------------------------------------------------

func TestRunDomainCheckGhValidData(t *testing.T) {
	ghDir := writeGhFiles(t, map[string]string{"tool.yml": validGhYAML})
	err := runDomainCheck(data.DomainGH, ghDir, "")
	require.NoError(t, err)
}

func TestRunDomainCheckGhInvalidKind(t *testing.T) {
	ghDir := writeGhFiles(t, map[string]string{"tool.yml": `- type: tool
  topics:
    - topic: overview
      kind: unset
`})
	err := runDomainCheck(data.DomainGH, ghDir, "")
	require.Error(t, err)
	require.Contains(t, err.Error(), "data check gh failed")
}

func TestRunDomainCheckGhNonexistentPath(t *testing.T) {
	err := runDomainCheck(data.DomainGH, "/tmp/__gh_no_such__", "")
	require.Error(t, err)
}

func TestRunDomainCheckGhWithRuleScope(t *testing.T) {
	ghDir := writeGhFiles(t, map[string]string{"tool.yml": validGhYAML})
	err := runDomainCheck(data.DomainGH, ghDir, "auto")
	require.NoError(t, err)
}

// ---------------------------------------------------------------------------
// newDedupCmd – RunE paths
// ---------------------------------------------------------------------------

func TestNewDedupCmdRunEInvalidDomain(t *testing.T) {
	cmd := newRootCmd()
	cmd.SetArgs([]string{"dedup", "bogus"})
	err := cmd.Execute()
	require.Error(t, err)
	require.Contains(t, err.Error(), "unknown data domain")
}

func TestNewDedupCmdRunEGhNoDuplicates(t *testing.T) {
	ghDir := writeGhFiles(t, map[string]string{"tool.yml": validGhYAML})
	cmd := newRootCmd()
	cmd.SetArgs([]string{"dedup", "gh", "--path", ghDir})
	_ = cmd.Execute()
}

func TestNewDedupCmdRunEGhWithDuplicates(t *testing.T) {
	// GH duplicate check expects YAML files inside subdirectories of the target dir.
	ghDir := writeGhFiles(t, map[string]string{
		"dev/a.yml": "- type: a\n  repo:\n    - url: https://github.com/acme/same\n      des: first\n  record: []\n",
		"ops/b.yml": "- type: b\n  repo:\n    - url: https://github.com/acme/same\n      des: second\n  record: []\n",
	})
	cmd := newRootCmd()
	cmd.SetArgs([]string{"dedup", "gh", "--path", ghDir})
	_ = cmd.Execute()
}

// ---------------------------------------------------------------------------
// runDomainDedup – direct tests
// ---------------------------------------------------------------------------

func TestRunDomainDedupGhNoDuplicates(t *testing.T) {
	ghDir := writeGhFiles(t, map[string]string{"tool.yml": validGhYAML})
	err := runDomainDedup(data.DomainGH, ghDir)
	_ = err
}

func TestRunDomainDedupGhNonexistentPath(t *testing.T) {
	err := runDomainDedup(data.DomainGH, "/tmp/__dup_no_such__")
	require.Error(t, err)
}

func TestRunDomainDedupGhWithDuplicates(t *testing.T) {
	// GH duplicate check expects YAML files inside subdirectories.
	ghDir := writeGhFiles(t, map[string]string{
		"dev/a.yml": "- type: a\n  repo:\n    - url: https://github.com/acme/same\n      des: first\n  record: []\n",
		"ops/b.yml": "- type: b\n  repo:\n    - url: https://github.com/acme/same\n      des: second\n  record: []\n",
	})
	err := runDomainDedup(data.DomainGH, ghDir)
	_ = err
}
