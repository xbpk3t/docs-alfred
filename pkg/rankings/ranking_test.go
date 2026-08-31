package rankings

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------- 端到端：热度排行 ----------

func TestRankingEndToEnd(t *testing.T) {
	ctx := context.Background()
	storage := NewMemStorage()
	calc := NewArticleHeatCalc()
	decay := &NoDecay{} // 先不衰减，验证基础排序
	window := &AllTimeWindow{Name: "test"}
	r := NewRanking("heat", "rank:test", calc, decay, window, storage)

	now := time.Now()
	require.NoError(t, r.Submit(ctx, Item{
		Member:    "a",
		Fields:    map[string]float64{"views": 100},
		Timestamp: now,
	}))
	require.NoError(t, r.Submit(ctx, Item{
		Member:    "b",
		Fields:    map[string]float64{"views": 50, "likes": 20},
		Timestamp: now,
	}))

	t.Run("topN desc", func(t *testing.T) {
		top, err := r.TopN(ctx, 2)
		require.NoError(t, err)
		require.Len(t, top, 2)
		// a=100, b=50+60=110 → b first
		assert.Equal(t, "b", top[0].Member)
		assert.Equal(t, 1, top[0].Rank)
		assert.Equal(t, "a", top[1].Member)
		assert.Equal(t, 2, top[1].Rank)
	})

	t.Run("myRank", func(t *testing.T) {
		rank, err := r.MyRank(ctx, "a")
		require.NoError(t, err)
		assert.Equal(t, 2, rank)
	})

	t.Run("update overwrites score", func(t *testing.T) {
		// a 追加 views，应超越 b
		require.NoError(t, r.Submit(ctx, Item{
			Member:    "a",
			Fields:    map[string]float64{"views": 1000},
			Timestamp: now,
		}))
		rank, err := r.MyRank(ctx, "a")
		require.NoError(t, err)
		assert.Equal(t, 1, rank)
	})
}

// ---------- 端到端：内存存储多窗口隔离 ----------

func TestMemStorageWindowIsolation(t *testing.T) {
	s := NewMemStorage()
	ctx := context.Background()

	require.NoError(t, s.Update(ctx, "rank:d1", "m1", 10.0))
	require.NoError(t, s.Update(ctx, "rank:d2", "m1", 20.0))

	t.Run("keys independent", func(t *testing.T) {
		top1, err := s.TopN(ctx, "rank:d1", 10)
		require.NoError(t, err)
		require.Len(t, top1, 1)
		assert.Equal(t, 10.0, top1[0].Score)

		top2, err := s.TopN(ctx, "rank:d2", 10)
		require.NoError(t, err)
		require.Len(t, top2, 1)
		assert.Equal(t, 20.0, top2[0].Score)
	})

	t.Run("missing member rank = -1", func(t *testing.T) {
		rank, err := s.Rank(ctx, "rank:d1", "ghost")
		require.NoError(t, err)
		assert.Equal(t, -1, rank)
	})

	t.Run("empty key topN = nil", func(t *testing.T) {
		top, err := s.TopN(ctx, "rank:empty", 10)
		require.NoError(t, err)
		assert.Nil(t, top)
	})
}
