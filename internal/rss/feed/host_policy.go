package rss

import (
	"context"
	"fmt"
	"log/slog"
	"net/url"
	"strings"
	"sync"
	"time"

	retry "github.com/avast/retry-go/v4"
	"golang.org/x/sync/semaphore"
)

// resolvedHostRule is the effective per-host policy after defaults.
type resolvedHostRule struct {
	match        string
	concurrency  int64
	minInterval  time.Duration
	circuitOn5xx bool
	hasExplicit  bool // matched an entry in feed.hosts
}

func (f FeedConfig) resolveHostRule(host string) resolvedHostRule {
	host = strings.ToLower(strings.TrimSpace(host))
	defConc := f.HostDefaultConcurrency
	if defConc <= 0 {
		defConc = DefaultHostFetchConcurrency
	}

	for _, rule := range f.Hosts {
		match := strings.ToLower(strings.TrimSpace(rule.Match))
		if match == "" || match != host {
			continue
		}
		conc := rule.Concurrency
		if conc <= 0 {
			conc = defConc
		}
		// Never exceed global concurrency for a single host slot count sense;
		// host limit is independent but 0-clamp already handled.
		circuit := true
		if rule.CircuitOn5xx != nil {
			circuit = *rule.CircuitOn5xx
		}
		return resolvedHostRule{
			match:        match,
			concurrency:  int64(conc),
			minInterval:  time.Duration(rule.MinIntervalMs) * time.Millisecond,
			circuitOn5xx: circuit,
			hasExplicit:  true,
		}
	}

	return resolvedHostRule{
		match:        host,
		concurrency:  int64(defConc),
		minInterval:  0,
		circuitOn5xx: false,
		hasExplicit:  false,
	}
}

func feedFetchConcurrency(cfg *Config) int {
	if cfg == nil {
		return DefaultFeedFetchConcurrency
	}
	if cfg.FeedConfig.Concurrency > 0 {
		return cfg.FeedConfig.Concurrency
	}

	return DefaultFeedFetchConcurrency
}

func hostnameOf(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Hostname() == "" {
		return ""
	}

	return strings.ToLower(parsed.Hostname())
}

// hostFetchController limits per-host concurrency, optional spacing, and same-batch circuits.
type hostFetchController struct {
	semaphores map[string]*semaphore.Weighted
	lastDone   map[string]time.Time
	circuits   map[string]string
	cfg        FeedConfig
	mu         sync.Mutex
}

func newHostFetchController(cfg FeedConfig) *hostFetchController {
	return &hostFetchController{
		semaphores: make(map[string]*semaphore.Weighted),
		lastDone:   make(map[string]time.Time),
		circuits:   make(map[string]string),
		cfg:        cfg,
	}
}

func (h *hostFetchController) rule(host string) resolvedHostRule {
	return h.cfg.resolveHostRule(host)
}

func (h *hostFetchController) semFor(host string) *semaphore.Weighted {
	h.mu.Lock()
	defer h.mu.Unlock()
	if sem, ok := h.semaphores[host]; ok {
		return sem
	}
	rule := h.cfg.resolveHostRule(host)
	n := rule.concurrency
	if n <= 0 {
		n = 1
	}
	sem := semaphore.NewWeighted(n)
	h.semaphores[host] = sem

	return sem
}

func (h *hostFetchController) circuitReason(host string) (string, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	reason, ok := h.circuits[host]

	return reason, ok
}

func (h *hostFetchController) tripCircuit(host, reason string) {
	if host == "" {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if _, ok := h.circuits[host]; !ok {
		h.circuits[host] = reason
	}
}

// circuitFeedError returns a skip error when the host circuit is already open.
func (h *hostFetchController) circuitFeedError(rawURL, host string) *FeedError {
	if h == nil {
		return nil
	}
	reason, open := h.circuitReason(host)
	if !open {
		return nil
	}

	return hostCircuitFeedError(rawURL, reason)
}

// unrecoverableIfCircuitOpen stops retry.Do when the host was tripped mid-batch.
func (h *hostFetchController) unrecoverableIfCircuitOpen(host string) error {
	if h == nil {
		return nil
	}
	reason, open := h.circuitReason(host)
	if !open {
		return nil
	}

	return retry.Unrecoverable(fmt.Errorf("host circuit open for %s: %s", host, reason))
}

// tripIfNeeded opens a same-batch circuit for risk-control / configured 5xx hosts.
func (h *hostFetchController) tripIfNeeded(host, rawURL string, err error) bool {
	if h == nil {
		return false
	}
	if !shouldTripHostCircuit(h.rule(host), err) {
		return false
	}
	h.tripCircuit(host, err.Error())
	slog.Warn("Host circuit opened",
		slog.String("host", host),
		slog.String(LogKeyURL, rawURL),
		slog.Any(LogKeyError, err))

	return true
}

func (h *hostFetchController) acquire(ctx context.Context, host string) error {
	if host == "" {
		return nil
	}
	if err := h.semFor(host).Acquire(ctx, 1); err != nil {
		return err
	}
	rule := h.rule(host)
	if rule.minInterval <= 0 {
		return nil
	}

	h.mu.Lock()
	last := h.lastDone[host]
	h.mu.Unlock()
	if last.IsZero() {
		return nil
	}
	wait := rule.minInterval - time.Since(last)
	if wait <= 0 {
		return nil
	}
	timer := time.NewTimer(wait)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		h.semFor(host).Release(1)

		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func (h *hostFetchController) release(host string) {
	if host == "" {
		return
	}
	h.mu.Lock()
	h.lastDone[host] = time.Now()
	h.mu.Unlock()
	h.semFor(host).Release(1)
}

// shouldTripHostCircuit decides whether this error should open a same-batch host circuit.
func shouldTripHostCircuit(rule resolvedHostRule, err error) bool {
	if err == nil {
		return false
	}
	info := inspectFetchError(err)
	if info.RiskControl {
		return true
	}
	if rule.hasExplicit && rule.circuitOn5xx && info.Is5xx {
		return true
	}

	return false
}
