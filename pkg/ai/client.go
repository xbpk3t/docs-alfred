package ai

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	mafagent "github.com/microsoft/agent-framework-go/agent"
	"github.com/microsoft/agent-framework-go/provider/openaiprovider"
	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
	"github.com/openai/openai-go/v3/shared"
)

// defaultBaseURL is the gateway endpoint shared across all CLI defaults. The
// literal lives here once; config loaders and ClientPool.Get fall back to it.
const defaultBaseURL = "https://api.lucc.dev/v1"

// ClientPool reuses one openai.Client per (endpoint, key) instead of rebuilding
// its HTTP transport + connection pool on every call. Digest/loop call sites fan
// out many AI calls; openai-client values are safe for concurrent use, so a
// handful of long-lived clients beat per-URL client churn. openai.NewClient
// builds its own transport (with a response-header timeout) when none is given,
// and idle connections are reclaimed at process exit, so no Close/drain
// machinery is needed here.
type ClientPool struct {
	clients map[clientKey]openai.Client
	mu      sync.Mutex
}

// clientKey identifies the endpoint/credentials pair that determines a client.
type clientKey struct {
	baseURL string
	apiKey  string
}

// defaultPool is the process-wide pool used by NewOpenAIClient.
var defaultPool = &ClientPool{clients: map[clientKey]openai.Client{}}

// Get returns the cached client for cfg, building and caching one on first use.
func (p *ClientPool) Get(cfg *ClientConfig) openai.Client {
	base := cfg.BaseURL
	if base == "" {
		base = defaultBaseURL
	}
	key := clientKey{baseURL: base, apiKey: lookupKey(cfg.APIKey)}
	p.mu.Lock()
	defer p.mu.Unlock()
	if c, ok := p.clients[key]; ok {
		return c
	}
	c := openai.NewClient(
		option.WithAPIKey(key.apiKey),
		option.WithBaseURL(base),
		option.WithMaxRetries(0),
	)
	p.clients[key] = c
	return c
}

// DefaultAITimeout is the fallback call deadline when no Timeout is configured.
// 60s is generous enough for short prompts (linear2nl, rss2nl) without making
// hangs unbearable. Wiki digest uses its own perURLTimeout ctx; ccx sets 200s.
//
// Note: the MAF path here is NON-streaming. Reasoning / long-thinking models
// (deepseek-v4-flash at effort=max) can take far longer than 60s; callers that
// need headroom must set ClientConfig.Timeout.
const DefaultAITimeout = 60 * time.Second

// DefaultEffort is the reasoning depth used when a caller sets no explicit
// effort. openai-go's highest tier is "max" (ReasoningEffortMax).
const DefaultEffort = string(shared.ReasoningEffortMax)

// Role constants (kept for callers passing explicit system/user roles; the MAF
// helpers route a separate system/instructions string).
const (
	RoleUser   = "user"
	RoleSystem = "system"
)

// ClientConfig holds the AI client configuration (MAF / openai-go backed).
type ClientConfig struct {
	APIKey  string
	BaseURL string
	Model   string
	// Effort is the reasoning-effort tier sent as `reasoning_effort`. Empty
	// falls back to DefaultEffort ("max"). Recognized: none, minimal, low,
	// medium, high, xhigh, max.
	Effort string
	// Timeout is the per-call deadline; 0 uses DefaultAITimeout.
	Timeout time.Duration
	// Temperature sampling; 0 leaves the API default. Kept for signature
	// compat; deepseek reasoning models largely ignore it.
	Temperature float64
	// Streaming is intentionally IGNORED on the MAF path: chat-completions via
	// this gateway runs non-streaming with an explicit deadline, matching the
	// proven curate pattern (avoids Cloudflare 524 + SDK-retry fights).
	Streaming bool
}

// Message is a single chat message, retained as a neutral carrier.
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// EffortEnum maps a user-supplied effort string onto an openai-go tier.
// "" returns shared.ReasoningEffort("") (zero value = not set).
func EffortEnum(e string) shared.ReasoningEffort {
	switch strings.ToLower(strings.TrimSpace(e)) {
	case "max":
		return shared.ReasoningEffortMax
	case "xhigh":
		return shared.ReasoningEffortXhigh
	case "high":
		return shared.ReasoningEffortHigh
	case "medium":
		return shared.ReasoningEffortMedium
	case "low":
		return shared.ReasoningEffortLow
	case "minimal":
		return shared.ReasoningEffortMinimal
	case "none":
		return shared.ReasoningEffortNone
	default:
		return ""
	}
}

// NewOpenAIClient builds the OpenAI-compatible client for the SAME endpoint &
// credentials as the rest of the repo, from the shared process-wide ClientPool
// (one cached client per endpoint/key). option.WithMaxRetries(0) disables the
// SDK's own retry so retry/backoff is owned by a single explicit place (run in
// run.go, used by ChatContext and curate alike); the SDK would otherwise ignore
// a Retry-After>1min and fight our per-attempt timeout on slow reasoning
// models.
func NewOpenAIClient(cfg *ClientConfig) openai.Client {
	return defaultPool.Get(cfg)
}

// lookupKey returns cfgKey when set, else the project env fallbacks.
func lookupKey(cfgKey string) string {
	if cfgKey != "" {
		return cfgKey
	}
	if k := os.Getenv("OPENAI_API_KEY"); k != "" {
		return k
	}
	return os.Getenv("LLM_AxonHub")
}

// NewChatAgent builds a Chat-Completions-backed MAF agent (pattern validated in
// internal/rss/curate). instructions become the system prompt.
func NewChatAgent(cfg *ClientConfig, name, instructions string) *mafagent.Agent {
	return openaiprovider.NewChatCompletionsAgent(
		NewOpenAIClient(cfg),
		openaiprovider.AgentConfig{
			Model:        cfg.Model,
			Instructions: instructions,
			Config: mafagent.Config{
				Name: name,
			},
		},
	)
}

// UseEffort returns an agent run option carrying `reasoning_effort`. It is the
// single place effort is normalized: trimmed, lowercased, mapped to an
// openai-go tier (via EffortEnum), and defaulted to DefaultEffort when unset.
func UseEffort(effort string) mafagent.Option {
	e := strings.TrimSpace(effort)
	if e == "" {
		e = DefaultEffort
	}
	return openaiprovider.ChatCompletionNewParams(openai.ChatCompletionNewParams{
		ReasoningEffort: EffortEnum(e),
	})
}

// errors surfaced to callers; wrapped with context by runText.
var (
	errNoAPIKey = errors.New("OPENAI_API_KEY / LLM_AxonHub not set")
	errEmpty    = errors.New("empty response from AI model")
)

// ChatContext performs a single non-streaming chat call with caller-provided
// context, applying cfg.Effort (default "max" — single owner in pkg/ai) and a
// bounded timeout/retry. This is the one public MAF-backed bridge; every
// current AI call site routes through it.
func ChatContext(ctx context.Context, cfg *ClientConfig, messages []Message) (string, error) {
	if cfg == nil {
		return "", errors.New("nil AI config")
	}
	if lookupKey(cfg.APIKey) == "" {
		return "", errNoAPIKey
	}
	system, user := splitSystem(messages)
	return runTextContext(ctx, cfg, system, user)
}

// splitSystem pulls the first system message aside; the rest join to user text.
func splitSystem(messages []Message) (system, user string) {
	var buf strings.Builder
	for _, m := range messages {
		if m.Role == RoleSystem {
			if system == "" {
				system = m.Content
			}
			continue
		}
		if buf.Len() > 0 {
			buf.WriteString("\n")
		}
		buf.WriteString(m.Content)
	}
	return system, buf.String()
}

// RetryAfter reads a Retry-After header (seconds) off a typed *openai.Error,
// returned by Cloudflare for the 524/529 it wants you to back off for.
func RetryAfter(err error) time.Duration {
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

// IsRetryable reports whether err is a transient status/transport error worth
// retrying: 408/429/any 5xx (incl. Cloudflare 52x like 524) and closed/EOF
// transport. Our per-attempt context deadline and the caller's own context are
// NOT retried (that is the caller's contract; re-running on expiry would stall
// the loop repeatedly on slow reasoning calls). Shared by every AI caller.
//

func IsRetryable(err error) bool {
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, os.ErrDeadlineExceeded) {
		return false
	}
	var apiErr *openai.Error
	if errors.As(err, &apiErr) {
		return apiErr.StatusCode == http.StatusRequestTimeout ||
			apiErr.StatusCode == http.StatusTooManyRequests ||
			apiErr.StatusCode >= 500
	}
	var nerr net.Error
	if errors.As(err, &nerr) {
		return nerr.Timeout()
	}
	// Sentinel transport/stream errors (matched by identity and description, as a
	// premature EOF surfaces differently depending on wrapping).
	if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) ||
		errors.Is(err, syscall.ECONNRESET) || errors.Is(err, syscall.EPIPE) {
		return true
	}
	msg := err.Error()
	return strings.Contains(msg, "connection reset") ||
		strings.Contains(msg, "unexpected EOF") ||
		strings.Contains(msg, "broken pipe") ||
		strings.Contains(msg, "TLS handshake") ||
		strings.Contains(msg, "i/o timeout")
}
