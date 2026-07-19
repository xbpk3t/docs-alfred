package wikiingest

import (
	"strings"

	wikitypes "github.com/xbpk3t/docs-alfred/internal/docs/wiki/types"
)

type fetchFailureError struct {
	failureType wikitypes.FailureKind
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

func failureKindForFetchResult(result *wikitypes.ContentFetchResult) wikitypes.FailureKind {
	if result != nil && result.FailureKind != "" {
		return result.FailureKind
	}
	if result == nil {
		return wikitypes.FailureFetch
	}

	return legacyFailureKindFromMessage(result.Error)
}

func legacyFailureKindFromMessage(message string) wikitypes.FailureKind {
	// Compatibility for test doubles or older fetcher implementations that only
	// return the pre-typed error string. Real fetchers should set FailureKind.
	if strings.Contains(message, "extract:") {
		return wikitypes.FailureExtract
	}
	if strings.Contains(message, "resolve:") {
		return wikitypes.FailureResolve
	}

	return wikitypes.FailureFetch
}
