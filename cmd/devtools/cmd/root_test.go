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

func TestParseQuery_Stages(t *testing.T) {
	tests := []struct {
		q            string
		stage        queryStage
		tool         string
		action       string
		input        string
		toolFilter   string
		actionFilter string
	}{
		{q: "", stage: stageTools},
		{q: "   ", stage: stageTools},
		{q: "b", stage: stageTools, toolFilter: "b"},
		{q: "base64", stage: stageActions, tool: "base64"},
		{q: "base64 ", stage: stageActions, tool: "base64"},
		{q: "base64 en", stage: stageActions, tool: "base64", actionFilter: "en"},
		{q: "base64 encode", stage: stageInput, tool: "base64", action: "encode"},
		{q: "base64 encode ", stage: stageInput, tool: "base64", action: "encode"},
		{q: "base64 encode hello", stage: stageRun, tool: "base64", action: "encode", input: "hello"},
		{q: "base64 encode hello world", stage: stageRun, tool: "base64", action: "encode", input: "hello world"},
		{q: "base64 e hi", stage: stageRun, tool: "base64", action: "encode", input: "hi"},
		{q: "base64 decode aGVsbG8=", stage: stageRun, tool: "base64", action: "decode", input: "aGVsbG8="},
	}

	for _, tt := range tests {
		t.Run(tt.q, func(t *testing.T) {
			p := parseQuery(tt.q)
			assert.Equal(t, tt.stage, p.Stage)
			assert.Equal(t, tt.toolFilter, p.ToolFilter)
			assert.Equal(t, tt.actionFilter, p.ActionFilter)
			assert.Equal(t, tt.input, p.Input)
			if tt.tool == "" {
				assert.Nil(t, p.Tool)
			} else {
				require.NotNil(t, p.Tool)
				assert.Equal(t, tt.tool, p.Tool.Type)
			}
			if tt.action == "" {
				assert.Nil(t, p.Action)
			} else {
				require.NotNil(t, p.Action)
				assert.Equal(t, tt.action, p.Action.Name)
			}
		})
	}
}

func TestHandleQuery_ToolsLevel_AutocompleteContract(t *testing.T) {
	items := handleQuery("")
	require.NotEmpty(t, items)

	base64 := findItemByTitle(items, "base64")
	require.NotNil(t, base64)
	// Single-SF drill: valid false + autocomplete path (NOT chain arg hop).
	assert.False(t, base64.Valid)
	assert.Empty(t, base64.Arg)
	assert.Equal(t, "base64 ", base64.Autocomplete)
}

func TestHandleQuery_ToolsFilter(t *testing.T) {
	items := handleQuery("base")
	require.Len(t, items, 1)
	assert.Equal(t, "base64", items[0].Title)
	assert.False(t, items[0].Valid)

	unknown := handleQuery("nope")
	require.Len(t, unknown, 1)
	assert.Contains(t, unknown[0].Title, "未知工具")
	assert.False(t, unknown[0].Valid)
}

func TestHandleQuery_ActionsLevel_CleanTitles(t *testing.T) {
	items := handleQuery("base64")
	require.Len(t, items, 2)

	enc := findItemByTitle(items, "encode")
	require.NotNil(t, enc)
	assert.False(t, enc.Valid)
	assert.Empty(t, enc.Arg)
	assert.Equal(t, "base64 encode ", enc.Autocomplete)
	assert.NotContains(t, enc.Title, "base64")
	assert.NotContains(t, enc.Title, "dd")

	dec := findItemByTitle(items, "decode")
	require.NotNil(t, dec)
	assert.Equal(t, "base64 decode ", dec.Autocomplete)
}

func TestHandleQuery_ActionsFilter(t *testing.T) {
	items := handleQuery("base64 en")
	require.Len(t, items, 1)
	assert.Equal(t, "encode", items[0].Title)
}

func TestHandleQuery_PromptThenResult(t *testing.T) {
	prompt := handleQuery("base64 encode")
	require.Len(t, prompt, 1)
	assert.False(t, prompt[0].Valid)
	assert.Empty(t, prompt[0].Arg)
	assert.Contains(t, prompt[0].Title, "encode")

	result := handleQuery("base64 encode hello")
	require.Len(t, result, 1)
	assert.True(t, result[0].Valid)
	assert.Equal(t, "aGVsbG8=", result[0].Arg)
	assert.Equal(t, "aGVsbG8=", result[0].Title)
}

func TestHandleQuery_DecodeAndMultiWord(t *testing.T) {
	dec := handleQuery("base64 decode aGVsbG8=")
	require.Len(t, dec, 1)
	assert.Equal(t, "hello", dec[0].Arg)

	multi := handleQuery("base64 encode hello world")
	require.Len(t, multi, 1)
	assert.Equal(t, "aGVsbG8gd29ybGQ=", multi[0].Arg)
}

func TestHandleQuery_UnknownAction(t *testing.T) {
	items := handleQuery("base64 nope")
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

func TestFullChain_SingleSFJSONStages(t *testing.T) {
	f := &wf.AlfredFormatter{}

	// L1 tools
	raw, err := f.Format(handleQuery(""))
	require.NoError(t, err)
	var l1 wf.AlfredOutput
	require.NoError(t, json.Unmarshal([]byte(raw), &l1))
	require.NotEmpty(t, l1.Items)
	assert.False(t, l1.Items[0].Valid)
	assert.Equal(t, "base64 ", l1.Items[0].Autocomplete)

	// Simulate autocomplete expand → query "base64"
	raw, err = f.Format(handleQuery(strings.TrimSpace(l1.Items[0].Autocomplete)))
	require.NoError(t, err)
	var l2 wf.AlfredOutput
	require.NoError(t, json.Unmarshal([]byte(raw), &l2))
	require.Len(t, l2.Items, 2)
	titles := []string{l2.Items[0].Title, l2.Items[1].Title}
	assert.ElementsMatch(t, []string{"encode", "decode"}, titles)
	for _, it := range l2.Items {
		assert.False(t, it.Valid)
		assert.True(t, strings.HasPrefix(it.Autocomplete, "base64 "))
		assert.NotContains(t, it.Title, "base64")
	}

	// Pick encode via autocomplete
	var encAuto string
	for _, it := range l2.Items {
		if it.Title == "encode" {
			encAuto = it.Autocomplete
			break
		}
	}
	require.Equal(t, "base64 encode ", encAuto)

	raw, err = f.Format(handleQuery(strings.TrimSpace(encAuto)))
	require.NoError(t, err)
	var l3p wf.AlfredOutput
	require.NoError(t, json.Unmarshal([]byte(raw), &l3p))
	require.Len(t, l3p.Items, 1)
	assert.False(t, l3p.Items[0].Valid)

	raw, err = f.Format(handleQuery("base64 encode hello"))
	require.NoError(t, err)
	var l3r wf.AlfredOutput
	require.NoError(t, json.Unmarshal([]byte(raw), &l3r))
	require.Len(t, l3r.Items, 1)
	assert.True(t, l3r.Items[0].Valid)
	assert.Equal(t, "aGVsbG8=", l3r.Items[0].Arg)
}

func TestCobraWiring_SingleEntry(t *testing.T) {
	root := newRootCmd()
	// No tools/actions/run subcommands — single SF entry.
	for _, c := range root.Commands() {
		if c.Hidden {
			continue
		}
		assert.NotEqual(t, "tools", c.Name())
		assert.NotEqual(t, "actions", c.Name())
		assert.NotEqual(t, "run", c.Name())
	}
}

// TestWorkflowPlist_SingleSF guards the R1 architecture:
// one Script Filter → Clipboard; script is ./devtools "$1"; no Arg/Vars chain.
func TestWorkflowPlist_SingleSF(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	require.True(t, ok)
	plistPath := filepath.Join(filepath.Dir(thisFile), "..", ".workflow", "info.plist")
	raw, err := os.ReadFile(plistPath)
	require.NoError(t, err, "read %s", plistPath)
	content := string(raw)

	assert.Contains(t, content, `./devtools "$1"`)
	// Old multi-SF chain must be gone.
	assert.NotContains(t, content, `./devtools tools`)
	assert.NotContains(t, content, `./devtools actions`)
	assert.NotContains(t, content, `SF-ACTIONS`)
	assert.NotContains(t, content, `AV-TOOL`)
	assert.NotContains(t, content, `{var:tool}`)

	if runtime.GOOS != "darwin" {
		return
	}
	out, err := exec.Command("plutil", "-convert", "json", "-o", "-", plistPath).Output()
	require.NoError(t, err)
	var data map[string]any
	require.NoError(t, json.Unmarshal(out, &data))
	objects, _ := data["objects"].([]any)
	require.NotEmpty(t, objects)

	var sawSF, sawClip bool
	sfCount := 0
	for _, rawObj := range objects {
		obj, _ := rawObj.(map[string]any)
		uid, _ := obj["uid"].(string)
		cfg, _ := obj["config"].(map[string]any)
		typ, _ := obj["type"].(string)
		switch {
		case strings.Contains(typ, "scriptfilter"):
			sfCount++
			sawSF = true
			assert.Equal(t, `./devtools "$1"`, cfg["script"])
			assert.Equal(t, false, cfg["alfredfiltersresults"], "query state machine filters in-script")
			assert.EqualValues(t, 1, cfg["argumenttype"], "Optional so empty query lists tools")
			assert.Equal(t, true, cfg["withspace"])
		case strings.Contains(typ, "clipboard") || uid == "CLIPBOARD":
			sawClip = true
			assert.Equal(t, false, cfg["autopaste"], "copy only, no force-paste")
			assert.Equal(t, true, cfg["transient"], "do not pollute clipboard history")
		case strings.Contains(typ, "argument"):
			t.Fatalf("Arg/Vars utility must not exist in single-SF design, got uid=%s", uid)
		}
	}
	assert.True(t, sawSF)
	assert.True(t, sawClip)
	assert.Equal(t, 1, sfCount, "exactly one Script Filter")

	// connections: only SF → Clipboard
	conns, _ := data["connections"].(map[string]any)
	require.NotEmpty(t, conns)
	assert.Len(t, conns, 1)
}

func findItemByTitle(items []wf.AlfredItem, title string) *wf.AlfredItem {
	for i := range items {
		if items[i].Title == title {
			return &items[i]
		}
	}
	return nil
}
