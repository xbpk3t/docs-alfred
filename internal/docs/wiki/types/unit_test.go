package types

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDetectUnit(t *testing.T) {
	tests := []struct {
		name string
		url  string
		want UnitType
	}{
		{"zhihu question", "https://www.zhihu.com/question/35129528", UnitQnA},
		// Answer permalinks also contain "/question/", so the answer check
		// must win — otherwise a single answer is digested as a whole thread.
		{"zhihu answer permalink", "https://www.zhihu.com/question/35129528/answer/123456789", UnitAnswer},
		{"zhihu bare-domain answer permalink", "https://zhihu.com/question/35129528/answer/123", UnitAnswer},
		{"zhihu bare domain question", "https://zhihu.com/question/35129528", UnitQnA},
		{"zhuanlan article stays article", "https://zhuanlan.zhihu.com/p/123456789", UnitArticle},
		{"zhihu non-question path", "https://www.zhihu.com/hot", UnitArticle},
		{"regular article", "https://example.com/article", UnitArticle},
		{"bilibili video", "https://www.bilibili.com/video/BV1xx", UnitArticle},
		{"invalid url", "not-a-url", UnitArticle},
		{"empty url", "", UnitArticle},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, DetectUnit(tt.url))
		})
	}
}

// Guard: detect must never require network or panic on junk input.
func TestDetectUnitEdgeCases(t *testing.T) {
	require.NotPanics(t, func() {
		assert.Equal(t, UnitArticle, DetectUnit("://"))
	})
}
