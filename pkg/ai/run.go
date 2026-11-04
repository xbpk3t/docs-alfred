package ai

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"strings"
	"time"

	mafagent "github.com/microsoft/agent-framework-go/agent"
)

// RunOptions controls the shared, non-streaming retry loop every AI call runs
// through. Zero fields fall back to the repo-wide defaults (60s per-attempt
// timeout, 2 extra attempts, 2s jittered exponential backoff capped at 30s,
// unlimited wall clock).
type RunOptions struct {
	// Timeout is the per-attempt deadline; ≤0 uses DefaultAITimeout.
	Timeout time.Duration
	// Retries is the number of extra attempts after the first. <0 forces 0;
	// 0 (unset) uses defaultRetries.
	Retries int
	// Backoff is the initial backoff before the first retry, doubling each
	// further retry; ≤0 uses 2s.
	Backoff time.Duration
	// Budget caps the whole loop's wall clock (attempts + sleeps); ≤0 is
	// unlimited.
	Budget time.Duration
}

// defaultRetries is how many extra attempts (after the first) a zero RunOptions
// gives — a total of 3 calls, matching the historical runTextContext default.
const defaultRetries = 2

const (
	// defaultBackoff is the wait before the first retry; it doubles on each
	// further retry. 2s gives a rate-limited window room to drain — 500ms just
	// machine-gunned the gateway three times in a row.
	defaultBackoff = 2 * time.Second
	// maxBackoff caps the exponential growth so a retry never hangs a worker
	// for minutes inside a per-URL budget.
	maxBackoff = 30 * time.Second
)

// retryWait is the sleep before the next retry: exponential backoff with equal
// jitter — half fixed, half random — so concurrent failures don't re-collide on
// the same tick. A server-provided Retry-After (applied by the caller) always
// wins when longer. attempt is 1-based (the attempt that just failed).
func retryWait(base time.Duration, attempt int) time.Duration {
	wait := base << (attempt - 1)
	if wait <= 0 || wait > maxBackoff {
		wait = maxBackoff
	}
	half := wait / 2
	if half <= 0 {
		return wait
	}

	//nolint:gosec // retry jitter needs spread, not unpredictability
	return half + rand.N(half)
}

// run drives the bounded retry/backoff loop around a single non-streaming agent
// call. This is the one choke point every retry-capable AI call (curate's
// resolve/classify/curator, rss/linear/wiki call sites) flows through, so
// timeout / retryable / Retry-After / backoff semantics live in exactly one
// place. `try` turns a successful (non-error) response into the final result; a
// non-nil error from `try` is treated as terminal — a parse failure is never
// retried. `who`, when non-empty, labels start/done/retry log lines (curate
// passes the agent name); empty keeps the call silent.
//
//nolint:gocognit,gocyclo,revive // one deliberate retry/backoff/parse choke point for all AI calls
func run[T any](ctx context.Context, a *mafagent.Agent, prompt string, opts []mafagent.Option,
	who string, ro RunOptions, try func(text string) (T, error)) (T, error) {
	var zero T

	timeout := ro.Timeout
	if timeout <= 0 {
		timeout = DefaultAITimeout
	}
	retries := ro.Retries
	switch {
	case retries < 0:
		retries = 0
	case retries == 0:
		retries = defaultRetries
	}
	backoff := ro.Backoff
	if backoff <= 0 {
		backoff = defaultBackoff
	}
	var deadline time.Time
	if ro.Budget > 0 {
		deadline = time.Now().Add(ro.Budget)
	}

	attempts := 0
	var last error
	for {
		attempts++
		if attempts > retries+1 {
			break
		}
		// One token per HTTP attempt, acquired before the per-attempt deadline
		// starts so waiting for a slot never eats into the call's own budget.
		if err := acquireAI(ctx); err != nil {
			return zero, err
		}
		aCtx, cancel := context.WithTimeout(ctx, timeout)
		start := time.Now()
		if who != "" {
			slog.Info("ai call start", "agent", who, "attempt", attempts, "ms_epoch", start.UnixMilli())
		}
		resp, err := a.RunText(aCtx, prompt, opts...).Collect()
		cancel()
		if who != "" {
			slog.Info("ai call done", "agent", who, "attempt", attempts, "elapsed_ms", time.Since(start).Milliseconds())
		}
		if err == nil {
			out, werr := try(resp.String())
			if werr != nil {
				return zero, werr
			}
			return out, nil
		}
		last = err
		if attempts > retries || !IsRetryable(err) ||
			(!deadline.IsZero() && time.Now().After(deadline)) || ctx.Err() != nil {
			break
		}
		wait := retryWait(backoff, attempts)
		if ra := RetryAfter(err); ra > wait {
			wait = ra
		}
		if !deadline.IsZero() {
			if remaining := time.Until(deadline); wait > remaining {
				wait = remaining
			}
		}
		if wait > 0 {
			if who != "" {
				slog.Warn("ai call retry", "agent", who, "attempt", attempts, "err", err, "backoff_ms", wait.Milliseconds())
			}
			select {
			case <-time.After(wait):
			case <-ctx.Done():
				return zero, ctx.Err()
			}
		}
	}
	return zero, fmt.Errorf("AI call failed after %d attempt(s): %w", attempts, last)
}

// RunJSON runs the shared retry loop and parses the model's fence-tolerant JSON
// response into T via decode. decode is never retried: a parse error aborts the
// loop immediately. `who` (empty to stay silent) labels start/done/retry logs.
func RunJSON[T any](ctx context.Context, a *mafagent.Agent, prompt string, opts []mafagent.Option, who string,
	ro RunOptions, decode func(text string) (T, error)) (T, error) {
	return run[T](ctx, a, prompt, opts, who, ro, decode)
}

// runTextContext executes a single chatCompletions-agent call. It is the
// plainText half of the shared run loop: no JSON parsing, just a trimmed
// non-empty string (empty output is treated as a terminal error). Effort is
// defaulted inside UseEffort, matching every other AI caller.
func runTextContext(ctx context.Context, cfg *ClientConfig, system, user string) (string, error) {
	if strings.TrimSpace(user) == "" {
		return "", errors.New("empty user prompt")
	}
	a := NewChatAgent(cfg, "chat", system)
	opts := []mafagent.Option{UseEffort(cfg.Effort)}
	ro := RunOptions{Timeout: cfg.Timeout}
	return run[string](ctx, a, user, opts, "", ro, func(resp string) (string, error) {
		s := strings.TrimSpace(resp)
		if s == "" {
			return "", errEmpty
		}
		return s, nil
	})
}
