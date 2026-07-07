package cmd

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xbpk3t/docs-alfred/cmd/ccx/internal"
)

// withBrokenStdout replaces os.Stdout with a closed pipe so all writes
// fail immediately, exercising error-return paths in Fprintf-based functions.
func withBrokenStdout(t *testing.T, fn func() error) error {
	t.Helper()

	old := os.Stdout
	r, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stdout = w
	require.NoError(t, w.Close())

	fnErr := fn()

	os.Stdout = old
	_ = r.Close()

	return fnErr
}

func TestWriteExportResult_DryRunWithEmptyPaths(t *testing.T) {
	result := &internal.ExportResult{DryRun: true}

	output, err := captureStdout(t, func() error {
		return writeExportResult(result)
	})
	require.NoError(t, err)
	assert.Contains(t, output, "Dry run")
}

func TestWriteExportResult_NormalWithPaths(t *testing.T) {
	result := &internal.ExportResult{
		OutputPath: "/wiki/topic/file.md",
		TopicPath:  "topic/sub",
		Title:      "My Title",
		EngTitle:   "my-title",
		DryRun:     false,
	}

	output, err := captureStdout(t, func() error {
		return writeExportResult(result)
	})
	require.NoError(t, err)
	assert.Contains(t, output, "Exported session to /wiki/topic/file.md")
	assert.NotContains(t, output, "Dry run")
}

func TestWriteLines_FprintfError(t *testing.T) {
	result := &internal.ExportResult{
		OutputPath: "/tmp/test.md",
		TopicPath:  "dev/go",
		Title:      "Test Title",
		EngTitle:   "test-title",
	}

	err := withBrokenStdout(t, func() error {
		return writeLines("Exported to %s", result)
	})

	require.Error(t, err)
}

func TestWriteLines_MultipleFields(t *testing.T) {
	result := &internal.ExportResult{
		OutputPath: "/a/b/c.md",
		TopicPath:  "dev/go",
		Title:      "Go Testing",
		EngTitle:   "go-testing",
	}

	output, err := captureStdout(t, func() error {
		return writeLines("Written: %s", result)
	})
	require.NoError(t, err)

	lines := strings.Split(strings.TrimRight(output, "\n"), "\n")
	require.Len(t, lines, 4)
	assert.Equal(t, "Written: /a/b/c.md", lines[0])
	assert.Equal(t, "Topic: dev/go", lines[1])
	assert.Equal(t, "Title: Go Testing", lines[2])
	assert.Equal(t, "EngTitle: go-testing", lines[3])
}

func TestNewSessionExportCmd_RunE_Flags(t *testing.T) {
	cmd := newSessionExportCmd()
	for _, name := range []string{"agent", "config", "dry-run", "verbose", "wiki-root", "output-dir", "session"} {
		require.NotNil(t, cmd.Flags().Lookup(name), "--%s flag should exist", name)
	}
}

func TestNewSessionExportCmd_Execute_MissingAgent(t *testing.T) {
	cmd := NewSessionCmd()
	cmd.SetArgs([]string{"export"})

	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), `required flag(s) "agent" not set`)
}

func TestNewSessionExportCmd_Execute_ConfigLoadError(t *testing.T) {
	cmd := NewSessionCmd()
	cmd.SetArgs([]string{"export", "--agent", "cc", "--config", "/nonexistent/path/config.yml"})

	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "load config")
}

func TestNewSessionExportCmd_Execute_WikiRootValidationError(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	t.Setenv("CCX_WIKI_ROOT", "")
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("CCX_AI_BASE_URL", "")
	t.Setenv("CCX_AI_MODEL", "")

	cmd := NewSessionCmd()
	cmd.SetArgs([]string{"export", "--agent", "cc", "--wiki-root", "/nonexistent/wiki/root"})

	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "export session")
}

func TestNewSessionExportCmd_Execute_InvalidAgent(t *testing.T) {
	wikiRoot := t.TempDir()
	cmd := NewSessionCmd()
	cmd.SetArgs([]string{"export", "--agent", "unknown", "--wiki-root", wikiRoot, "--session", "sess"})

	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "validation failed")
}

func TestNewSessionExportCmd_Execute_ResolveSessionError(t *testing.T) {
	projectDir := t.TempDir()
	wikiRoot := t.TempDir()
	homeDir := t.TempDir()
	t.Setenv("CLAUDE_PROJECT_DIR", projectDir)
	t.Setenv("HOME", homeDir)
	t.Setenv("CCX_WIKI_ROOT", "")
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("CCX_AI_BASE_URL", "")
	t.Setenv("CCX_AI_MODEL", "")

	cmd := NewSessionCmd()
	cmd.SetArgs([]string{"export", "--agent", "cc", "--wiki-root", wikiRoot, "--session", "nonexistent-session"})

	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "resolve session")
}
