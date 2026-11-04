package wikiingest

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	wikitypes "github.com/xbpk3t/docs-alfred/internal/docs/wiki/types"
)

// bilibiliMetadataBody mirrors the 25-row field/value table emitted by
// `opencli bilibili video -f md` (clis/bilibili/video.js). It is metadata, not
// transcript content.
func bilibiliMetadataBody() string {
	fields := [][2]string{
		{"bvid", "BV1xx411c7mD"},
		{"aid", "123456789"},
		{"title", "A fairly ordinary bilibili video title"},
		{"author", "some creator (mid: 123456)"},
		{"mid", "123456"},
		{"face", "https://i0.hdslb.com/bfs/face/abcdef0123456789.jpg"},
		{"category", "科技"},
		{"publish_time", "2026-10-01 12:00"},
		{"pubdate_ts", "1790000000"},
		{"duration", "12m34s (754s)"},
		{"duration_sec", "754"},
		{"view", "123456"},
		{"danmaku", "789"},
		{"reply", "456"},
		{"like", "12345"},
		{"coin", "678"},
		{"favorite", "2345"},
		{"share", "345"},
		{"parts", "1"},
		{"thumbnail", "https://i0.hdslb.com/bfs/archive/abcdef0123456789.jpg"},
		{"description", strings.Repeat("视频简介内容。", 20)},
		{"requires_payment", "false"},
		{"payment_type", ""},
		{"pay_preview", "false"},
		{"redirect_url", ""},
	}
	var b strings.Builder
	b.WriteString("| Field | Value |\n| --- | --- |\n")
	for _, f := range fields {
		fmt.Fprintf(&b, "| %s | %s |\n", f[0], f[1])
	}

	return b.String()
}

// Claim 7 says a video with no subtitles and no official summary reaches the
// permanent too-short gate. The metadata table alone exceeds minVideoRunes.
func TestReviewVideoMetadataOnlyStillTripsTooShortGate(t *testing.T) {
	body := bilibiliMetadataBody()
	require.Greater(t, len([]rune(body)), minVideoRunes,
		"the bilibili metadata table is already larger than the too-short threshold")

	_, tooShort := videoTooShortPending(
		"https://www.bilibili.com/video/BV1xx411c7mD",
		wikitypes.ContentVideo,
		&wikitypes.ContentFetchResult{Title: "A video", Body: body},
	)
	assert.True(t, tooShort, "metadata-only output must not be treated as transcript content")
}

// The "not found" substring heuristic classifies a missing local executable as
// a permanent 404, which flushes the inbox line.
func TestReviewMissingExecutableIsNotPermanentNotFound(t *testing.T) {
	result := &wikitypes.ContentFetchResult{
		Error: "exec: opencli: executable file not found in $PATH",
	}
	reason := failureReasonForFetchResult(result)

	assert.NotEqual(t, wikitypes.FailureReasonNotFound, reason)
	assert.True(t, failureRetryable(reason), "a missing tool is transient, not a dead URL")
}

// A nil fetch result is a driver bug or transient failure, not proof that the
// URL has permanently empty content.
func TestReviewNilFetchResultDoesNotFlushInboxLine(t *testing.T) {
	reason := failureReasonForFetchResult(nil)
	assert.True(t, failureRetryable(reason), "nil driver result is transient")
}

// A link-only X post is deterministic (the target is another status), so
// retrying it forever only burns browser slots and keeps the inbox queue alive.
func TestReviewLinkOnlyPostIsPermanent(t *testing.T) {
	assert.False(t, failureRetryable(wikitypes.FailureReasonLinkOnlyPost))
}

// Untyped extract failures used to flush the inbox line. Mapping every such
// error to retryable partial_extract keeps deterministic failures queued
// forever (direct-audio URLs, empty feeds, parse errors).
func TestReviewUntypedExtractFailureIsNotRetryableForever(t *testing.T) {
	reason := failureReasonForFetchResult(&wikitypes.ContentFetchResult{
		Error:       "extract: rss transcript unavailable: direct audio URL has no RSS metadata",
		FailureKind: wikitypes.FailureExtract,
	})
	assert.False(t, failureRetryable(reason), "a deterministic extract failure must not be retried forever")
}
