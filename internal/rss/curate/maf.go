package curate

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/microsoft/agent-framework-go/agent"
	"github.com/microsoft/agent-framework-go/provider/openaiprovider"
	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"

	"github.com/xbpk3t/docs-alfred/pkg/ai"
)

// openAIClient builds the provider client for the SAME endpoint/credentials as
// the rest of the repo (pkg/ai env conventions), so MAF does not get its own
// model/key story. BaseURL/model come from the curate Config (env-driven).
func openAIClient(cfg *Config) openai.Client {
	base := cfg.BaseURL
	if base == "" {
		base = ai.DefaultConfig().BaseURL
	}
	// Option take precedence and honor the LLM_AxonHub fallback from pkg/ai.
	// MaxRetries(0) disables the SDK's own retry so retry/backoff is owned by a
	// single, explicit place (run); the SDK would otherwise ignore a
	// Retry-After > 1min and fight our per-attempt timeout on slow models.
	return openai.NewClient(
		option.WithAPIKey(ai.DefaultConfig().APIKey),
		option.WithBaseURL(base),
		option.WithMaxRetries(0),
	)
}

// newAgent builds a Chat-Completions-backed MAF agent (validated by the spike).
// Each of the three AI steps uses its own instructions + structured schema.
func newAgent(cfg *Config, name, instructions string) *agent.Agent {
	return openaiprovider.NewChatCompletionsAgent(
		openAIClient(cfg),
		openaiprovider.AgentConfig{
			Model:        cfg.Model,
			Instructions: instructions,
			Config: agent.Config{
				Name: name,
			},
		},
	)
}

// run invokes the agent and unmarshals the model's JSON response into *out
// with a single plain call. We deliberately skip MAF's strict JSON-schema mode
// (WithStructuredOutput): on this endpoint some models (e.g. glm-5.2) ignore
// response_format and force a second call, doubling latency/cost for every
// candidate. One call + a fence-tolerant parse (see jsonUnmarshal / the
// bare-array-tolerant UnmarshalJSON on each batch type) is faster and works for
// every model we run (hy3, deepseek-v4-flash, glm-5.2). T should carry json tags.
//
//nolint:gocyclo // one deliberate retry/backoff/parse choke point for all LLM calls
func run[T any](ctx context.Context, cfg *Config, a *agent.Agent, prompt string) (T, error) {
	var zero T
	var out T

	timeout := cfg.AITimeout
	if timeout <= 0 {
		timeout = defaultAITimeout
	}
	retries := cfg.AIRetries
	if retries < 0 {
		retries = 0
	}
	backoff := time.Duration(cfg.AIRetryInitialMs) * time.Millisecond
	if backoff <= 0 {
		backoff = defaultAIRetryBackoff
	}
	budget := cfg.AIRetryBudget
	if budget <= 0 {
		budget = defaultAIRetryBudget
	}

	// Per-call timing + retry log at the single choke point every LLM call
	// (resolve / classify / curator) runs through. When fan-out drives several
	// calls at once, overlapping "start" lines prove real concurrency.
	who := a.Name()
	deadline := time.Now().Add(budget)
	attempts := 0
	var last error
	for {
		attempts++
		aCtx, cancel := context.WithTimeout(ctx, timeout)
		start := time.Now()
		slog.Info("curate ai call start", "agent", who, "attempt", attempts, "ms_epoch", start.UnixMilli())
		resp, err := a.RunText(aCtx, prompt).Collect()
		cancel()
		slog.Info("curate ai call done", "agent", who, "attempt", attempts, "elapsed_ms", time.Since(start).Milliseconds())
		if err == nil {
			if uerr := jsonUnmarshal(resp.String(), &out); uerr != nil {
				return zero, fmt.Errorf("structured parse: %w", uerr)
			}
			return out, nil
		}
		last = err
		if attempts > retries || !retryable(err) || time.Now().After(deadline) {
			break
		}
		wait := backoff
		if ra := nextRetryAfter(err); ra > wait {
			wait = ra
		}
		if remaining := time.Until(deadline); wait > remaining {
			wait = remaining
		}
		if wait > 0 {
			slog.Warn("curate ai call retry", "agent", who, "attempt", attempts, "err", err, "backoff_ms", wait.Milliseconds())
			select {
			case <-time.After(wait):
			case <-ctx.Done():
				return zero, ctx.Err()
			}
		}
		backoff *= 2
	}

	return zero, fmt.Errorf("ai call failed after %d attempt(s): %w", attempts, last)
}

// retryable reports whether err is a transient server/rate/network status we
// should retry: 408, 429, any 5xx (including Cloudflare 52x such as 524), and
// transport-timeout errors. Context deadline exceed is NOT retryable here (the
// caller's ctx governs that).
func retryable(err error) bool {
	var apiErr *openai.Error
	if errors.As(err, &apiErr) {
		switch {
		case apiErr.StatusCode == http.StatusRequestTimeout, // 408
			apiErr.StatusCode == http.StatusTooManyRequests, // 429
			apiErr.StatusCode >= 500:                        // 5xx incl. 502/503/524/529
			return true
		}
		return false
	}
	var nerr net.Error
	if errors.As(err, &nerr) {
		return nerr.Timeout()
	}

	// Common transient transport/stream errors from chat completions. Both the
	// sentinels and (as a safety net) the rendered text of openai-go/client
	// errors are matched, because a premature EOF surfaces differently depending
	// on whether it's wrapped.
	isEOF := errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF)
	isReset := errors.Is(err, syscall.ECONNRESET) || errors.Is(err, syscall.EPIPE)
	if isEOF {
		return true
	}
	if isReset {
		return true
	}
	msg := err.Error()
	return strings.Contains(msg, "connection reset") ||
		strings.Contains(msg, "unexpected EOF") ||
		strings.Contains(msg, "broken pipe") ||
		strings.Contains(msg, "TLS handshake")
}

// nextRetryAfter reads a Retry-After header (seconds) from an API error, which
// servers like Cloudflare send for the 524/529 it wants you to back off for.
func nextRetryAfter(err error) time.Duration {
	var apiErr *openai.Error
	if errors.As(err, &apiErr) && apiErr.Response != nil {
		if ra := apiErr.Response.Header.Get("Retry-After"); ra != "" {
			if s, perr := strconv.Atoi(ra); perr == nil && s > 0 {
				return time.Duration(s) * time.Second
			}
		}
	}
	return 0
}

// jsonBytes strips markdown fences / preamble and isolates the outermost JSON
// value, then validates it. Deterministic — the AI only produces the bytes.
func jsonBytes(raw string) ([]byte, error) {
	t := strings.TrimSpace(raw)
	if strings.HasPrefix(t, "```") {
		if i := strings.IndexByte(t, '\n'); i >= 0 {
			t = strings.TrimSpace(t[i+1:])
		} else {
			t = strings.Trim(t, "`\n ")
		}
		if e := strings.LastIndex(t, "```"); e >= 0 {
			t = strings.TrimSpace(t[:e])
		}
	}
	t = strings.TrimSpace(t)

	// Whole thing valid? use it.
	if t != "" && json.Valid([]byte(t)) {
		return []byte(t), nil
	}
	// else: the model wrapped the object in prose — take the outermost {..}.
	if i := strings.IndexByte(t, '{'); i >= 0 {
		if j := strings.LastIndexByte(t, '}'); j > i {
			if cand := t[i : j+1]; json.Valid([]byte(cand)) {
				return []byte(cand), nil
			}
		}
	}

	return nil, fmt.Errorf("no parseable JSON in model output")
}

func jsonUnmarshal(t string, out any) error {
	data, err := jsonBytes(t)
	if err != nil {
		return err
	}

	return json.Unmarshal(data, out)
}

// unmarshalObjectOrList accepts a model's batch output in EITHER the wrapped
// form ({"<field>":[ ... ]}) or a bare array. `list` must be a *[]T. This
// makes the batch types tolerant of models (like glm-5.2) that return the
// unwrapped array.
func unmarshalObjectOrList(data []byte, obj, list any) error {
	t := strings.TrimSpace(string(data))
	if t == "" {
		return errors.New("empty model output")
	}
	if t[0] == '{' {
		return json.Unmarshal([]byte(t), obj)
	}

	return json.Unmarshal([]byte(t), list)
}
