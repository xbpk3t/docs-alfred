package cmd

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRootCommandOwnsAlfredActions(t *testing.T) {
	root := newRootCmd()

	require.Equal(t, "gh-alfred", root.Name())
	requireCommandNames(t, root.Commands(), []string{"export", "schema", "search", "sync", "validate"})
}

// TestWorkflowPlist_Metadata locks Alfred gallery fields to the shared
// docs-alfred convention (author/website) and per-workflow name/bundleid.
func TestWorkflowPlist_Metadata(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("plutil only on darwin")
	}
	_, thisFile, _, ok := runtime.Caller(0)
	require.True(t, ok)
	plistPath := filepath.Join(filepath.Dir(thisFile), "..", ".workflow", "info.plist")
	out, err := exec.Command("plutil", "-convert", "json", "-o", "-", plistPath).Output()
	require.NoError(t, err)
	var data map[string]any
	require.NoError(t, json.Unmarshal(out, &data))

	assert.Equal(t, "gh-alfred", data["name"])
	assert.Equal(t, "lucas", data["createdby"])
	assert.Equal(t, "com.gh-alfred.lucas", data["bundleid"])
	assert.Equal(t, "https://github.com/xbpk3t/docs-alfred", data["webaddress"])
}

func TestAkJSON_Metadata(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	require.True(t, ok)
	raw, err := os.ReadFile(filepath.Join(filepath.Dir(thisFile), "..", "ak.json"))
	require.NoError(t, err)
	var data map[string]any
	require.NoError(t, json.Unmarshal(raw, &data))
	wf, _ := data["workflow"].(map[string]any)
	require.NotNil(t, wf)
	assert.Equal(t, "gh-alfred", wf["name"])
	assert.Equal(t, "lucas", wf["created_by"])
	assert.Equal(t, "com.gh-alfred.lucas", wf["bundle_id"])
	assert.Equal(t, "https://github.com/xbpk3t/docs-alfred", wf["web_address"])
	assert.Equal(t, "github.com/xbpk3t/docs-alfred", data["go_mod_package"])
}

func requireCommandNames(t *testing.T, commands []*cobra.Command, want []string) {
	t.Helper()

	got := make([]string, 0, len(commands))
	for _, cmd := range commands {
		if cmd.Hidden {
			continue
		}
		got = append(got, cmd.Name())
	}

	require.ElementsMatch(t, want, got)
}
