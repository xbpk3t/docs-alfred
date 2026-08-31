package rankings

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------- ScoreCalculator 测试 ----------

func TestArticleHeatCalc(t *testing.T) {
	t.Run("weighted sum", func(t *testing.T) {
		calc := NewArticleHeatCalc()
		item := Item{
			Member:    "m1",
			Fields:    map[string]float64{"views": 100, "likes": 5, "comments": 3},
			Timestamp: time.Now(),
		}
		score, err := calc.Calculate(item)
		require.NoError(t, err)
		// 100*1 + 5*3 + 3*5 = 130
		assert.Equal(t, 130.0, score)
	})

	t.Run("empty fields = zero", func(t *testing.T) {
		calc := NewArticleHeatCalc()
		score, err := calc.Calculate(Item{Member: "m2", Timestamp: time.Now()})
		require.NoError(t, err)
		assert.Equal(t, 0.0, score)
	})
}

// validateScore 是库里唯一的浮点精度守卫，直接单测覆盖。
func TestValidateScore(t *testing.T) {
	t.Run("within limit passes", func(t *testing.T) {
		assert.NoError(t, validateScore(safeScoreLimit, "test"))
		assert.NoError(t, validateScore(0, "test"))
		assert.NoError(t, validateScore(-safeScoreLimit, "test"))
	})

	t.Run("beyond limit rejects", func(t *testing.T) {
		// float64 在 2^53 之上只表示偶数：+1 会舍回 2^53 本体，逃过守卫。
		// 用可真实表示的溢位值（+128）验证守卫生效。
		over := safeScoreLimit + 128
		assert.Error(t, validateScore(over, "test"))
		assert.Error(t, validateScore(-over, "test"))
	})
}
