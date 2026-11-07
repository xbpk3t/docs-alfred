package wikiingest

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	wikitypes "github.com/xbpk3t/docs-alfred/internal/docs/wiki/types"
	"github.com/xbpk3t/docs-alfred/pkg/urlutil"
)

// BackfillHistoryInput merges the pre-R2 local success log into the R2 success
// history so old URLs are not digested again.
type BackfillHistoryInput struct {
	Config    *Config
	LocalPath string
	DryRun    bool
}

// BackfillHistoryResult reports what the merge would do (or did).
type BackfillHistoryResult struct {
	Key          string `json:"key"`
	LocalUnique  int    `json:"localUnique"`
	RemoteUnique int    `json:"remoteUnique"`
	Added        int    `json:"added"`
	DryRun       bool   `json:"dryRun"`
}

const digestSuccessLogKey = "logs/digest-success.jsonl"

// RunBackfillDigestHistory reads the local success log, merges its unique
// successes into logs/digest-success.jsonl in R2, and uploads the result once.
func RunBackfillDigestHistory(ctx context.Context, input BackfillHistoryInput) (*BackfillHistoryResult, error) {
	if input.Config == nil {
		return nil, fmt.Errorf("wiki config is required")
	}
	store := newMediaStore(input.Config)
	if store == nil {
		return nil, fmt.Errorf("wiki backfill-history requires r2.bucket and R2 credentials")
	}

	localPath := input.LocalPath
	if localPath == "" {
		localPath = filepath.Join(ResolveWikiRoot(input.Config), "digest-success.jsonl")
	}
	local, err := os.ReadFile(localPath)
	if err != nil {
		return nil, fmt.Errorf("read local success log: %w", err)
	}
	remote, err := store.GetObject(ctx, digestSuccessLogKey)
	if err != nil {
		return nil, fmt.Errorf("read remote success log: %w", err)
	}

	merged, stats, err := mergeSuccessHistory(local, remote)
	if err != nil {
		return nil, err
	}
	result := &BackfillHistoryResult{
		LocalUnique:  stats.localUnique,
		RemoteUnique: stats.remoteUnique,
		Added:        stats.added,
		Key:          digestSuccessLogKey,
		DryRun:       input.DryRun,
	}
	if input.DryRun || stats.added == 0 {
		return result, nil
	}
	if _, err := store.Upload(ctx, digestSuccessLogKey, string(merged)); err != nil {
		return nil, fmt.Errorf("upload merged success log: %w", err)
	}

	return result, nil
}

type backfillStats struct {
	localUnique  int
	remoteUnique int
	added        int
}

// mergeSuccessHistory appends local-only success entries to the remote log.
// Entries are deduplicated by the same normalized URL key the digest history
// uses. Remote content is preserved (minus malformed lines).
func mergeSuccessHistory(local, remote []byte) ([]byte, backfillStats, error) {
	var stats backfillStats
	seen := map[string]bool{}

	var base strings.Builder
	for _, line := range strings.Split(string(remote), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var entry wikitypes.DigestEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			continue
		}
		if entry.Status == wikitypes.DigestSuccess && entry.URL != "" {
			key := urlutil.NormalizeForDedup(entry.URL)
			if seen[key] {
				continue
			}
			seen[key] = true
		}
		base.WriteString(line)
		base.WriteByte('\n')
	}
	stats.remoteUnique = len(seen)

	localSeen := map[string]bool{}
	var additions strings.Builder
	for _, line := range strings.Split(string(local), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var entry wikitypes.DigestEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			continue
		}
		if entry.Status != wikitypes.DigestSuccess || entry.URL == "" {
			continue
		}
		key := urlutil.NormalizeForDedup(entry.URL)
		if localSeen[key] {
			continue
		}
		localSeen[key] = true
		if seen[key] {
			continue
		}
		seen[key] = true
		additions.WriteString(line)
		additions.WriteByte('\n')
		stats.added++
	}
	stats.localUnique = len(localSeen)

	return []byte(base.String() + additions.String()), stats, nil
}
