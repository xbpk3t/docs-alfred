package audit

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// makeWiki builds a temp wiki tree for stats tests.
func makeWiki(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for rel, body := range files {
		p := filepath.Join(root, filepath.FromSlash(rel))
		require.NoError(t, os.MkdirAll(filepath.Dir(p), 0o755))
		require.NoError(t, os.WriteFile(p, []byte(body), 0o644))
	}
	return root
}

const researchFM = "---\ntype: research\ntitle: t\ndate: 2026-01-01\nsource: test\n---\n\nbody\n"

type kv struct{ k, v string }

// rowAsKV flattens a StatRow to its first key/value pair.
func rowAsKV(r StatRow) kv {
	return kv{fmt.Sprintf("%v", r[0].Key), fmt.Sprintf("%v", r[0].Value)}
}

func TestRunStats_Overview(t *testing.T) {
	root := makeWiki(t, map[string]string{
		"AI/topic1/a.md":  researchFM,
		"AI/topic2/b.md":  researchFM,
		"sys/sys/c.md":    "# no frontmatter\n",
		"digest-s.log.jsonl": "{}",
	})
	stats, err := RunStats(StatsOptions{WikiRoot: root, TopN: 10,
		ExcludeNames: []string{"digest-s.log.jsonl"}})
	require.NoError(t, err)
	require.Len(t, stats, 3) // overview, types, research topN
	ov := stats[0]
	assert.Equal(t, "overview", ov.Section)
	require.Len(t, ov.Data, 3)
	// Each row is a {metric, value} record.
	for _, r := range ov.Data {
		require.Len(t, r, 2)
	}
	assert.Equal(t, "metric", fmt.Sprintf("%v", ov.Data[0][0].Key))
	assert.Equal(t, "files", fmt.Sprintf("%v", ov.Data[0][0].Value))
	assert.Equal(t, "3", fmt.Sprintf("%v", ov.Data[0][1].Value)) // jsonl excluded
	assert.Equal(t, "metric", fmt.Sprintf("%v", ov.Data[1][0].Key))
	assert.Equal(t, "md", fmt.Sprintf("%v", ov.Data[1][0].Value))
	assert.Equal(t, "3", fmt.Sprintf("%v", ov.Data[1][1].Value))
	assert.Equal(t, "size", fmt.Sprintf("%v", ov.Data[2][0].Value))
	assert.Contains(t, fmt.Sprintf("%v", ov.Data[2][1].Value), "B") // human size
}

func TestRunStats_ResearchTop10_SortedDesc(t *testing.T) {
	root := makeWiki(t, map[string]string{
		"AI/a/x1.md": researchFM,
		"AI/a/x2.md": researchFM,
		"AI/b/y1.md": researchFM,
		"sys/z1.md":  researchFM,
		"sys/z2.md":  researchFM,
		"sys/z3.md":  researchFM,
	})
	stats, err := RunStats(StatsOptions{WikiRoot: root, TopN: 10})
	require.NoError(t, err)
	sec := stats[2] // [overview, types, research]
	assert.Equal(t, "research top10", sec.Section)
	require.Len(t, sec.Data, 3) // 3 research topics, one full record each
	rows := make([]kv, 0, len(sec.Data))
	for _, r := range sec.Data {
		rows = append(rows, rowAsKV(r))
	}
	// Descending by count: sys (3) → AI/a (2) → AI/b (1).
	assert.Equal(t, "topic", rows[0].k)
	assert.Equal(t, "sys", rows[0].v)
	assert.Equal(t, 2, len(sec.Data[0])) // {topic, count} two keys per record
	assert.Equal(t, "count", fmt.Sprintf("%v", sec.Data[0][1].Key))
	assert.Equal(t, "3", fmt.Sprintf("%v", sec.Data[0][1].Value))
	assert.Equal(t, "topic", rows[1].k)
	assert.Equal(t, "AI/a", rows[1].v)
	assert.Equal(t, "AI/b", rows[2].v)
}

func TestRunStats_ResearchTopN_Cap(t *testing.T) {
	root := makeWiki(t, map[string]string{
		"a/1.md": researchFM,
		"b/2.md": researchFM,
		"c/3.md": researchFM,
		"d/4.md": researchFM,
	})
	stats, err := RunStats(StatsOptions{WikiRoot: root, TopN: 2})
	require.NoError(t, err)
	assert.Equal(t, "research top2", stats[2].Section)
	assert.Len(t, stats[2].Data, 2) // 2 topics, one record each
}

func TestRunStats_TypesSection(t *testing.T) {
	root := makeWiki(t, map[string]string{
		"a/r1.md":  researchFM,                 // research (enum)
		"a/b1.md":  "---\ntype: blog\ntitle: t\ndate: 2026-01-01\nsource: s\n---\n\nx\n",   // blog (enum)
		"a/t1.md":  "---\ntype: transcript\ntitle: t\ndate: 2026-01-01\nsource: s\n---\n\nx\n", // unknown
		"a/n1.md":  "# no frontmatter\n",       // none
	})
	stats, err := RunStats(StatsOptions{WikiRoot: root})
	require.NoError(t, err)
	types := stats[1]
	assert.Equal(t, "types", types.Section)

	got := map[string]int{}
	for _, r := range types.Data {
		// {type, count, unknown?}
		name := fmt.Sprintf("%v", r[0].Value)
		count := fmt.Sprintf("%v", r[1].Value)
		var n int
		_, _ = fmt.Sscanf(count, "%d", &n)
		got[name] = n
	}
	assert.Equal(t, 1, got["research"])
	assert.Equal(t, 1, got["blog"])
	assert.Equal(t, 1, got["transcript"]) // unknown type visible, not dropped
	assert.Equal(t, 1, got["none"])
}

func TestRunStats_ExcludeNames(t *testing.T) {
	// data.go / tmp.md misc
	root := makeWiki(t, map[string]string{
		"a/x.md":   researchFM,
		"temp.md":  "# temp\n",
		"digest.jsonl": "{}\n",
	})
	stats, err := RunStats(StatsOptions{
		WikiRoot:     root,
		ExcludeNames: []string{"temp.md", "digest.jsonl"},
	})
	require.NoError(t, err)
	assert.Equal(t, "1", fmt.Sprintf("%v", stats[0].Data[0][1].Value)) // files metric = 1
}
