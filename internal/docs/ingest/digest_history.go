package wikiingest

import (
	"bufio"
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"sync"

	wikitypes "github.com/xbpk3t/docs-alfred/internal/docs/wiki/types"
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
	keys map[string]bool
	mu   sync.Mutex
}

// newDigestHistory returns an empty history, ready for in-run deduplication.
func newDigestHistory() *digestHistory {
	return &digestHistory{keys: make(map[string]bool)}
}

// loadDigestHistory reads the persisted success history and returns a history
// seeded with it. A missing or unreadable log file is not an error — it just
// yields an empty history.
func loadDigestHistory(wikiRoot string) *digestHistory {
	h := newDigestHistory()
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
