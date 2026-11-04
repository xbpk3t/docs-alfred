package ai

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Jitter means only bounds are assertable: equal jitter puts the wait in
// [exponential/2, exponential), capped at maxBackoff.
func TestRetryWaitJitterBounds(t *testing.T) {
	for _, tc := range []struct {
		name             string
		attempt          int
		wantMin, wantMax time.Duration
	}{
		{"first retry", 1, time.Second, 2 * time.Second},
		{"third retry", 3, 4 * time.Second, 8 * time.Second},
		{"capped", 10, maxBackoff / 2, maxBackoff},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for range 20 {
				wait := retryWait(defaultBackoff, tc.attempt)
				assert.GreaterOrEqual(t, wait, tc.wantMin)
				assert.Less(t, wait, tc.wantMax)
			}
		})
	}
}

func TestRetryWaitExplicitBase(t *testing.T) {
	// A caller-provided base replaces the default, still jittered and capped.
	wait := retryWait(100*time.Millisecond, 1)
	assert.GreaterOrEqual(t, wait, 50*time.Millisecond)
	assert.Less(t, wait, 100*time.Millisecond)
}

func TestSetGlobalRPMShapesRate(t *testing.T) {
	t.Cleanup(func() { SetGlobalRPM(0) })

	SetGlobalRPM(0)
	require.NoError(t, acquireAI(context.Background()), "unset limiter must be a no-op")

	// 600 RPM = 10 tokens/s, burst 5: the first burst passes instantly, the
	// next token needs ~100ms of refill.
	SetGlobalRPM(600)
	start := time.Now()
	for range aiBurst {
		require.NoError(t, acquireAI(context.Background()))
	}
	assert.Less(t, time.Since(start), 50*time.Millisecond, "burst must be immediate")

	err := acquireAI(context.Background())
	require.NoError(t, err)
	assert.GreaterOrEqual(t, time.Since(start), 50*time.Millisecond, "post-burst call must wait for refill")
}

func TestSetGlobalRPMZeroRemovesCap(t *testing.T) {
	SetGlobalRPM(600)
	SetGlobalRPM(0)
	start := time.Now()
	for range 20 {
		require.NoError(t, acquireAI(context.Background()))
	}
	assert.Less(t, time.Since(start), 50*time.Millisecond, "removing the cap must unblock immediately")
}
