package write

import (
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
// The log file is created under opts.WikiRoot.
// This function is thread-safe (uses per-file locking via lockPath).
func LogDigestEntry(entry *types.DigestEntry, opts *WriteOptions) (string, error) {
	if opts == nil || opts.WikiRoot == "" {
		// Nothing to log — skip silently.
		return "", nil
	}
	if entry.Timestamp == "" {
		entry.Timestamp = carbon.Now().Layout(time.RFC3339)
	}
	if entry.BatchID == "" {
		entry.BatchID = opts.BatchID
	}

	filename := digestFilename(entry)
	logPath := filepath.Join(opts.WikiRoot, filename)

	if opts.DryRun {
		slog.Info("[DRY RUN] Would log digest entry", "path", logPath,
			"url", entry.URL, "status", entry.Status, "failureKind", entry.FailureKind)

		return logPath, nil
	}

	line, err := json.Marshal(entry)
	if err != nil {
		return "", fmt.Errorf("marshal digest entry: %w", err)
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
