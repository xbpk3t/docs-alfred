package wikiingest

import (
	wikisvc "github.com/xbpk3t/docs-alfred/service/wiki"
)

type serviceWriter struct{}

func (serviceWriter) WriteSummary(item *wikisvc.ClassifyItem, opts *wikisvc.WriteOptions) (string, error) {
	return wikisvc.WriteSummary(item, opts)
}

func (serviceWriter) WriteFailureEntry(
	item *wikisvc.ClassifyItem,
	failureType wikisvc.FailureKind,
	extraInfo string,
	opts *wikisvc.WriteOptions,
) (string, error) {
	return wikisvc.WriteFailureEntry(item, failureType, extraInfo, opts)
}

func (serviceWriter) WriteManualReviewEntry(
	item *wikisvc.ClassifyItem,
	opts *wikisvc.WriteOptions,
) (string, error) {
	return wikisvc.WriteManualReviewEntry(item, opts)
}

type serviceInboxStore struct{}

func (serviceInboxStore) ParseInbox(filePath string) ([]wikisvc.InboxEntry, error) {
	return wikisvc.ParseInbox(filePath)
}

func (serviceInboxStore) FlushInbox(filePath string, handledURLsByLine map[int][]string) error {
	return wikisvc.FlushInbox(filePath, handledURLsByLine)
}
