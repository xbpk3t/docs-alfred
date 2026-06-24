package cmd

import (
	"bytes"
	"encoding/json"
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

func TestNewSessionCmd_HasSubcommands(t *testing.T) {
	cmd := NewSessionCmd()

	names := make(map[string]bool)
	for _, sub := range cmd.Commands() {
		names[sub.Name()] = true
	}

	require.True(t, names["chain"], "session command should have 'chain' subcommand")
	require.True(t, names["export"], "session command should have 'export' subcommand")
}

func TestNewSessionCmd_ShowsHelpOnRun(t *testing.T) {
	cmd := NewSessionCmd()

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{})

	err := cmd.Execute()
	require.NoError(t, err)
	require.Contains(t, buf.String(), "Commands for managing Claude Code sessions")
}

func TestNewSessionChainCmd_Flags(t *testing.T) {
	cmd := newSessionChainCmd()

	require.True(t, cmd.Flags().HasFlags(), "chain command should have flags")

	jsonFlag := cmd.Flags().Lookup("json")
	require.NotNil(t, jsonFlag, "--json flag should exist")

	rawFlag := cmd.Flags().Lookup("raw")
	require.NotNil(t, rawFlag, "--raw flag should exist")

	sessionFlag := cmd.Flags().Lookup("session")
	require.NotNil(t, sessionFlag, "--session flag should exist")
}

func TestNewSessionExportCmd_Flags(t *testing.T) {
	cmd := newSessionExportCmd()

	require.True(t, cmd.Flags().HasFlags(), "export command should have flags")

	configFlag := cmd.Flags().Lookup("config")
	require.NotNil(t, configFlag, "--config flag should exist")

	dryRunFlag := cmd.Flags().Lookup("dry-run")
	require.NotNil(t, dryRunFlag, "--dry-run flag should exist")

	verboseFlag := cmd.Flags().Lookup("verbose")
	require.NotNil(t, verboseFlag, "--verbose flag should exist")

	wikiRootFlag := cmd.Flags().Lookup("wiki-root")
	require.NotNil(t, wikiRootFlag, "--wiki-root flag should exist")

	outputDirFlag := cmd.Flags().Lookup("output-dir")
	require.NotNil(t, outputDirFlag, "--output-dir flag should exist")

	sessionFlag := cmd.Flags().Lookup("session")
	require.NotNil(t, sessionFlag, "--session flag should exist")
}

func TestWriteJSON(t *testing.T) {
	prev := "prev-session-1"
	ended := "2025-01-01T12:00:00Z"
	chain := []internal.ChainRecord{
		{
			SessionID:      "sess-1",
			PrevSessionID:  nil,
			StartedAt:      "2025-01-01T10:00:00Z",
			EndedAt:        &ended,
			Display:        "test display",
			TranscriptPath: "/tmp/sess-1.jsonl",
			IsSidechain:    false,
		},
		{
			SessionID:      "sess-2",
			PrevSessionID:  &prev,
			StartedAt:      "2025-01-01T11:00:00Z",
			EndedAt:        nil,
			Display:        "sidechain display",
			TranscriptPath: "/tmp/sess-2.jsonl",
			IsSidechain:    true,
		},
	}

	output, err := captureStdout(t, func() error {
		return writeJSON(chain)
	})

	require.NoError(t, err)

	var decoded []internal.ChainRecord
	require.NoError(t, json.Unmarshal([]byte(output), &decoded))
	require.Len(t, decoded, 2)
	require.Equal(t, "sess-1", decoded[0].SessionID)
	require.Nil(t, decoded[0].PrevSessionID)
	require.NotNil(t, decoded[0].EndedAt)
	require.Equal(t, "2025-01-01T12:00:00Z", *decoded[0].EndedAt)
	require.Equal(t, "sess-2", decoded[1].SessionID)
	require.NotNil(t, decoded[1].PrevSessionID)
	require.Equal(t, "prev-session-1", *decoded[1].PrevSessionID)
	require.True(t, decoded[1].IsSidechain)
	require.Nil(t, decoded[1].EndedAt)
}

func TestWriteJSON_EmptyChain(t *testing.T) {
	output, err := captureStdout(t, func() error {
		return writeJSON([]internal.ChainRecord{})
	})

	require.NoError(t, err)

	var decoded []internal.ChainRecord
	require.NoError(t, json.Unmarshal([]byte(output), &decoded))
	require.Empty(t, decoded)
}

func TestWriteRaw(t *testing.T) {
	prev := "prev-session-1"
	ended := "2025-01-01T12:00:00Z"
	chain := []internal.ChainRecord{
		{
			SessionID:      "sess-1",
			PrevSessionID:  nil,
			StartedAt:      "2025-01-01T10:00:00Z",
			EndedAt:        &ended,
			Display:        "test display",
			TranscriptPath: "/tmp/sess-1.jsonl",
			IsSidechain:    false,
		},
		{
			SessionID:      "sess-2",
			PrevSessionID:  &prev,
			StartedAt:      "2025-01-01T11:00:00Z",
			EndedAt:        nil,
			Display:        "sidechain display",
			TranscriptPath: "/tmp/sess-2.jsonl",
			IsSidechain:    true,
		},
	}

	output, err := captureStdout(t, func() error {
		return writeRaw(chain)
	})

	require.NoError(t, err)

	lines := strings.Split(strings.TrimRight(output, "\n"), "\n")
	require.Len(t, lines, 2)

	// First line: session with endedAt and no prevSessionID
	fields1 := strings.Split(lines[1-1], "\t")
	// fields: sessionID, prevSessionID, startedAt, endedAt, display, transcriptPath, isSidechain
	require.Len(t, fields1, 7)
	require.Equal(t, "sess-1", fields1[0])
	require.Empty(t, fields1[1], "empty prevSessionID should be empty string")
	require.Equal(t, "2025-01-01T10:00:00Z", fields1[2])
	require.Equal(t, "2025-01-01T12:00:00Z", fields1[3])
	require.Equal(t, "test display", fields1[4])
	require.Equal(t, "/tmp/sess-1.jsonl", fields1[5])
	require.Equal(t, "false", fields1[6])

	// Second line: sidechain with prevSessionID, no endedAt
	fields2 := strings.Split(lines[2-1], "\t")
	require.Len(t, fields2, 7)
	require.Equal(t, "sess-2", fields2[0])
	require.Equal(t, "prev-session-1", fields2[1])
	require.Equal(t, "2025-01-01T11:00:00Z", fields2[2])
	require.Empty(t, fields2[3], "empty endedAt should be empty string")
	require.Equal(t, "sidechain display", fields2[4])
	require.Equal(t, "/tmp/sess-2.jsonl", fields2[5])
	require.Equal(t, "true", fields2[6])
}

func TestWriteHumanReadable(t *testing.T) {
	prev := "prev-session-1"
	ended := "2025-01-01T12:00:00Z"
	chain := []internal.ChainRecord{
		{
			SessionID:      "sess-1",
			PrevSessionID:  nil,
			StartedAt:      "2025-01-01T10:00:00Z",
			EndedAt:        &ended,
			Display:        "main session display",
			TranscriptPath: "/tmp/sess-1.jsonl",
			IsSidechain:    false,
		},
		{
			SessionID:      "sess-2",
			PrevSessionID:  &prev,
			StartedAt:      "2025-01-01T11:00:00Z",
			EndedAt:        nil,
			Display:        "sidechain display",
			TranscriptPath: "/tmp/sess-2.jsonl",
			IsSidechain:    true,
		},
	}

	output, err := captureStdout(t, func() error {
		return writeHumanReadable(chain)
	})

	require.NoError(t, err)

	require.Contains(t, output, "Session Chain (2 entries):")
	require.Contains(t, output, "[0] sess-1 (prev: null, started: 2025-01-01T10:00:00Z, ended: 2025-01-01T12:00:00Z)")
	require.Contains(t, output, "display: main session display")
	require.Contains(t, output, "transcript: /tmp/sess-1.jsonl")
	require.Contains(t, output, "[1] sess-2 [sidechain] (prev: prev-session-1, started: 2025-01-01T11:00:00Z, ended: null)")
	require.Contains(t, output, "display: sidechain display")
	require.Contains(t, output, "transcript: /tmp/sess-2.jsonl")
}

func TestWriteHumanReadable_EmptyChain(t *testing.T) {
	output, err := captureStdout(t, func() error {
		return writeHumanReadable([]internal.ChainRecord{})
	})

	require.NoError(t, err)
	require.Contains(t, output, "Session Chain (0 entries):")
}

func TestWriteHumanReadable_NoSidechain(t *testing.T) {
	chain := []internal.ChainRecord{
		{
			SessionID:      "sess-only",
			PrevSessionID:  nil,
			StartedAt:      "2025-01-01T10:00:00Z",
			EndedAt:        nil,
			Display:        "only session",
			TranscriptPath: "/tmp/sess-only.jsonl",
			IsSidechain:    false,
		},
	}

	output, err := captureStdout(t, func() error {
		return writeHumanReadable(chain)
	})

	require.NoError(t, err)
	require.Contains(t, output, "Session Chain (1 entries):")
	require.Contains(t, output, "[0] sess-only (prev: null, started: 2025-01-01T10:00:00Z, ended: null)")
	require.NotContains(t, output, "[sidechain]")
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
	result := &internal.ExportResult{
		OutputPath: "",
		TopicPath:  "",
		Title:      "",
		EngTitle:   "",
	}

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

func TestWriteHumanReadable_AllNullOptionals(t *testing.T) {
	chain := []internal.ChainRecord{
		{
			SessionID:      "sess-minimal",
			PrevSessionID:  nil,
			StartedAt:      "2025-01-01T00:00:00Z",
			EndedAt:        nil,
			Display:        "minimal",
			TranscriptPath: "/tmp/minimal.jsonl",
			IsSidechain:    false,
		},
	}

	output, err := captureStdout(t, func() error {
		return writeHumanReadable(chain)
	})

	require.NoError(t, err)
	require.Contains(t, output, "prev: null")
	require.Contains(t, output, "ended: null")
}

func TestWriteRaw_EmptyChain(t *testing.T) {
	output, err := captureStdout(t, func() error {
		return writeRaw([]internal.ChainRecord{})
	})

	require.NoError(t, err)
	require.Empty(t, strings.TrimSpace(output))
}
