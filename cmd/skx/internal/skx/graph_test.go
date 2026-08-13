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
  pl-serial:
    - brk
  pl-parallel:
    - diagram
    - vs
`))
	require.NoError(t, writeDirFile(refs, "brk.yml", "frontmatter:\n  name: brk\n  role: atom\n"))
	require.NoError(t, writeDirFile(refs, "diagram.yml", "frontmatter:\n  name: diagram\n  role: atom\n"))
	require.NoError(t, writeDirFile(refs, "vs.yml", `frontmatter:
  name: vs
  role: composite
  pl-serial:
    - table2yml
`))
	require.NoError(t, writeDirFile(refs, "table2yml.yml", "frontmatter:\n  name: table2yml\n  role: atom\n"))

	g, _, err := BuildGraph(refs)
	require.NoError(t, err)

	assert.Equal(t, []string{"3w3h", "brk", "diagram", "table2yml", "vs"}, g.Nodes)
	assert.ElementsMatch(t, []Edge{
		{From: "3w3h", To: "brk", Mode: "serial"},
		{From: "3w3h", To: "diagram", Mode: "parallel"},
		{From: "3w3h", To: "vs", Mode: "parallel"},
		{From: "vs", To: "table2yml", Mode: "serial"},
	}, g.Edges)
	assert.Empty(t, g.Cycles)
}

func TestBuildGraphLegacyPipelineIgnored(t *testing.T) {
	root := t.TempDir()
	refs := filepath.Join(root, "references")
	require.NoError(t, writeDirFile(refs, "a.yml", "frontmatter:\n  name: a\n  role: composite\n  pipeline:\n    - b\n"))
	require.NoError(t, writeDirFile(refs, "b.yml", "frontmatter:\n  name: b\n  role: atom\n"))

	g, _, err := BuildGraph(refs)
	require.NoError(t, err)
	assert.Equal(t, []string{"a", "b"}, g.Nodes)
	assert.Empty(t, g.Edges, "legacy pipeline must not produce edges")
}

func TestBuildGraphCycle(t *testing.T) {
	root := t.TempDir()
	refs := filepath.Join(root, "references")
	require.NoError(t, writeDirFile(refs, "a.yml", "frontmatter:\n  name: a\n  role: composite\n  pl-parallel:\n    - b\n"))
	require.NoError(t, writeDirFile(refs, "b.yml", "frontmatter:\n  name: b\n  role: composite\n  pl-serial:\n    - a\n"))

	g, _, err := BuildGraph(refs)
	require.NoError(t, err)
	assert.Equal(t, [][]string{{"a", "b", "a"}}, g.Cycles)
}

func TestBuildGraphSelfLoop(t *testing.T) {
	root := t.TempDir()
	refs := filepath.Join(root, "references")
	require.NoError(t, writeDirFile(refs, "a.yml", "frontmatter:\n  name: a\n  role: composite\n  pl-serial:\n    - a\n"))

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
