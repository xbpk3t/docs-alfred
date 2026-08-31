package audit

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTopicMetrics(t *testing.T) {
	root := makeWiki(t, map[string]string{
		"AI/go/r1.md":  researchFM,
		"AI/go/r2.md":  researchFM,
		"AI/go/b1.md":  "# no frontmatter\n",                                                       // content, no type → counted, not research
		"sys/sys/a.md": researchFM,                                                                 // research
		"sys/sys/t.md": "---\ntype: transcript\ntitle: t\ndate: 2026-01-01\nsource: x\n---\n\nx\n", // artifact tag → excluded
		"digest.jsonl": "{}\n",                                                                     // excluded (jsonl)
	})
	got, err := TopicMetrics(root)
	require.NoError(t, err)

	require.Len(t, got, 2) // AI/go + sys/, transcript + jsonl excluded

	// research desc → AI/go (2 research) first, then sys/sys (1).
	assert.Equal(t, "AI/go", got[0].Path)
	assert.Equal(t, 3, got[0].Files) // r1, r2, b1
	assert.Equal(t, 2, got[0].Research)
	assert.Equal(t, "sys/sys", got[1].Path)
	assert.Equal(t, 1, got[1].Files)
	assert.Equal(t, 1, got[1].Research)

	// Size > 0 for both.
	assert.Positive(t, got[0].Size)
	assert.Positive(t, got[1].Size)
}

func TestTopicMetrics_DeterministicTiebreak(t *testing.T) {
	root := makeWiki(t, map[string]string{
		"a/x1.md": researchFM,
		"b/y1.md": researchFM,
	})
	got, err := TopicMetrics(root)
	require.NoError(t, err)
	require.Len(t, got, 2)
	// Equal research + files → alphabetical: a before b.
	assert.Equal(t, "a", got[0].Path)
	assert.Equal(t, "b", got[1].Path)
}
