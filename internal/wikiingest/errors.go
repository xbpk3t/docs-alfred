package wikiingest

import (
	"strings"

	wikisvc "github.com/xbpk3t/docs-alfred/service/wiki"
)

type fetchFailureError struct {
	failureType wikisvc.FailureKind
	message     string
}

func (e *fetchFailureError) Error() string {
	return e.message
}

// classifyRetryError is returned when a transient classifcation failure occurs
// (AI timeout, rate limit, invalid response) that may succeed on retry.
type classifyRetryError struct {
	message string
}

func (e *classifyRetryError) Error() string {
	return e.message
}

func failureKindForFetchResult(result *wikisvc.ContentFetchResult) wikisvc.FailureKind {
	if result != nil && result.FailureKind != "" {
		return result.FailureKind
	}
	if result == nil {
		return wikisvc.FailureFetch
	}

	return legacyFailureKindFromMessage(result.Error)
}

func legacyFailureKindFromMessage(message string) wikisvc.FailureKind {
	// Compatibility for test doubles or older fetcher implementations that only
	// return the pre-typed error string. Real fetchers should set FailureKind.
	if strings.Contains(message, "extract:") {
		return wikisvc.FailureExtract
	}
	if strings.Contains(message, "resolve:") {
		return wikisvc.FailureResolve
	}

	return wikisvc.FailureFetch
}
