package skx

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildGraph(t *testing.T) {
	dir := t.TempDir()
	writePrompt(t, dir, "3w3h.md", mdPrompt("composite", "x", []string{"brk", "diagram", "vs"}, validBody))
	writePrompt(t, dir, "brk.md", mdPrompt("atom", "x", nil, validBody))
	writePrompt(t, dir, "diagram.md", mdPrompt("atom", "x", nil, validBody))
	writePrompt(t, dir, "vs.md", mdPrompt("composite", "x", []string{"table2yml"}, validBody))
	writePrompt(t, dir, "table2yml.md", mdPrompt("atom", "x", nil, validBody))

	g, _, err := BuildGraph(dir)
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

func TestBuildGraphNoPipeline(t *testing.T) {
	dir := t.TempDir()
	writePrompt(t, dir, "a.md", mdPrompt("atom", "x", nil, validBody))
	writePrompt(t, dir, "b.md", mdPrompt("atom", "x", nil, validBody))

	g, _, err := BuildGraph(dir)
	require.NoError(t, err)
	assert.Equal(t, []string{"a", "b"}, g.Nodes)
	assert.Empty(t, g.Edges)
}

func TestBuildGraphCycle(t *testing.T) {
	dir := t.TempDir()
	writePrompt(t, dir, "a.md", mdPrompt("composite", "x", []string{"b"}, validBody))
	writePrompt(t, dir, "b.md", mdPrompt("composite", "x", []string{"a"}, validBody))

	g, _, err := BuildGraph(dir)
	require.NoError(t, err)
	assert.Equal(t, [][]string{{"a", "b", "a"}}, g.Cycles)
}

func TestBuildGraphSelfLoop(t *testing.T) {
	dir := t.TempDir()
	writePrompt(t, dir, "a.md", mdPrompt("composite", "x", []string{"a"}, validBody))

	g, _, err := BuildGraph(dir)
	require.NoError(t, err)
	assert.Equal(t, [][]string{{"a", "a"}}, g.Cycles)
}

func TestBuildGraphHiddenExcluded(t *testing.T) {
	dir := t.TempDir()
	writePrompt(t, dir, "a.md", mdPrompt("composite", "x", []string{"hidden"}, validBody))
	writePrompt(t, dir, ".hidden.md", mdPrompt("atom", "x", nil, validBody))

	g, skipped, err := BuildGraph(dir)
	require.NoError(t, err)
	assert.Equal(t, []string{"a"}, g.Nodes)
	assert.Empty(t, g.Edges)
	assert.Contains(t, skipped, "a→hidden(serial)")
}

func TestBuildGraphIgnoresShadowedStem(t *testing.T) {
	dir := t.TempDir()
	writePrompt(t, dir, "a.md", mdPrompt("atom", "x", nil, validBody))
	writePrompt(t, dir, "b.md", mdPrompt("atom", "x", nil, validBody))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "zzz"), 0o755))
	writePrompt(t, dir, filepath.Join("zzz", "a.md"), mdPrompt("composite", "x", []string{"b"}, validBody))

	g, _, err := BuildGraph(dir)
	require.NoError(t, err)
	assert.Equal(t, []string{"a", "b"}, g.Nodes)
	assert.Empty(t, g.Edges)
}
