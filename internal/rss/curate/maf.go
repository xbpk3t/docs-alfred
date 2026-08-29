package curate

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/microsoft/agent-framework-go/agent"

	"github.com/xbpk3t/docs-alfred/pkg/ai"
)

// aiConfig maps curate's config onto the shared pkg/ai client config, keeping
// model/base local and the API key env-driven (LLM_AxonHub fallback), so MAF
// does not get its own model/key story. Model/BaseURL are already resolved from
// env by applyModelDefaults during LoadConfig.
func aiConfig(cfg *Config) *ai.ClientConfig {
	return &ai.ClientConfig{Model: cfg.Model, BaseURL: cfg.BaseURL}
}

// newAgent builds a Chat-Completions-backed MAF agent (the shared pkg/ai
// factory, whose client is cached per endpoint and reused across the whole
// batch). Each curate step uses its own instructions + JSON schema.
func newAgent(cfg *Config, name, instructions string) *agent.Agent {
	return ai.NewChatAgent(aiConfig(cfg), name, instructions)
}

// run invokes the agent and unmarshals the model's JSON response into *out with
// a single plain call. We deliberately skip MAF's strict JSON-schema mode
// (WithStructuredOutput): on this endpoint some models (e.g. glm-5.2) ignore
// response_format and force a second call, especially for every candidate. One
// call + a fence-tolerant parse (see jsonUnmarshal / the bare-array-tolerant
// UnmarshalJSON on each batch type) is faster and works for every model we run
// (hy3, deepseek-v4-flash, glm-5.2). T should carry json tags.
//
// The retry/backoff/timeout loop, the per-call "ai call start/done" logs and the
// Retry-After handling all live in pkg/ai (ai.RunJSON); curate only supplies the
// config values and its own JSON decode. A parse failure returned by decode is
// treated as terminal (never retried), matching the historical behavior.
func run[T any](ctx context.Context, cfg *Config, a *agent.Agent, prompt string) (T, error) {
	return ai.RunJSON[T](ctx, a, prompt, runOpts(cfg), a.Name(), ai.RunOptions{
		Timeout: cfg.AITimeout,
		Retries: cfg.AIRetries,
		Backoff: time.Duration(cfg.AIRetryInitialMs) * time.Millisecond,
		Budget:  cfg.AIRetryBudget,
	}, func(text string) (T, error) {
		var out T
		if uerr := jsonUnmarshal(text, &out); uerr != nil {
			var zero T
			return zero, fmt.Errorf("structured parse: %w", uerr)
		}
		return out, nil
	})
}

// runOpts builds the per-attempt agent options. Effort trimming/defaulting is
// owned by ai.UseEffort.
func runOpts(cfg *Config) []agent.Option {
	return []agent.Option{ai.UseEffort(cfg.Effort)}
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
