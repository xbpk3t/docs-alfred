package wikiingest

import (
	"bufio"
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"

	wikitypes "github.com/xbpk3t/docs-alfred/internal/docs/wiki/types"
	"github.com/xbpk3t/docs-alfred/pkg/urlutil"
)

// digestFileNameSuccess is the JSONL file that records successful digest
// outcomes (see internal/docs/wiki/write/digest.go).
const digestFileNameSuccess = "digest-success.jsonl"

// digestHistory tracks URLs already successfully digested, loaded from
// <wikiRoot>/digest-success.jsonl. Only successes count as "already digested":
// failure entries (fetch/extract/classify/AI) are NOT remembered, so a failed
// URL is retried on the next digest instead of being skipped.
type digestHistory struct {
	// keys holds normalized-for-dedup URL keys (see urlutil.NormalizeForDedup).
	keys map[string]bool
}

// loadDigestHistory reads digest-success.jsonl under wikiRoot and builds the
// set of successfully digested URL keys. A missing or unreadable log file is
// not an error — it just yields an empty history.
func loadDigestHistory(wikiRoot string) *digestHistory {
	h := &digestHistory{keys: make(map[string]bool)}
	if wikiRoot == "" {
		return h
	}

	path := filepath.Join(wikiRoot, digestFileNameSuccess)
	f, err := os.Open(path)
	if err != nil {
		if !os.IsNotExist(err) {
			slog.Warn("wiki digest: cannot read success history, skipping dedup", "path", path, "error", err)
		}
		return h
	}
	defer func() { _ = f.Close() }()

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

	return h
}

// alreadyDigested reports whether a URL has a successful digest history entry.
func (h *digestHistory) alreadyDigested(urlStr string) bool {
	if h == nil {
		return false
	}
	return h.keys[urlutil.NormalizeForDedup(urlStr)]
}
