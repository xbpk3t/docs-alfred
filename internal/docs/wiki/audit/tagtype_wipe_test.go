package audit

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Regression: a nested topic directory under a type (tag/type/topic/…) is
// walked AFTER that type's direct files, and recordDirType used to re-seed the
// (tag,type) bucket to 0 on it — wiping research counts mid-walk. Real break:
// ../wiki has AI/LLM-use/LLM files plus AI/LLM-use/LLM/{blog,code-search}/…
// subdirs, collapsing LLM-use from 12+ down to 1.
func TestRunTagTypeStats_NestedTopicDirDoesNotWipe(t *testing.T) {
	root := makeWiki(t, map[string]string{
		"AI/LLM-use/LLM/a.md":        researchFM, // research → AI/LLM-use
		"AI/LLM-use/LLM/b.md":        researchFM,
		"AI/LLM-use/LLM/sub/c.md":    researchFM, // nested topic dir, walked after a/b
		"AI/LLM-use/LLM/sub/deep/d.md": researchFM, // 4-seg, walked after c → would re-seed too
	})
	secs, err := RunTagTypeStats(root, 3)
	require.NoError(t, err)
	row := secs[0].Data[0]
	t.Logf("row=%v", row)
	// a/b/c/d are 4 research files under the LLM-use type; the nested sub/dirs
	// must not reset the bucket to 0 (pre-fix: 1).
	assert.Equal(t, "LLM-use (4)", valOf(row, 1))
}
