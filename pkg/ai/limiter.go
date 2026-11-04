package ai

import (
	"context"
	"sync"

	"golang.org/x/time/rate"
)

// aiBurst is the token-bucket burst size: how many AI calls may fire at once
// before the refill rate starts spacing them. Small on purpose — gateways like
// the ones behind api.lucc.dev count per-minute volume, not per-second spacing,
// so bursts are cheap and the refill rate does the real shaping.
const aiBurst = 5

// The limiter caps AI request rate process-wide so a high fetch concurrency
// cannot stampede a rate-limited gateway (1302-style account RPM limits).
// Nil until SetGlobalRPM is called: unset means unlimited, and every CLI that
// never configures it keeps today's behavior.
var (
	limiterMu sync.Mutex
	limiter   *rate.Limiter
)

// SetGlobalRPM caps all AI calls in this process to rpm requests per minute,
// shared across every goroutine and call site (token bucket, burst aiBurst).
// It is a client-side shaping knob: set it below the gateway's real account
// limit so 429s never happen. rpm ≤ 0 removes the cap. Safe to call again —
// the latest value wins.
func SetGlobalRPM(rpm int) {
	limiterMu.Lock()
	defer limiterMu.Unlock()
	if rpm <= 0 {
		limiter = nil

		return
	}
	limiter = rate.NewLimiter(rate.Limit(float64(rpm)/60.0), aiBurst)
}

// acquireAI blocks until a request slot is available or ctx is done. A nil
// limiter (no SetGlobalRPM) is a no-op.
func acquireAI(ctx context.Context) error {
	limiterMu.Lock()
	l := limiter
	limiterMu.Unlock()
	if l == nil {
		return nil
	}

	return l.Wait(ctx)
}
