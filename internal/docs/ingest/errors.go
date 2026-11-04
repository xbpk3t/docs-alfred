package wikiingest

import (
	"strings"

	wikitypes "github.com/xbpk3t/docs-alfred/internal/docs/wiki/types"
	"github.com/xbpk3t/docs-alfred/pkg/textutil"
)

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

func failureReasonForFetchResult(result *wikitypes.ContentFetchResult) wikitypes.FailureReason {
	if result != nil && result.FailureReason != "" {
		return result.FailureReason
	}
	if result == nil {
		// A nil driver result is a bug/cancellation, not proof that the URL has
		// permanently empty content. Keep the inbox line for a retry.
		return wikitypes.FailureReasonHTTPError
	}
	kind := failureKindForFetchResult(result)
	message := strings.ToLower(result.Error)
	if kind == wikitypes.FailureResolve || kind == wikitypes.FailureFetch {
		if strings.Contains(message, "404") || strings.Contains(message, "410") {
			return wikitypes.FailureReasonNotFound
		}
		// Permanent client errors. 401/403 are deliberately left retryable:
		// they usually mean an expired session/anti-bot wall.
		if textutil.ContainsAny(message, []string{
			"400 bad request", "http 400", "status 400",
			"405 method not allowed", "http 405",
			"406 not acceptable", "http 406",
			"422 unprocessable", "http 422",
			"451 unavailable for legal reasons", "http 451",
		}) {
			return wikitypes.FailureReasonHTTPClientError
		}
	}

	if kind == wikitypes.FailureExtract {
		// Drivers that know an extract failure is transient set FailureReason
		// explicitly (partial_extract). An untyped extract error is treated like
		// the pre-typed behavior: permanent, so the inbox line can drain.
		return wikitypes.FailureReasonEmpty
	}

	return wikitypes.FailureReasonHTTPError
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
