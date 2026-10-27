package wikiingest

import (
	"context"

	wikitypes "github.com/xbpk3t/docs-alfred/internal/docs/wiki/types"
	wikiwrite "github.com/xbpk3t/docs-alfred/internal/docs/wiki/write"
)

type serviceWriter struct{}

func (serviceWriter) WriteSummary(item *wikitypes.ClassifyItem, opts *wikiwrite.WriteOptions) (string, error) {
	return wikiwrite.WriteSummary(item, opts)
}

func (serviceWriter) WriteFailureEntry(
	item *wikitypes.ClassifyItem,
	failureType wikitypes.FailureKind,
	extraInfo string,
	opts *wikiwrite.WriteOptions,
) (string, error) {
	return wikiwrite.WriteFailureEntry(item, failureType, extraInfo, opts)
}

func (serviceWriter) WriteManualReviewEntry(
	item *wikitypes.ClassifyItem,
	opts *wikiwrite.WriteOptions,
) (string, error) {
	return wikiwrite.WriteManualReviewEntry(item, opts)
}

type serviceInboxStore struct{}

func (serviceInboxStore) ParseInbox(filePath string) ([]wikiwrite.InboxEntry, error) {
	return wikiwrite.ParseInbox(filePath)
}

func (serviceInboxStore) FlushInbox(filePath string, handledURLsByLine map[int][]string) error {
	return wikiwrite.FlushInbox(filePath, handledURLsByLine)
}

// digestStoreAdapter adapts the media store (which returns *litter.Result from
// Upload) to the write package's DigestStore (which wants a URL string), so the
// write layer stays decoupled from the R2 driver type.
type digestStoreAdapter struct {
	store mediaStore
}

func (a digestStoreAdapter) Upload(ctx context.Context, key, content string) (string, error) {
	res, err := a.store.Upload(ctx, key, content)
	if err != nil {
		return "", err
	}

	return res.URL, nil
}

func (a digestStoreAdapter) GetObject(ctx context.Context, key string) ([]byte, error) {
	return a.store.GetObject(ctx, key)
}

// digestStoreOrNil returns the write-layer DigestStore for a run's R2 store, or
// nil when R2 is not configured (digest artifacts then stay local).
func digestStoreOrNil(deps *dependencies) wikiwrite.DigestStore {
	if deps == nil || deps.r2 == nil {
		return nil
	}

	return digestStoreAdapter{store: deps.r2}
}
