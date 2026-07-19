// Package types holds shared wiki DTOs and constants with no business logic.
package types

// FailureKind classifies wiki ingestion failures.
type FailureKind string

// Failure type constants for WriteFailureEntry / fetch results.
const (
	FailureFetch    FailureKind = "fetch"
	FailureResolve  FailureKind = "resolve"
	FailureExtract  FailureKind = "extract"
	FailureClassify FailureKind = "classify"
	FailureAI       FailureKind = "ai"
)

// String returns the stable wire value for the failure kind.
func (k FailureKind) String() string { return string(k) }
