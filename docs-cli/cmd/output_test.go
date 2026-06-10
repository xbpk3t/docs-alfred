package cmd

import (
	"encoding/json"
	"io"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	wikiuc "github.com/xbpk3t/docs-alfred/internal/wikiingest"
	workspaceuc "github.com/xbpk3t/docs-alfred/internal/workspaceops"
	"github.com/xbpk3t/docs-alfred/service/workspace/dotfiles"
)

func TestWriteCommandOutputJSONEnvelope(t *testing.T) {
	stdout := captureStdout(t)

	err := writeCommandOutput(outputFormatJSON, &CommandOutput{
		Name:    "demo",
		OK:      true,
		Summary: map[string]any{"count": 1},
		Results: []string{"item"},
	}, "")

	require.NoError(t, err)
	var got map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout()), &got))
	require.Equal(t, "demo", got["name"])
	require.Equal(t, true, got["ok"])
	require.Contains(t, got, "summary")
	require.Contains(t, got, "results")
}

func TestWriteWikiResultJSONIncludesURLDetails(t *testing.T) {
	stdout := captureStdout(t)

	err := writeWikiResult(&wikiuc.Result{
		Name:     "wiki add",
		WikiRoot: "wiki",
		URLResults: []wikiuc.URLResult{{
			URL:     "https://example.com/a",
			Status:  wikiuc.StatusSummaryWritten,
			Handled: true,
		}},
	}, outputFormatJSON)

	require.NoError(t, err)
	var got struct {
		Name    string             `json:"name"`
		OK      bool               `json:"ok"`
		Results []wikiuc.URLResult `json:"results"`
	}
	require.NoError(t, json.Unmarshal([]byte(stdout()), &got))
	require.Equal(t, "wiki add", got.Name)
	require.True(t, got.OK)
	require.Len(t, got.Results, 1)
	require.Equal(t, "https://example.com/a", got.Results[0].URL)
}

func TestWriteDotfilesSyncRecordJSONUsesCommonEnvelope(t *testing.T) {
	stdout := captureStdout(t)

	err := writeDotfilesSyncRecordResult(&workspaceuc.DotfilesSyncRecordResult{
		OK:           true,
		DotfilesPath: "dotfiles",
		ChangedFiles: []dotfiles.ChangeFile{{Path: "home/base/dev/default.nix", Status: "M"}},
	}, outputFormatJSON)

	require.NoError(t, err)
	var got struct {
		Name    string                `json:"name"`
		OK      bool                  `json:"ok"`
		Results []dotfiles.ChangeFile `json:"results"`
	}
	require.NoError(t, json.Unmarshal([]byte(stdout()), &got))
	require.Equal(t, "dotfiles sync-record", got.Name)
	require.True(t, got.OK)
	require.Len(t, got.Results, 1)
}

func captureStdout(t *testing.T) func() string {
	t.Helper()

	original := os.Stdout
	r, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stdout = w

	return func() string {
		require.NoError(t, w.Close())
		os.Stdout = original
		data, err := io.ReadAll(r)
		require.NoError(t, err)
		require.NoError(t, r.Close())

		return string(data)
	}
}
