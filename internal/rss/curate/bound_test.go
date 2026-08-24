package curate

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRunBoundedParallel proves the fan-out is actually concurrent: with fixed
// per-task latency, lowering the limit should stretch wall-clock ~linearly,
// while raising it collapses wall-clock to ~latency. This is the measurable
// counterpart to `curate ai call start/done` log overlap for real LLM calls.
func TestRunBoundedParallel(t *testing.T) {
	const (
		tasks   = 6
		latency = 80 * time.Millisecond
	)
	shoot := func(limit int) time.Duration {
		var ran atomic.Int64
		start := time.Now()
		err := runBounded(context.Background(), tasks, limit, func(context.Context, int) error {
			time.Sleep(latency)
			ran.Add(1)
			return nil
		})
		require.NoError(t, err)
		require.Equal(t, int64(tasks), ran.Load())
		return time.Since(start)
	}

	serial := shoot(1)
	parallel := shoot(tasks) // unlimited up to tasks

	// 6×80ms: serial ≈480ms, parallel ≈80ms → parallel must be far under
	// serial (allow machine noise by asserting < 45% of serial).
	assert.Less(t, parallel, serial*45/100, "fan-out should be ~N× faster")
}

func TestRunBoundedReturnsFirstError(t *testing.T) {
	err := runBounded(context.Background(), 4, 4, func(_ context.Context, i int) error {
		if i == 2 {
			return assert.AnError
		}
		return nil
	})
	assert.ErrorIs(t, err, assert.AnError)
}

func TestRunBoundedEmpty(t *testing.T) {
	require.NoError(t, runBounded(context.Background(), 0, 3, func(context.Context, int) error { return nil }))
}
