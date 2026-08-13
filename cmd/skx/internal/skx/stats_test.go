package skx

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRankStatsOrderAndSuggestion(t *testing.T) {
	entries := []StatsEntry{
		{Name: "a", Count: 2},
		{Name: "b", Count: 100},
		{Name: "c", Count: 1},
	}
	rankings := RankStats(entries, 400) // b=25% keep; a=0.5%, c=0.25% both <1% → demote

	require.Len(t, rankings, 3)
	assert.Equal(t, "b", rankings[0].Name) // highest count first
	assert.Equal(t, 1, rankings[0].Rank)
	assert.Equal(t, "a", rankings[1].Name)
	assert.Equal(t, "c", rankings[2].Name)

	assert.Equal(t, "keep", rankings[0].Sug)
	assert.Equal(t, "demote", rankings[1].Sug)
	assert.Equal(t, "demote", rankings[2].Sug)
}

func TestRankStatsComputesTotalWhenZero(t *testing.T) {
	entries := []StatsEntry{
		{Name: "x", Count: 30},
		{Name: "y", Count: 10},
	}
	rankings := RankStats(entries, 0) // total derived from entries (40)
	require.Len(t, rankings, 2)
	assert.InDelta(t, 75.0, rankings[0].Percent, 0.01) // 30/40
	assert.InDelta(t, 25.0, rankings[1].Percent, 0.01)
}

func TestRecordHitIncrementsExisting(t *testing.T) {
	path := filepath.Join(t.TempDir(), "stats.json")
	require.NoError(t, SaveStats(path, []StatsEntry{{Name: "brk", Count: 5, Target: "analysis/brk"}}))

	require.NoError(t, RecordHit(path, "brk", "analysis/brk"))
	entries, err := LoadStats(path)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, 6, entries[0].Count)
	assert.NotEmpty(t, entries[0].LastTs)
}

func TestRecordHitAppendsNew(t *testing.T) {
	path := filepath.Join(t.TempDir(), "stats.json")
	require.NoError(t, SaveStats(path, nil))

	require.NoError(t, RecordHit(path, "uu", "analysis/uu"))
	entries, err := LoadStats(path)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, "uu", entries[0].Name)
	assert.Equal(t, 1, entries[0].Count)
	assert.Equal(t, "analysis/uu", entries[0].Target)
}

func TestLoadStatsMissingFile(t *testing.T) {
	entries, err := LoadStats(filepath.Join(t.TempDir(), "nope.json"))
	require.NoError(t, err)
	assert.Empty(t, entries)
}

func TestResolvePromptHitAndMiss(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, writeDirFile(dir, "brk.yml", "frontmatter:\n  name: brk\n  role: atom\n"))
	require.NoError(t, writeDirFile(dir, ".TableCate.yml", "frontmatter:\n  name: TableCate\n  role: atom\n"))

	rel, mdAbs, ok, err := ResolvePrompt(dir, "brk")
	require.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, "brk", rel)
	assert.Equal(t, filepath.Join(dir, "brk.md"), mdAbs)

	// hidden file must not resolve
	_, _, okHidden, err := ResolvePrompt(dir, "TableCate")
	require.NoError(t, err)
	assert.False(t, okHidden)

	_, _, okMiss, err := ResolvePrompt(dir, "ghost")
	require.NoError(t, err)
	assert.False(t, okMiss)
}

func TestAvailableNamesExcludesHidden(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, writeDirFile(dir, "b.yml", "frontmatter:\n  name: b\n  role: atom\n"))
	require.NoError(t, writeDirFile(dir, "a.yml", "frontmatter:\n  name: a\n  role: atom\n"))
	require.NoError(t, writeDirFile(dir, ".hidden.yml", "frontmatter:\n  name: hidden\n  role: atom\n"))

	names, err := AvailableNames(dir)
	require.NoError(t, err)
	assert.Equal(t, []string{"a", "b"}, names)
}

func TestSkipHidden(t *testing.T) {
	assert.True(t, skipHidden("repo/.TableCate.yml"))
	assert.True(t, skipHidden(".hidden.yml"))
	assert.False(t, skipHidden("analysis/brk.yml"))
	assert.False(t, skipHidden("repo/vs.yml"))
}
