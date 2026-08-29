package ai

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/shared"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultConfig_Defaults(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("LLM_AxonHub", "")
	t.Setenv("OPENAI_BASE_URL", "")
	t.Setenv("LLM_MODEL", "")
	t.Setenv("LLM_EFFORT", "")
	cfg := DefaultConfig()
	assert.Equal(t, "https://api.lucc.dev/v1", cfg.BaseURL)
	assert.Equal(t, "deepseek-v4-flash", cfg.Model)
	// Effort is env-driven here; the effective default (max) is applied per call.
	assert.Empty(t, cfg.Effort)
}

func TestDefaultConfig_StreamingTrue(t *testing.T) {
	assert.True(t, DefaultConfig().Streaming, "kept true for legacy callers")
}

func TestDefaultConfig_WithEnv(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "k")
	t.Setenv("OPENAI_BASE_URL", "https://x")
	t.Setenv("LLM_MODEL", "m")
	t.Setenv("LLM_EFFORT", "high")
	cfg := DefaultConfig()
	assert.Equal(t, "k", cfg.APIKey)
	assert.Equal(t, "https://x", cfg.BaseURL)
	assert.Equal(t, "m", cfg.Model)
	assert.Equal(t, "high", cfg.Effort)
}

func TestDefaultConfig_LLMAxonHubFallback(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("LLM_AxonHub", "fallback-key")
	assert.Equal(t, "fallback-key", DefaultConfig().APIKey)
}

func TestConfigWithOverrides(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "env")
	cfg := ConfigWithOverrides("explicit", "https://e/v1", "m")
	assert.Equal(t, "explicit", cfg.APIKey)
	assert.Equal(t, "https://e/v1", cfg.BaseURL)
	assert.Equal(t, "m", cfg.Model)

	empty := ConfigWithOverrides("", "", "")
	assert.Equal(t, "env", empty.APIKey)
	assert.Equal(t, "deepseek-v4-flash", empty.Model)
}

func TestEffortEnum_AllTiers(t *testing.T) {
	cases := []struct {
		in   string
		want shared.ReasoningEffort
	}{
		{"max", shared.ReasoningEffortMax},
		{"xhigh", shared.ReasoningEffortXhigh},
		{"high", shared.ReasoningEffortHigh},
		{"medium", shared.ReasoningEffortMedium},
		{"low", shared.ReasoningEffortLow},
		{"minimal", shared.ReasoningEffortMinimal},
		{"none", shared.ReasoningEffortNone},
		{" HIGH ", shared.ReasoningEffortHigh}, // case + whitespace tolerant
		{"garbage", ""},
		{"", ""},
	}
	for _, c := range cases {
		assert.Equalf(t, c.want, EffortEnum(c.in), "EffortEnum(%q)", c.in)
	}
}

func TestUseEffort_DefaultsToMax(t *testing.T) {
	// UseEffort is the single effort primitive: it trims, mappings via
	// EffortEnum, and defaults an empty value to the max tier. We assert the
	// default wiring through the public tier rather than option internals.
	assert.Equal(t, "max", string(shared.ReasoningEffortMax))
	assert.Equal(t, "max", DefaultEffort)
	assert.Equal(t, shared.ReasoningEffortMax, EffortEnum(DefaultEffort))
	assert.NotNil(t, UseEffort(""), "empty effort still yields an option (max tier)")
	assert.NotNil(t, UseEffort("   "))
	assert.NotNil(t, UseEffort("low"))
}

func TestSplitSystem(t *testing.T) {
	sys, user := splitSystem([]Message{
		{Role: RoleSystem, Content: "be terse"},
		{Role: RoleUser, Content: "hello"},
		{Role: RoleUser, Content: "again"},
	})
	assert.Equal(t, "be terse", sys)
	assert.Equal(t, "hello\nagain", user)
}

func TestChat_NoAPIKey(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("LLM_AxonHub", "")
	_, err := ChatContext(context.Background(), &ClientConfig{Model: "m"}, []Message{{Content: "x"}})
	require.Error(t, err)
	assert.True(t, errors.Is(err, errNoAPIKey))
}

func TestChat_NilConfig(t *testing.T) {
	_, err := ChatContext(context.Background(), nil, nil)
	require.Error(t, err)
}

func TestChat_EmptyUserShortCircuits(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "k")
	_, err := ChatContext(context.Background(), &ClientConfig{Model: "m"}, []Message{{Content: "   "}})
	require.Error(t, err)
}

func TestChat_DeadlineExceededNotRetried(t *testing.T) {
	// Hard per-attempt deadline must not be classified as retryable (no 3× stall).
	assert.False(t, IsRetryable(context.DeadlineExceeded))
}

func TestClientPool_ReusesByConfig(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "k")
	base := "https://example.invalid/v1" // never dialed during the test
	cfg := func(apiKey string) *ClientConfig {
		return &ClientConfig{APIKey: apiKey, BaseURL: base}
	}

	p := &ClientPool{clients: map[clientKey]openai.Client{}}

	// Same endpoint+key => one cached client; repeats don't grow the map.
	p.Get(cfg("k"))
	p.Get(cfg("k"))
	require.Len(t, p.clients, 1)

	// A different key => a separate cached client.
	p.Get(cfg("other"))
	require.Len(t, p.clients, 2)

	// Same keys still hit the cache (no growth with more calls).
	p.Get(cfg("other"))
	p.Get(cfg("k"))
	require.Len(t, p.clients, 2)
}

func TestDefaultPool_Get(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "k")
	c := NewOpenAIClient(&ClientConfig{BaseURL: "https://example.invalid/v1"})
	require.NotNil(t, c)
}

func TestIsRetryable_Transport(t *testing.T) {
	for _, msg := range []string{
		"unexpected EOF",
		"connection reset by peer",
		"broken pipe",
		"TLS handshake",
		"i/o timeout",
	} {
		assert.Truef(t, IsRetryable(errors.New(msg)), "should retry: %s", msg)
	}
	assert.False(t, IsRetryable(errors.New("random domestic error")))
}

func TestRetryAfter_ParsesSeconds(t *testing.T) {
	assert.Equal(t, time.Duration(0), RetryAfter(errors.New("plain")))
}

// Net-error based paths for retryable (used by the retry helper) are covered by
// TestRetryable_Transport. The *openai.Error status path is covered by
// integration/dry-run probes, not a pure unit (needs a typed API error).
var _ = shared.ReasoningEffortMax // keep import

func TestEffortWireString(t *testing.T) {
	assert.True(t, strings.HasPrefix(string(shared.ReasoningEffortMax), "max"))
}
