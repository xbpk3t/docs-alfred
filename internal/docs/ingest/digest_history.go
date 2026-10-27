package wikiingest

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"sync"

	wikitypes "github.com/xbpk3t/docs-alfred/internal/docs/wiki/types"
	wikiwrite "github.com/xbpk3t/docs-alfred/internal/docs/wiki/write"
	"github.com/xbpk3t/docs-alfred/pkg/urlutil"
)

// digestFileNameSuccess is the JSONL file that records successful digest
// outcomes (see internal/docs/wiki/write/digest.go).
const digestFileNameSuccess = "digest-success.jsonl"

// digestHistory tracks URLs already digested. Only successes count as "already
// digested": failure entries (fetch/extract/classify/AI) are NOT remembered, so
// a failed URL is retried on the next digest instead of being skipped.
//
// The single keys set has two jobs: it is seeded from the persisted success
// history, and it is then grown through the run as each URL is claimed, so a
// duplicate URL later in the same batch collapses to one process. Whether the
// persisted history is seeded is a caller policy (wiki add skips it under
// --force, to re-digest); the claim set itself always deduplicates within a run.
type digestHistory struct {
	r2Store wikiwrite.DigestStore
	keys    map[string]bool
	mu      sync.Mutex
}

// newDigestHistory returns an empty history, ready for in-run deduplication.
// r2Store, when non-nil, is where the persisted success log is read from
// (logs/digest-success.jsonl in the bucket instead of the local wiki root).
func newDigestHistory(r2Store wikiwrite.DigestStore) *digestHistory {
	return &digestHistory{keys: make(map[string]bool), r2Store: r2Store}
}

// loadDigestHistory reads the persisted success history and returns a history
// seeded with it. A missing or unreadable log file is not an error — it just
// yields an empty history.
func loadDigestHistory(wikiRoot string, r2Store wikiwrite.DigestStore) *digestHistory {
	h := newDigestHistory(r2Store)
	h.loadSuccesses(wikiRoot)
	return h
}

// loadSuccesses merges keys from the persisted success history into h. Used at
// run start to seed history, or to seed it on top of an existing (possibly
// already-claimed) set; it is idempotent.
func (h *digestHistory) loadSuccesses(wikiRoot string) {
	if h == nil || wikiRoot == "" {
		return
	}

	if h.r2Store != nil {
		h.loadSuccessesFromR2()

		return
	}

	path := filepath.Join(wikiRoot, digestFileNameSuccess)
	f, err := os.Open(path)
	if err != nil {
		if !os.IsNotExist(err) {
			slog.Warn("wiki digest: cannot read success history, skipping dedup", "path", path, "error", err)
		}
		return
	}
	defer func() { _ = f.Close() }()

	h.mu.Lock()
	defer h.mu.Unlock()

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	var line int
	for scanner.Scan() {
		line++
		var entry wikitypes.DigestEntry
		if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil {
			slog.Debug("wiki digest: skipping malformed history line", "line", line, "error", err)
			continue
		}
		if entry.Status != wikitypes.DigestSuccess || entry.URL == "" {
			continue
		}
		h.keys[urlutil.NormalizeForDedup(entry.URL)] = true
	}
	if err := scanner.Err(); err != nil {
		slog.Warn("wiki digest: partial history read", "path", path, "error", err)
	}

	slog.Info("wiki digest: loaded digest history", "count", len(h.keys), "path", path)
}

// loadSuccessesFromR2 seeds history from logs/digest-success.jsonl in the
// bucket. A missing object just yields an empty history.
func (h *digestHistory) loadSuccessesFromR2() {
	data, err := h.r2Store.GetObject(context.Background(), "logs/"+digestFileNameSuccess)
	if err != nil || len(data) == 0 {
		if err != nil {
			slog.Debug("wiki digest: no R2 success history yet", "error", err)
		}

		return
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	for _, raw := range bytes.Split(data, []byte("\n")) {
		if len(bytes.TrimSpace(raw)) == 0 {
			continue
		}
		var entry wikitypes.DigestEntry
		if err := json.Unmarshal(raw, &entry); err != nil {
			slog.Debug("wiki digest: skipping malformed history line", "error", err)
			continue
		}
		if entry.Status != wikitypes.DigestSuccess || entry.URL == "" {
			continue
		}
		h.keys[urlutil.NormalizeForDedup(entry.URL)] = true
	}

	slog.Info("wiki digest: loaded digest history from R2", "count", len(h.keys))
}

// claimOrSeen reports whether a URL is already handled for the purposes of this
// run and, if not, claims it so a later duplicate in the same batch collapses.
//
// A URL is "seen" when it is already in the keys set — either seeded from the
// persisted history or claimed earlier in this run. The check and the claim are
// atomic under a lock, so concurrent processing of a duplicate URL in one batch
// still results in exactly one real process.
func (h *digestHistory) claimOrSeen(urlStr string) bool {
	if h == nil {
		return false
	}
	key := urlutil.NormalizeForDedup(urlStr)
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.keys[key] {
		return true
	}
	h.keys[key] = true

	return false
}
