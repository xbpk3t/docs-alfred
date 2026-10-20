package wikiingest

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	wikitypes "github.com/xbpk3t/docs-alfred/internal/docs/wiki/types"
	"github.com/xbpk3t/docs-alfred/pkg/litter"
)

// fakeMediaStore fakes the R2 surface: object listing/reading for the trns
// index and uploads for transcripts.
type fakeMediaStore struct {
	objects map[string]string // key → content
	uploads map[string]string // filename → content
	upErr   error
	listErr error
	getErr  error
}

func newFakeMediaStore() *fakeMediaStore {
	return &fakeMediaStore{objects: map[string]string{}, uploads: map[string]string{}}
}

func (f *fakeMediaStore) Upload(_ context.Context, filename, content string) (*litter.Result, error) {
	if f.upErr != nil {
		return nil, f.upErr
	}
	f.uploads[filename] = content

	return &litter.Result{URL: "https://media.example/" + filename, Driver: "r2"}, nil
}

func (f *fakeMediaStore) ListKeys(_ context.Context, prefix string) ([]string, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	var keys []string
	for k := range f.objects {
		if strings.HasPrefix(k, prefix) {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)

	return keys, nil
}

func (f *fakeMediaStore) GetObject(_ context.Context, key string) ([]byte, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}
	data, ok := f.objects[key]
	if !ok {
		return nil, fmt.Errorf("no such object: %s", key)
	}

	return []byte(data), nil
}

const (
	testVideoURL = "https://www.bilibili.com/video/BV1xx411c7mD"
	testAudioURL = "https://www.xiaoyuzhoufm.com/episode/abc123"
	testTextURL  = "https://example.com/article"
)

func videoFetchResult(body string) *wikitypes.ContentFetchResult {
	return &wikitypes.ContentFetchResult{Title: "视频标题", Body: body}
}

func TestExtractTranscriptVideoUsesSubtitleSection(t *testing.T) {
	body := "元数据\n\n" + transcriptSectionMarker + "\n\n第一句字幕\n第二句字幕\n"
	got := extractTranscript(testVideoURL, videoFetchResult(body))
	assert.Equal(t, "第一句字幕\n第二句字幕", got)
}

func TestExtractTranscriptVideoWithoutMarkerIsEmpty(t *testing.T) {
	got := extractTranscript(testVideoURL, videoFetchResult("只有元数据"))
	assert.Empty(t, got)
}

func TestExtractTranscriptAudioIsWholeBody(t *testing.T) {
	got := extractTranscript(testAudioURL, videoFetchResult("  episode transcript text  "))
	assert.Equal(t, "episode transcript text", got)
}

func TestExtractTranscriptNonMediaIsEmpty(t *testing.T) {
	got := extractTranscript(testTextURL, videoFetchResult("plain article body"))
	assert.Empty(t, got)
}

func TestTranscriptObjectKeyBilibiliUsesBV(t *testing.T) {
	assert.Equal(t, "transcript-BV1xx411c7mD.md", transcriptObjectKey(testVideoURL))
	// A query string must not leak into the key.
	assert.Equal(t, "transcript-BV1xx411c7mD.md", transcriptObjectKey(testVideoURL+"?spm_id_from=333.999"))
}

func TestTranscriptObjectKeyHashStableAndDistinct(t *testing.T) {
	first := transcriptObjectKey(testAudioURL)
	assert.Equal(t, first, transcriptObjectKey(testAudioURL), "same URL must map to the same key")
	assert.NotEqual(t, first, transcriptObjectKey(testAudioURL+"?t=1"), "different URLs must not collide")
	assert.Regexp(t, `^transcript-[0-9a-f]{16}\.md$`, first)
}

func TestMaybeUploadTranscriptSkipsWithoutStore(t *testing.T) {
	deps := &dependencies{}
	got := maybeUploadTranscript(context.Background(), deps, testVideoURL, videoFetchResult("x"))
	assert.Empty(t, got)
}

func TestMaybeUploadTranscriptSkipsNonMedia(t *testing.T) {
	store := newFakeMediaStore()
	deps := &dependencies{r2: store}

	got := maybeUploadTranscript(context.Background(), deps, testTextURL, videoFetchResult("body"))
	assert.Empty(t, got)
	assert.Empty(t, store.uploads)
}

func TestMaybeUploadTranscriptUploadsVideoTranscript(t *testing.T) {
	store := newFakeMediaStore()
	deps := &dependencies{r2: store}
	body := transcriptSectionMarker + "\n\n字幕正文"

	got := maybeUploadTranscript(context.Background(), deps, testVideoURL, videoFetchResult(body))

	require.Equal(t, "https://media.example/transcript-BV1xx411c7mD.md", got)
	require.Contains(t, store.uploads, "transcript-BV1xx411c7mD.md")
	uploaded := store.uploads["transcript-BV1xx411c7mD.md"]
	assert.Contains(t, uploaded, "# 视频标题")
	assert.Contains(t, uploaded, "Source: "+testVideoURL)
	assert.Contains(t, uploaded, "字幕正文")
}

func TestMaybeUploadTranscriptFailureDegradesToEmpty(t *testing.T) {
	store := newFakeMediaStore()
	store.upErr = errors.New("r2 put: connection refused")
	deps := &dependencies{r2: store}

	got := maybeUploadTranscript(context.Background(), deps, testVideoURL, videoFetchResult(transcriptSectionMarker+"\n\n字幕"))

	assert.Empty(t, got, "a failed upload must not fail the entry")
}

func TestAttachTranscriptURLOnlySetsOnItem(t *testing.T) {
	pending := pendingURLWrite{Kind: pendingFetchFailure}
	attachTranscriptURL(&pending, "https://media.example/x.md")
	assert.Empty(t, pending.Item, "failure pendings carry no item to attach to")

	item := &wikitypes.ClassifyItem{URL: testVideoURL}
	pending = pendingURLWrite{Kind: pendingSummary, Item: item}
	attachTranscriptURL(&pending, "https://media.example/x.md")
	assert.Equal(t, "https://media.example/x.md", item.TranscriptURL)
}
