package cmd

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRootCommandMetadata(t *testing.T) {
	root := newRootCmd()

	require.Equal(t, "pwgen [website]", root.Use)
	require.True(t, root.HasAvailableFlags())
	require.NotNil(t, root.Flags().Lookup("secret"))
	require.NotNil(t, root.Flags().Lookup("length"))
	require.NotNil(t, root.Flags().Lookup("format"))
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

	assert.Equal(t, "pwgen", data["name"])
	assert.Equal(t, "lucas", data["createdby"])
	assert.Equal(t, "com.pwgen.lucas", data["bundleid"])
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
	assert.Equal(t, "pwgen", wf["name"])
	assert.Equal(t, "lucas", wf["created_by"])
	assert.Equal(t, "com.pwgen.lucas", wf["bundle_id"])
	assert.Equal(t, "https://github.com/xbpk3t/docs-alfred", wf["web_address"])
	assert.Equal(t, "github.com/xbpk3t/docs-alfred", data["go_mod_package"])
}

func TestRootCommandRequiresOneArg(t *testing.T) {
	root := newRootCmd()
	root.SetArgs([]string{})

	err := root.Execute()
	require.Error(t, err)
	require.Contains(t, err.Error(), "arg")
}
