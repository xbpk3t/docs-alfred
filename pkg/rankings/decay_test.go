package rankings

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNoDecay(t *testing.T) {
	d := &NoDecay{}
	score := d.Apply(100.0, 24*time.Hour)
	assert.Equal(t, 100.0, score, "NoDecay should return raw score unchanged")
}

func TestExponentialDecay(t *testing.T) {
	d := &ExponentialDecay{Lambda: 0.01}

	t.Run("zero elapsed = no decay", func(t *testing.T) {
		score := d.Apply(100.0, 0)
		assert.InDelta(t, 100.0, score, 1e-9)
	})

	t.Run("positive elapsed decays", func(t *testing.T) {
		score := d.Apply(100.0, time.Hour)
		assert.Less(t, score, 100.0, "score should decay with elapsed time")
		assert.Greater(t, score, 0.0)
	})

	t.Run("bigger lambda decays faster", func(t *testing.T) {
		d1 := &ExponentialDecay{Lambda: 0.01}
		d2 := &ExponentialDecay{Lambda: 0.1}
		s1 := d1.Apply(100.0, time.Hour)
		s2 := d2.Apply(100.0, time.Hour)
		assert.Less(t, s2, s1, "higher lambda should decay faster")
	})
}

func TestLinearDecay(t *testing.T) {
	t.Run("zero elapsed = no decay", func(t *testing.T) {
		d := &LinearDecay{DailyRate: 0.1}
		assert.Equal(t, 100.0, d.Apply(100.0, 0))
	})

	t.Run("linear decay by days", func(t *testing.T) {
		d := &LinearDecay{DailyRate: 0.1}
		score := d.Apply(100.0, 24*time.Hour)
		assert.InDelta(t, 90.0, score, 1e-9, "one day at 10%%/day = 90")
	})

	t.Run("floor at zero", func(t *testing.T) {
		d := &LinearDecay{DailyRate: 0.1}
		score := d.Apply(100.0, 20*24*time.Hour) // 20 days > 10 day zero point
		assert.Equal(t, 0.0, score, "score should clamp to 0 after fully decayed")
	})
}
