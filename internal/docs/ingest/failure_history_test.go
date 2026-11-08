package wikiingest

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadFailureHistoryCountsAuthFailures(t *testing.T) {
	root := t.TempDir()
	auth := `{"timestamp":"2026-11-08T09:00:00+08:00","url":"https://example.com/auth","failureReason":"auth_required"}`
	other := `{"timestamp":"2026-11-08T09:00:00+08:00","url":"https://example.com/other","failureReason":"http_error"}`
	require.NoError(t, os.WriteFile(
		filepath.Join(root, "digest-extract-error.jsonl"),
		[]byte(auth+"\n"+auth+"\n"+auth+"\n"+other+"\n"),
		0o600,
	))

	history := loadFailureHistory(root, nil)
	assert.Equal(t, 3, history.authFailures("https://example.com/auth"))
	assert.Equal(t, 0, history.authFailures("https://example.com/other"))
}

func TestFailureHistoryNilIsSafe(t *testing.T) {
	var history *failureHistory
	assert.Zero(t, history.authFailures("https://example.com/auth"))
}
