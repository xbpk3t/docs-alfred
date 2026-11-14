package wikiingest

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLoadFailureHistoryCountsAuthFailures(t *testing.T) {
	store := newFakeMediaStore()
	auth := `{"timestamp":"2026-11-08T09:00:00+08:00","url":"https://example.com/auth","failureReason":"auth_required"}`
	other := `{"timestamp":"2026-11-08T09:00:00+08:00","url":"https://example.com/other","failureReason":"http_error"}`
	store.objects["logs/digest-extract-error.jsonl"] = auth + "\n" + auth + "\n" + auth + "\n" + other + "\n"

	history := loadFailureHistory(digestStoreAdapter{store: store})
	assert.Equal(t, 3, history.authFailures("https://example.com/auth"))
	assert.Equal(t, 0, history.authFailures("https://example.com/other"))
}

func TestFailureHistoryNilIsSafe(t *testing.T) {
	var history *failureHistory
	assert.Zero(t, history.authFailures("https://example.com/auth"))
}
