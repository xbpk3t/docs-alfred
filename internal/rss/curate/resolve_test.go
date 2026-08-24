package curate

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExtractDeterministicFromOPMLOutline(t *testing.T) {
	seed := `<?xml version="1.0"?>
<outline text="人人都是产品经理" xmlUrl="https://wechat2rss.bestblogs.dev/feed/abc.xml"/>
<outline text="腾讯技术工程" type="rss" xmlUrl="https://wechat2rss.bestblogs.dev/feed/def.xml"/>
some prose without URL is skipped here`

	cands := extractDeterministic(seed)
	require.Len(t, cands, 2)
	assert.Equal(t, "人人都是产品经理", cands[0].Name)
	assert.Equal(t, "https://wechat2rss.bestblogs.dev/feed/abc.xml", cands[0].Feed)
	assert.Equal(t, "腾讯技术工程", cands[1].Name)
	assert.Equal(t, "https://wechat2rss.bestblogs.dev/feed/def.xml", cands[1].Feed)
}
