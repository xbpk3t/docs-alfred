package wikiingest

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"regexp"
	"strings"

	wikifetch "github.com/xbpk3t/docs-alfred/internal/docs/wiki/fetch"
	wikitypes "github.com/xbpk3t/docs-alfred/internal/docs/wiki/types"
	"github.com/xbpk3t/docs-alfred/pkg/litter"
)

// mediaStore is the R2 surface the digest needs: upload transcripts and read
// the rss2nl trns index. *litter.R2 implements it; tests fake it.
type mediaStore interface {
	Upload(ctx context.Context, filename, content string) (*litter.Result, error)
	ListKeys(ctx context.Context, prefix string) ([]string, error)
	GetObject(ctx context.Context, key string) ([]byte, error)
}

// transcriptSectionMarker is the heading the opencli driver appends when it
// merges a video subtitle into the fetch body.
const transcriptSectionMarker = "## 字幕内容"

// maybeUploadTranscript uploads the fetched media transcript to R2 and returns
// its public URL. Returns "" when R2 is not configured, the content carries no
// transcript, or the upload fails — the entry still writes without the
// Transcript link, and the failure reason lands in the log.
func maybeUploadTranscript(
	ctx context.Context,
	deps *dependencies,
	urlStr string,
	fetchResult *wikitypes.ContentFetchResult,
) string {
	if deps.r2 == nil || fetchResult == nil {
		return ""
	}
	transcript := extractTranscript(urlStr, fetchResult)
	if transcript == "" {
		return ""
	}

	result, err := deps.r2.Upload(ctx, transcriptObjectKey(urlStr), transcriptDocument(fetchResult.Title, urlStr, transcript))
	if err != nil {
		slog.Warn("wiki digest: transcript upload failed", "url", urlStr, "error", err)

		return ""
	}
	slog.Info("wiki digest: transcript uploaded", "url", urlStr, "transcript", result.URL)

	return result.URL
}

// extractTranscript pulls the transcript text out of a media fetch result:
// video bodies carry it under the 字幕内容 section, audio bodies are the
// transcript itself. Non-media content yields "".
func extractTranscript(urlStr string, fetchResult *wikitypes.ContentFetchResult) string {
	body := fetchResult.Body
	switch wikifetch.DetectContentType(urlStr) {
	case wikitypes.ContentVideo:
		idx := strings.Index(body, transcriptSectionMarker)
		if idx < 0 {
			return ""
		}

		return strings.TrimSpace(strings.TrimPrefix(body[idx:], transcriptSectionMarker))
	case wikitypes.ContentAudio:
		return strings.TrimSpace(body)
	default:
		return ""
	}
}

// transcriptDocument wraps a transcript into a self-contained markdown file
// with its source link.
func transcriptDocument(title, urlStr, transcript string) string {
	var b strings.Builder
	if t := strings.TrimSpace(title); t != "" {
		fmt.Fprintf(&b, "# %s\n\n", t)
	}
	fmt.Fprintf(&b, "Source: %s\n\n%s\n", urlStr, transcript)

	return b.String()
}

var bvIDRe = regexp.MustCompile(`BV[0-9A-Za-z]{10}`)

// transcriptObjectKey names the uploaded transcript: bilibili videos use the
// stable BV id, everything else a short hash of the episode URL.
func transcriptObjectKey(urlStr string) string {
	if bv := bvIDRe.FindString(urlStr); bv != "" {
		return "transcript-" + bv + ".md"
	}
	sum := sha256.Sum256([]byte(urlStr))

	return "transcript-" + hex.EncodeToString(sum[:8]) + ".md"
}
