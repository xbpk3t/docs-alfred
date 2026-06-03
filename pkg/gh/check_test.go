package gh

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
	assert.False(t, HasErrors(r))

	r.Issues = append(r.Issues, CheckIssue{Severity: "warn", Message: "warning"})
	assert.False(t, HasErrors(r))

	r.Issues = append(r.Issues, CheckIssue{Severity: "error", Message: "error"})
	assert.True(t, HasErrors(r))
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

func TestDatePattern(t *testing.T) {
	assert.True(t, datePattern.MatchString("2024-01-01"))
	assert.True(t, datePattern.MatchString("1999-12-31"))
	assert.True(t, datePattern.MatchString("2024-13-01"), "regex only checks format, not month validity")
	assert.True(t, datePattern.MatchString("2024-00-00"))
	assert.False(t, datePattern.MatchString("not-a-date"))
	assert.False(t, datePattern.MatchString("2024/01/01"))
	assert.False(t, datePattern.MatchString("240101"))
}

func TestCheckResult_Report(t *testing.T) {
	r := &CheckResult{}
	r.addIssue("file.yml", "warn", "test warning")
	r.addIssue("file.yml", "error", "test error")
	// Report should not panic
	r.Report("test command")
}
