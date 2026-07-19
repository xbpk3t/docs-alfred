package wikiingest

import (
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
