package write

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/xbpk3t/docs-alfred/internal/docs/wiki/types"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	carbon "github.com/dromara/carbon/v2"
	"github.com/xbpk3t/docs-alfred/pkg/fileutil"
)

// digestFilenames maps outcomes to log files.
const (
	digestFilenameSuccess        = "digest-success.jsonl"
	digestFilenameFetchError     = "digest-fetch-error.jsonl"
	digestFilenameExtractError   = "digest-extract-error.jsonl"
	digestFilenameAIError        = "digest-ai-error.jsonl"
	digestFilenameClassifyReject = "digest-classify-rejected.jsonl"
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

// LogDigestEntry appends a JSON line to the appropriate digest log file.
// With R2Store set, the log lands at logs/digest-<kind>.jsonl in the bucket
// instead of a local file under WikiRoot; otherwise it appends locally.
// The local write is thread-safe (uses per-file locking via lockPath).
func LogDigestEntry(entry *types.DigestEntry, opts *WriteOptions) (string, error) {
	if opts == nil || (opts.WikiRoot == "" && opts.R2Store == nil) {
		// Nothing to log — skip silently.
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

	if opts.R2Store != nil {
		return logDigestEntryR2(entry, opts, line)
	}

	logPath := logDigestPath(opts, entry)
	if opts.DryRun {
		slog.Info("[DRY RUN] Would log digest entry", "path", logPath,
			"url", entry.URL, "status", entry.Status, "failureKind", entry.FailureKind)

		return logPath, nil
	}

	unlock := lockPath(logPath)
	defer unlock()

	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, fileutil.FilePermPrivate)
	if err != nil {
		return "", fmt.Errorf("open digest log %s: %w", logPath, err)
	}
	defer func() { _ = f.Close() }()

	if _, err := f.Write(line); err != nil {
		return "", fmt.Errorf("write digest entry: %w", err)
	}
	if _, err := f.WriteString("\n"); err != nil {
		return "", fmt.Errorf("write digest newline: %w", err)
	}

	slog.Debug("Digest entry logged", "path", logPath, "url", entry.URL, "status", entry.Status)

	return logPath, nil
}

// logDigestPath is the local JSONL path for the given entry's outcome.
func logDigestPath(opts *WriteOptions, entry *types.DigestEntry) string {
	return filepath.Join(opts.WikiRoot, digestFilename(entry))
}

// logDigestEntryR2 appends one JSON line to a logs/digest-<kind>.jsonl object
// in the bucket. R2 has no append, so it read-merge-writes the object; the
// success log is read back by history dedup, so its URL set must stay complete.
func logDigestEntryR2(entry *types.DigestEntry, opts *WriteOptions, line []byte) (string, error) {
	key := "logs/" + digestFilename(entry)
	if opts.DryRun {
		slog.Info("[DRY RUN] Would log digest entry to R2", "key", key,
			"url", entry.URL, "status", entry.Status, "failureKind", entry.FailureKind)

		return key, nil
	}

	existing, getErr := opts.R2Store.GetObject(context.Background(), key)
	if getErr != nil {
		return "", fmt.Errorf("r2 log %s: read existing: %w", key, getErr)
	}
	merged := make([]byte, 0, len(existing)+len(line)+1)
	merged = append(merged, existing...)
	merged = append(merged, line...)
	if merged[len(merged)-1] != '\n' {
		merged = append(merged, '\n')
	}
	if _, err := opts.R2Store.Upload(context.Background(), key, string(merged)); err != nil {
		return "", fmt.Errorf("r2 log %s: %w", key, err)
	}
	slog.Debug("Digest entry logged to R2", "key", key, "url", entry.URL, "status", entry.Status)

	return key, nil
}
