package cmd

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
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

// TestWorkflowPlist_ActionGraph locks the thin-executor design:
// one Open URL (mods share it via JSON) + one Clipboard on ⌘.
func TestWorkflowPlist_ActionGraph(t *testing.T) {
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

	objects, _ := data["objects"].([]any)
	var openURLUIDs, clipboardUIDs, scriptFilterUIDs []string
	for _, rawObj := range objects {
		obj, _ := rawObj.(map[string]any)
		uid, _ := obj["uid"].(string)
		typ, _ := obj["type"].(string)
		switch {
		case strings.Contains(typ, "openurl"):
			openURLUIDs = append(openURLUIDs, uid)
		case strings.Contains(typ, "clipboard"):
			clipboardUIDs = append(clipboardUIDs, uid)
		case strings.Contains(typ, "scriptfilter"):
			scriptFilterUIDs = append(scriptFilterUIDs, uid)
		}
	}
	require.Len(t, openURLUIDs, 1, "exactly one Open URL executor")
	require.Len(t, clipboardUIDs, 1, "exactly one Clipboard executor")
	require.Len(t, scriptFilterUIDs, 1)

	sfUID := scriptFilterUIDs[0]
	conns, _ := data["connections"].(map[string]any)
	sfConns, _ := conns[sfUID].([]any)
	require.Len(t, sfConns, 2, "SF should only fan out to Open URL + Clipboard")

	const modNone = 0
	const modCmd = 1048576 // ⌘
	var sawOpen, sawClip bool
	for _, rawC := range sfConns {
		c, _ := rawC.(map[string]any)
		dest, _ := c["destinationuid"].(string)
		mods := int(asFloat(c["modifiers"]))
		switch dest {
		case openURLUIDs[0]:
			assert.Equal(t, modNone, mods, "Open URL must be default (no modifier) connection")
			sawOpen = true
		case clipboardUIDs[0]:
			assert.Equal(t, modCmd, mods, "Clipboard must be on cmd modifier")
			sawClip = true
		default:
			t.Fatalf("unexpected SF destination %s", dest)
		}
	}
	assert.True(t, sawOpen)
	assert.True(t, sawClip)
}

func asFloat(v any) float64 {
	switch n := v.(type) {
	case float64:
		return n
	case int:
		return float64(n)
	case json.Number:
		f, _ := n.Float64()
		return f
	default:
		return -1
	}
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
