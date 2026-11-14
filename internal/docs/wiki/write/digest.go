package write

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/xbpk3t/docs-alfred/internal/docs/wiki/types"
	"log/slog"
	"time"

	carbon "github.com/dromara/carbon/v2"
)

// digestFilenames maps outcomes to log files.
const (
	digestFilenameSuccess        = "digest-success.jsonl"
	digestFilenameFetchError     = "digest-fetch-error.jsonl"
	digestFilenameExtractError   = "digest-extract-error.jsonl"
	digestFilenameAIError        = "digest-ai-error.jsonl"
	digestFilenameClassifyReject = "digest-classify-rejected.jsonl"
	digestFilenameRuns           = "digest-runs.jsonl"
)

// digestFilename returns the JSONL filename for the given entry.
func digestFilename(entry *types.DigestEntry) string {
	if entry.Status == types.DigestSuccess {
		return digestFilenameSuccess
	}
	switch entry.FailureKind {
	case string(types.FailureFetch), string(types.FailureResolve):
		return digestFilenameFetchError
	case string(types.FailureExtract):
		return digestFilenameExtractError
	case string(types.FailureClassify):
		return digestFilenameClassifyReject
	case string(types.FailureAI):
		return digestFilenameAIError
	default:
		// Unexpected failures.
		return digestFilenameAIError
	}
}

// LogDigestEntry appends a JSON line to logs/digest-<kind>.jsonl in the R2
// bucket.
func LogDigestEntry(entry *types.DigestEntry, opts *WriteOptions) (string, error) {
	if opts == nil {
		return "", nil
	}
	if entry.Timestamp == "" {
		entry.Timestamp = carbon.Now().Layout(time.RFC3339)
	}
	if entry.BatchID == "" {
		entry.BatchID = opts.BatchID
	}

	line, err := json.Marshal(entry)
	if err != nil {
		return "", fmt.Errorf("marshal digest entry: %w", err)
	}

	return appendLogLine(digestFilename(entry), line, opts)
}

// DigestRunLogKey is the R2 key of the digest run-summary log.
const DigestRunLogKey = "logs/" + digestFilenameRuns

// DigestFailureLogNames returns the JSONL files that can contain failure
// entries, so readers do not re-declare the writer's filenames.
func DigestFailureLogNames() []string {
	return []string{digestFilenameAIError, digestFilenameExtractError, digestFilenameFetchError}
}

// appendLogLine appends one JSON line to a logs/<filename> object in the
// bucket. The success log is read back by history dedup, so its URL set must
// stay complete.
func appendLogLine(filename string, line []byte, opts *WriteOptions) (string, error) {
	if opts.R2Store == nil {
		return "", errors.New("digest log: r2 store required")
	}
	r2LogMu.Lock()
	defer r2LogMu.Unlock()

	key := "logs/" + filename
	if opts.DryRun {
		slog.Info("[DRY RUN] Would log digest line to R2", "key", key)

		return key, nil
	}

	existing, getErr := opts.R2Store.GetObject(context.Background(), key)
	if getErr != nil {
		return "", fmt.Errorf("r2 log %s: read existing: %w", key, getErr)
	}
	merged := make([]byte, 0, len(existing)+len(line)+1)
	merged = append(merged, existing...)
	if len(merged) > 0 && merged[len(merged)-1] != '\n' {
		merged = append(merged, '\n')
	}
	merged = append(merged, line...)
	merged = append(merged, '\n')
	if _, err := opts.R2Store.Upload(context.Background(), key, string(merged)); err != nil {
		return "", fmt.Errorf("r2 log %s: %w", key, err)
	}

	return key, nil
}

// LogDigestRun appends one run-summary line to logs/digest-runs.jsonl in the
// bucket.
func LogDigestRun(entry *types.DigestRunEntry, opts *WriteOptions) (string, error) {
	if opts == nil {
		return "", nil
	}
	if entry.Timestamp == "" {
		entry.Timestamp = carbon.Now().Layout(time.RFC3339)
	}
	line, err := json.Marshal(entry)
	if err != nil {
		return "", fmt.Errorf("marshal digest run: %w", err)
	}

	return appendLogLine(digestFilenameRuns, line, opts)
}
