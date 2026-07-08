package cmd

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/xbpk3t/docs-alfred/cmd/ccx/internal"
)

// captureStdout runs fn while capturing everything written to os.Stdout.
func captureStdout(t *testing.T, fn func() error) (string, error) {
	t.Helper()

	old := os.Stdout
	r, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stdout = w

	fnErr := fn()

	require.NoError(t, w.Close())
	os.Stdout = old

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	_ = r.Close()

	return buf.String(), fnErr
}

func TestNewSessionCmd_HasOnlyExportSubcommand(t *testing.T) {
	cmd := NewSessionCmd()

	names := make(map[string]bool)
	for _, sub := range cmd.Commands() {
		names[sub.Name()] = true
	}

	require.True(t, names["export"], "session command should have 'export' subcommand")
}

func TestNewSessionCmd_ShowsHelpOnRun(t *testing.T) {
	cmd := NewSessionCmd()

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{})

	err := cmd.Execute()
	require.NoError(t, err)
	require.Contains(t, buf.String(), "Commands for exporting agent sessions")
}

func TestNewSessionExportCmd_Flags(t *testing.T) {
	cmd := newSessionExportCmd()

	require.True(t, cmd.Flags().HasFlags(), "export command should have flags")

	for _, name := range []string{"agent", "config", "dry-run", "verbose", "wiki-root", "output-dir", "session"} {
		require.NotNil(t, cmd.Flags().Lookup(name), "--%s flag should exist", name)
	}
}

func TestWriteExportResult_DryRun(t *testing.T) {
	result := &internal.ExportResult{
		OutputPath: "/tmp/wiki/topic/2025-01-01-title.md",
		TopicPath:  "topic/sub",
		Title:      "Test Title",
		EngTitle:   "test-title",
		DryRun:     true,
	}

	output, err := captureStdout(t, func() error {
		return writeExportResult(result)
	})

	require.NoError(t, err)
	require.Contains(t, output, "Dry run: would write to /tmp/wiki/topic/2025-01-01-title.md")
	require.Contains(t, output, "Topic: topic/sub")
	require.Contains(t, output, "Title: Test Title")
	require.Contains(t, output, "EngTitle: test-title")
}

func TestWriteExportResult_Normal(t *testing.T) {
	result := &internal.ExportResult{
		OutputPath: "/tmp/wiki/topic/2025-01-01-title.md",
		TopicPath:  "topic/sub",
		Title:      "Test Title",
		EngTitle:   "test-title",
		DryRun:     false,
	}

	output, err := captureStdout(t, func() error {
		return writeExportResult(result)
	})

	require.NoError(t, err)
	require.Contains(t, output, "Exported session to /tmp/wiki/topic/2025-01-01-title.md")
	require.Contains(t, output, "Topic: topic/sub")
	require.Contains(t, output, "Title: Test Title")
	require.Contains(t, output, "EngTitle: test-title")
	require.NotContains(t, output, "Dry run")
}

func TestWriteLines(t *testing.T) {
	result := &internal.ExportResult{
		OutputPath: "/output/path.md",
		TopicPath:  "dev/go",
		Title:      "My Title",
		EngTitle:   "my-title",
	}

	output, err := captureStdout(t, func() error {
		return writeLines("Custom prefix %s", result)
	})

	require.NoError(t, err)

	lines := strings.Split(strings.TrimRight(output, "\n"), "\n")
	require.Len(t, lines, 4)
	require.Equal(t, "Custom prefix /output/path.md", lines[0])
	require.Equal(t, "Topic: dev/go", lines[1])
	require.Equal(t, "Title: My Title", lines[2])
	require.Equal(t, "EngTitle: my-title", lines[3])
}

func TestWriteLines_EmptyValues(t *testing.T) {
	result := &internal.ExportResult{}

	output, err := captureStdout(t, func() error {
		return writeLines("Prefix: %s", result)
	})

	require.NoError(t, err)

	lines := strings.Split(strings.TrimRight(output, "\n"), "\n")
	require.Len(t, lines, 4)
	require.Equal(t, "Prefix: ", lines[0])
	require.Equal(t, "Topic: ", lines[1])
	require.Equal(t, "Title: ", lines[2])
	require.Equal(t, "EngTitle: ", lines[3])
}
