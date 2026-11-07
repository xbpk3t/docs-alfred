package write

import (
	"context"
	"errors"
	"strings"

	"github.com/xbpk3t/docs-alfred/pkg/urlutil"
)

// PruneResult reports which uncat entries a prune removed (or would remove).
type PruneResult struct {
	Key     string
	Removed []string
	DryRun  bool
}

// PruneManualReview removes entry blocks whose URL matches one of urls from
// the R2 uncat.md queue. R2 has no delete primitive; this is a serialized
// read-merge-write, so it must run while no digest is writing uncat.md.
func PruneManualReview(
	ctx context.Context,
	store DigestStore,
	urls []string,
	dryRun bool,
) (*PruneResult, error) {
	if store == nil {
		return nil, errors.New("prune uncat: r2 store required")
	}
	targets := make(map[string]bool, len(urls))
	for _, u := range urls {
		if key := urlutil.NormalizeForDedup(u); key != "" {
			targets[key] = true
		}
	}
	result := &PruneResult{Key: manualReviewKey, DryRun: dryRun}
	if len(targets) == 0 {
		return result, nil
	}

	r2UncatMu.Lock()
	defer r2UncatMu.Unlock()

	existing, err := store.GetObject(ctx, manualReviewKey)
	if err != nil {
		return nil, err
	}
	pruned, removed := pruneUncatContent(string(existing), targets)
	if len(removed) == 0 {
		return result, nil
	}
	if !dryRun {
		if _, err := store.Upload(ctx, manualReviewKey, pruned); err != nil {
			return nil, err
		}
	}
	result.Removed = removed

	return result, nil
}

// pruneUncatContent removes ### entry blocks whose metadata URL is in targets.
// Date headings and all non-matching blocks are preserved.
func pruneUncatContent(content string, targets map[string]bool) (string, []string) {
	if strings.TrimSpace(content) == "" || len(targets) == 0 {
		return content, nil
	}

	var out, block []string
	var blockURL string
	var removed []string

	flush := func() {
		if len(block) == 0 {
			return
		}
		if blockURL != "" && targets[blockURL] {
			removed = append(removed, blockURL)
		} else {
			out = append(out, block...)
		}
		block = nil
		blockURL = ""
	}

	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(trimmed, "### "):
			flush()
			block = append(block, line)
		case strings.HasPrefix(trimmed, "## "):
			flush()
			out = append(out, line)
		case len(block) > 0:
			block = append(block, line)
			if blockURL == "" {
				if key, ok := entryURLFromLine(line); ok {
					blockURL = key
				}
			}
		default:
			out = append(out, line)
		}
	}
	flush()

	if len(removed) == 0 {
		return content, nil
	}

	pruned := strings.Join(out, "\n")
	for strings.Contains(pruned, "\n\n\n") {
		pruned = strings.ReplaceAll(pruned, "\n\n\n", "\n\n")
	}
	if !strings.HasSuffix(pruned, "\n") {
		pruned += "\n"
	}

	return pruned, removed
}
