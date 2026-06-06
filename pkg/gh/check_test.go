package gh

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xbpk3t/docs-alfred/pkg/checkutil"
)

func TestIsValidURL(t *testing.T) {
	assert.True(t, isValidURL("https://github.com/owner/repo"))
	assert.True(t, isValidURL("http://example.com"))
	assert.True(t, isValidURL("https://gitlab.com/group/project.git"))
	assert.False(t, isValidURL(""))
	assert.False(t, isValidURL("not-a-url"))
	assert.False(t, isValidURL("://bad"))
}

func TestCheckResult_HasErrors(t *testing.T) {
	r := &CheckResult{}
	assert.False(t, checkutil.HasErrors(r.Issues))

	r.Issues = append(r.Issues, checkutil.Issue{Severity: "warn", Message: "warning"})
	assert.False(t, checkutil.HasErrors(r.Issues))

	r.Issues = append(r.Issues, checkutil.Issue{Severity: "error", Message: "error"})
	assert.True(t, checkutil.HasErrors(r.Issues))
}

func TestGhCheck_UnreadableDir(t *testing.T) {
	result, err := RunGhCheck("/tmp/nonexistent-gh-check-dir-67890")
	require.Error(t, err)
	require.NotNil(t, result, "RunGhCheck always returns a result")
}

func TestGhCheck_EmptyDir(t *testing.T) {
	tmpDir := t.TempDir()
	result, err := RunGhCheck(tmpDir)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, 0, result.ScannedFiles)
}

func TestGhCheck_InvalidYAML(t *testing.T) {
	tmpDir := t.TempDir()
	badFile := filepath.Join(tmpDir, "bad.yml")
	require.NoError(t, os.WriteFile(badFile, []byte("invalid: [yaml: broken\n"), 0644))

	result, err := RunGhCheck(tmpDir)
	require.Error(t, err, "YAML parse error should be propagated")
	assert.Contains(t, err.Error(), "yaml")
	require.NotNil(t, result)
}

func TestCheckResult_ReportResult(t *testing.T) {
	r := &CheckResult{}
	r.addIssue("file.yml", "warn", "test warning")
	r.addIssue("file.yml", "error", "test error")

	report := (&checkutil.Result{Issues: r.Issues}).ReportResult("test command")
	assert.Contains(t, report, "WARN file.yml: test warning")
	assert.Contains(t, report, "ERROR file.yml: test error")
	assert.Contains(t, report, "test command failed")
}
