package skx

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildGraph(t *testing.T) {
	root := t.TempDir()
	refs := filepath.Join(root, "references")
	require.NoError(t, writeDirFile(refs, "3w3h.yml", `frontmatter:
  name: 3w3h
  role: composite
workflow:
  - phase: x
    steps:
      - kind: prompt
        name: brk
      - kind: prompt
        name: diagram
      - kind: prompt
        name: vs
what:
  is: x
`))
	require.NoError(t, writeDirFile(refs, "brk.yml", "frontmatter:\n  name: brk\n  role: atom\nwhat:\n  is: x\n"))
	require.NoError(t, writeDirFile(refs, "diagram.yml", "frontmatter:\n  name: diagram\n  role: atom\nwhat:\n  is: x\n"))
	require.NoError(t, writeDirFile(refs, "vs.yml", `frontmatter:
  name: vs
  role: composite
workflow:
  - phase: x
    steps:
      - kind: prompt
        name: table2yml
what:
  is: x
`))
	require.NoError(t, writeDirFile(refs, "table2yml.yml", "frontmatter:\n  name: table2yml\n  role: atom\nwhat:\n  is: x\n"))

	g, _, err := BuildGraph(refs)
	require.NoError(t, err)

	assert.Equal(t, []string{"3w3h", "brk", "diagram", "table2yml", "vs"}, g.Nodes)
	assert.ElementsMatch(t, []Edge{
		{From: "3w3h", To: "brk", Mode: "serial"},
		{From: "3w3h", To: "diagram", Mode: "serial"},
		{From: "3w3h", To: "vs", Mode: "serial"},
		{From: "vs", To: "table2yml", Mode: "serial"},
	}, g.Edges)
	assert.Empty(t, g.Cycles)
}

func TestBuildGraphPlainStepsNoDispatch(t *testing.T) {
	root := t.TempDir()
	refs := filepath.Join(root, "references")
	require.NoError(t, writeDirFile(refs, "a.yml", "frontmatter:\n  name: a\n  role: composite\nworkflow:\n  - phase: x\n    steps:\n      - 普通操作\n      - 另一个\n"))
	require.NoError(t, writeDirFile(refs, "b.yml", "frontmatter:\n  name: b\n  role: atom\n"))

	g, _, err := BuildGraph(refs)
	require.NoError(t, err)
	assert.Equal(t, []string{"a", "b"}, g.Nodes)
	assert.Empty(t, g.Edges, "plain string steps must not produce dispatch edges")
}

func TestBuildGraphCycle(t *testing.T) {
	root := t.TempDir()
	refs := filepath.Join(root, "references")
	require.NoError(t, writeDirFile(refs, "a.yml", "frontmatter:\n  name: a\n  role: composite\nworkflow:\n  - phase: x\n    steps:\n      - kind: prompt\n        name: b\n"))
	require.NoError(t, writeDirFile(refs, "b.yml", "frontmatter:\n  name: b\n  role: composite\nworkflow:\n  - phase: x\n    steps:\n      - kind: prompt\n        name: a\n"))

	g, _, err := BuildGraph(refs)
	require.NoError(t, err)
	assert.Equal(t, [][]string{{"a", "b", "a"}}, g.Cycles)
}

func TestBuildGraphSelfLoop(t *testing.T) {
	root := t.TempDir()
	refs := filepath.Join(root, "references")
	require.NoError(t, writeDirFile(refs, "a.yml", "frontmatter:\n  name: a\n  role: composite\nworkflow:\n  - phase: x\n    steps:\n      - kind: prompt\n        name: a\n"))

	g, _, err := BuildGraph(refs)
	require.NoError(t, err)
	assert.Equal(t, [][]string{{"a", "a"}}, g.Cycles)
}

func writeDirFile(dir, name, content string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644)
}
