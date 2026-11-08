package wikiingest

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"

	wikitypes "github.com/xbpk3t/docs-alfred/internal/docs/wiki/types"
	wikiwrite "github.com/xbpk3t/docs-alfred/internal/docs/wiki/write"
	"github.com/xbpk3t/docs-alfred/pkg/urlutil"
)

// maxAuthFailures is the number of prior auth failures after which a URL is
// quarantined. Auth is normally recoverable by logging in; this cap prevents a
// forgotten session from keeping the same lines in inbox forever.
const maxAuthFailures = 3

// failureHistory counts prior retryable auth failures per URL. Only digest
// reads it; wiki add stays explicitly re-digest-capable.
type failureHistory struct {
	authCounts map[string]int
}

func loadFailureHistory(wikiRoot string, r2Store wikiwrite.DigestStore) *failureHistory {
	h := &failureHistory{authCounts: map[string]int{}}
	for _, name := range wikiwrite.DigestFailureLogNames() {
		var data []byte
		if r2Store != nil {
			var err error
			data, err = r2Store.GetObject(context.Background(), "logs/"+name)
			if err != nil {
				continue
			}
		} else {
			data, _ = os.ReadFile(filepath.Join(wikiRoot, name))
		}
		for _, raw := range bytes.Split(data, []byte("\n")) {
			// Only auth failures count; skip the JSON decode for every other
			// (much more common) line.
			if !bytes.Contains(raw, []byte(wikitypes.FailureReasonAuthRequired)) {
				continue
			}
			var entry wikitypes.DigestEntry
			if json.Unmarshal(raw, &entry) != nil || entry.URL == "" {
				continue
			}
			if entry.FailureReason != wikitypes.FailureReasonAuthRequired {
				continue
			}
			h.authCounts[urlutil.NormalizeForDedup(entry.URL)]++
		}
	}

	return h
}

func (h *failureHistory) authFailures(urlStr string) int {
	if h == nil {
		return 0
	}

	return h.authCounts[urlutil.NormalizeForDedup(urlStr)]
}
