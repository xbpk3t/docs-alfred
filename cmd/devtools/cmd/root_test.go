package cmd

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xbpk3t/docs-alfred/pkg/wf"
)

func TestListToolTypes_ChainContract(t *testing.T) {
	items := listToolTypes()
	require.NotEmpty(t, items)

	base64 := findItemByTitle(items, "base64")
	require.NotNil(t, base64)
	// Level 1 must leave SF via valid+arg (NOT autocomplete fill-back).
	assert.True(t, base64.Valid)
	assert.Equal(t, "base64", base64.Arg)
	assert.Empty(t, base64.Autocomplete)
}

func TestListActions_ShortTitleChainArg(t *testing.T) {
	items := listActionsForType("base64")
	require.Len(t, items, 2)

	enc := findItemByTitle(items, "encode")
	require.NotNil(t, enc)
	assert.True(t, enc.Valid)
	assert.Equal(t, "encode", enc.Arg, "action hop arg is only the action name")
	assert.Empty(t, enc.Autocomplete)
	assert.NotContains(t, enc.Title, "base64")

	dec := findItemByTitle(items, "decode")
	require.NotNil(t, dec)
	assert.Equal(t, "decode", dec.Arg)
}

func TestListActions_UnknownTool(t *testing.T) {
	items := listActionsForType("nope")
	require.Len(t, items, 1)
	assert.False(t, items[0].Valid)
	assert.Contains(t, items[0].Title, "未知工具")
}

func TestExecute_PromptThenResult(t *testing.T) {
	prompt := executeAction("base64", "encode", "")
	require.Len(t, prompt, 1)
	assert.False(t, prompt[0].Valid)
	assert.Empty(t, prompt[0].Arg)

	result := executeAction("base64", "encode", "hello")
	require.Len(t, result, 1)
	assert.True(t, result[0].Valid)
	assert.Equal(t, "aGVsbG8=", result[0].Arg)
	assert.Equal(t, "aGVsbG8=", result[0].Title)
}

func TestExecute_Decode(t *testing.T) {
	items := executeAction("base64", "decode", "aGVsbG8=")
	require.Len(t, items, 1)
	assert.Equal(t, "hello", items[0].Arg)
}

func TestExecute_MultiWordInput(t *testing.T) {
	items := executeAction("base64", "encode", "hello world")
	require.Len(t, items, 1)
	assert.Equal(t, "aGVsbG8gd29ybGQ=", items[0].Arg)
}

func TestExecute_UnknownTool(t *testing.T) {
	items := executeAction("nope", "encode", "x")
	require.Len(t, items, 1)
	assert.False(t, items[0].Valid)
	assert.Contains(t, items[0].Title, "不支持的工具")
}

func TestExecute_UnknownAction(t *testing.T) {
	items := executeAction("base64", "nope", "x")
	require.Len(t, items, 1)
	assert.False(t, items[0].Valid)
	assert.Contains(t, items[0].Title, "未知操作")
}

func TestRegistry_EveryToolHasRunnerAndActions(t *testing.T) {
	require.NotEmpty(t, toolsIndex)
	for _, tool := range toolsIndex {
		t.Run(tool.Type, func(t *testing.T) {
			require.NotNil(t, tool.Run, "tool %s must register Run", tool.Type)
			require.NotEmpty(t, tool.Actions, "tool %s must list actions", tool.Type)
			for _, a := range tool.Actions {
				assert.NotEmpty(t, a.Name)
				assert.NotEmpty(t, a.Title)
			}
		})
	}
}

func TestFullChain_AlfredJSONStages(t *testing.T) {
	f := &wf.AlfredFormatter{}

	raw, err := f.Format(listToolTypes())
	require.NoError(t, err)
	var l1 wf.AlfredOutput
	require.NoError(t, json.Unmarshal([]byte(raw), &l1))
	require.NotEmpty(t, l1.Items)
	assert.True(t, l1.Items[0].Valid)
	assert.Equal(t, "base64", l1.Items[0].Arg)
	assert.Empty(t, l1.Items[0].Autocomplete)

	raw, err = f.Format(listActionsForType(l1.Items[0].Arg))
	require.NoError(t, err)
	var l2 wf.AlfredOutput
	require.NoError(t, json.Unmarshal([]byte(raw), &l2))
	require.Len(t, l2.Items, 2)
	titles := []string{l2.Items[0].Title, l2.Items[1].Title}
	assert.ElementsMatch(t, []string{"encode", "decode"}, titles)
	for _, it := range l2.Items {
		assert.True(t, it.Valid)
		assert.NotEmpty(t, it.Arg)
		assert.NotContains(t, it.Title, "dv")
		assert.NotEqual(t, "base64 encode", it.Title)
	}

	action := l2.Items[0].Arg
	raw, err = f.Format(executeAction("base64", action, ""))
	require.NoError(t, err)
	var l3p wf.AlfredOutput
	require.NoError(t, json.Unmarshal([]byte(raw), &l3p))
	require.Len(t, l3p.Items, 1)
	assert.False(t, l3p.Items[0].Valid)

	raw, err = f.Format(executeAction("base64", action, "hello"))
	require.NoError(t, err)
	var l3r wf.AlfredOutput
	require.NoError(t, json.Unmarshal([]byte(raw), &l3r))
	require.Len(t, l3r.Items, 1)
	assert.True(t, l3r.Items[0].Valid)
	assert.Equal(t, "aGVsbG8=", l3r.Items[0].Arg)
}

func TestCobraWiring(t *testing.T) {
	root := newRootCmd()
	names := map[string]bool{}
	for _, c := range root.Commands() {
		if !c.Hidden {
			names[c.Name()] = true
		}
	}
	assert.True(t, names["tools"])
	assert.True(t, names["actions"])
	assert.True(t, names["run"])
}

// TestWorkflowPlist_ChainScripts guards the Alfred regression where
// script bodies used "{var:tool}" (literal) instead of env "$tool".
func TestWorkflowPlist_ChainScripts(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	require.True(t, ok)
	plistPath := filepath.Join(filepath.Dir(thisFile), "..", ".workflow", "info.plist")
	raw, err := os.ReadFile(plistPath)
	require.NoError(t, err, "read %s", plistPath)
	content := string(raw)

	assert.Contains(t, content, `./devtools tools`)
	assert.Contains(t, content, `./devtools actions "$tool"`)
	assert.Contains(t, content, `./devtools run "$tool" "$action" "$1"`)

	// Script body must not pass literal Alfred placeholders as CLI args.
	assert.NotContains(t, content, `actions "{var:tool}"`)
	assert.NotContains(t, content, `actions '{var:tool}'`)
	assert.NotContains(t, content, `run "{var:tool}"`)
	assert.NotContains(t, content, `run '{var:tool}'`)

	require.Contains(t, content, `SF-RUN`)

	// Parse with plutil on macOS for structured checks.
	if runtime.GOOS == "darwin" {
		out, err := exec.Command("plutil", "-convert", "json", "-o", "-", plistPath).Output()
		require.NoError(t, err)
		var data map[string]any
		require.NoError(t, json.Unmarshal(out, &data))
		objects, _ := data["objects"].([]any)
		require.NotEmpty(t, objects)

		var sawRun, sawClip bool
		for _, rawObj := range objects {
			obj, _ := rawObj.(map[string]any)
			uid, _ := obj["uid"].(string)
			cfg, _ := obj["config"].(map[string]any)
			typ, _ := obj["type"].(string)
			switch {
			case uid == "SF-RUN":
				sawRun = true
				// Optional argument so empty query still runs and shows prompt.
				assert.EqualValues(t, 1, cfg["argumenttype"], "SF-RUN must be Optional")
				assert.Equal(t, false, cfg["alfredfiltersresults"])
				assert.Equal(t, `./devtools run "$tool" "$action" "$1"`, cfg["script"])
			case uid == "SF-ACTIONS":
				assert.Equal(t, `./devtools actions "$tool"`, cfg["script"])
			case uid == "SF-TOOLS":
				assert.Equal(t, `./devtools tools`, cfg["script"])
			case strings.Contains(typ, "clipboard"):
				sawClip = true
				assert.Equal(t, false, cfg["autopaste"], "copy only, no force-paste")
				assert.Equal(t, true, cfg["transient"], "do not pollute clipboard history")
			}
		}
		assert.True(t, sawRun)
		assert.True(t, sawClip)
	}
}

func findItemByTitle(items []wf.AlfredItem, title string) *wf.AlfredItem {
	for i := range items {
		if items[i].Title == title {
			return &items[i]
		}
	}
	return nil
}
