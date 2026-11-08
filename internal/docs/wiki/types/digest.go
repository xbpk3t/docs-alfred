package types

// DigestStage identifies which pipeline stage generated the entry.
type DigestStage string

const (
	StageFetch    DigestStage = "fetch"
	StageExtract  DigestStage = "extract"
	StageClassify DigestStage = "classify"
	StageWrite    DigestStage = "write"
)

// DigestStatus indicates success or failure.
type DigestStatus string

const (
	DigestSuccess DigestStatus = "success"
	DigestFailure DigestStatus = "failure"
)

// FailureReason is a stable, machine-readable cause code for a failed digest.
// It is deliberately finer-grained than FailureKind so retry policy and stats
// can distinguish "the site blocked us" from "the URL is gone" without parsing
// Error text.
type FailureReason string

const (
	FailureReasonAntiBot         FailureReason = "anti_bot"
	FailureReasonCloudflare      FailureReason = "cloudflare_challenge"
	FailureReasonJSShell         FailureReason = "js_shell"
	FailureReasonBrowserError    FailureReason = "browser_error_page"
	FailureReasonHTTPError       FailureReason = "http_error"
	FailureReasonHTTPClientError FailureReason = "http_client_error"
	FailureReasonNotFound        FailureReason = "not_found"
	FailureReasonPDFUnsupported  FailureReason = "pdf_unsupported"
	// FailureReasonLinkOnlyPost is a deterministic X/Twitter post whose only
	// content is a link to another status. It cannot improve on retry.
	FailureReasonLinkOnlyPost   FailureReason = "link_only_post"
	FailureReasonPartialExtract FailureReason = "partial_extract"
	FailureReasonTooShort       FailureReason = "too_short"
	FailureReasonIndexPage      FailureReason = "index_page"
	FailureReasonEmpty          FailureReason = "empty"
	FailureReasonClassify       FailureReason = "classify_failed"
	FailureReasonAI             FailureReason = "ai_unavailable"
	// FailureReasonCatalog means the topic catalog was unavailable before the
	// AI was called. It is transient and must not be reported as an AI outage.
	FailureReasonCatalog FailureReason = "catalog_unavailable"
	// FailureReasonPaidContent is a video/page that requires payment/charging
	// before its content can be read. Retrying cannot change the outcome.
	FailureReasonPaidContent FailureReason = "paid_content"
	// FailureReasonAuthRequired means the site session is missing or lacks
	// permission. It is environment-dependent and stays retryable.
	FailureReasonAuthRequired FailureReason = "auth_required"
	// FailureReasonBrowserUnreachable is a terminal browser error page
	// (DNS/TLS/connection refused); retrying the same URL cannot recover it.
	FailureReasonBrowserUnreachable FailureReason = "browser_unreachable"
	// FailureReasonRetryExhausted is the terminal outcome after repeated
	// environment failures (for example auth_required) exhausted the budget.
	FailureReasonRetryExhausted FailureReason = "retry_exhausted"
)

// DigestRunEntry is one line in logs/digest-runs.jsonl: a run-level summary
// written once per digest run, including aborted runs.
type DigestRunEntry struct {
	FailureByReason map[string]int `json:"failureByReason,omitempty"`
	Timestamp       string         `json:"timestamp"`
	BatchID         string         `json:"batchId,omitempty"`
	StartedAt       string         `json:"startedAt"`
	Error           string         `json:"error,omitempty"`
	AbortReason     string         `json:"abortReason,omitempty"`
	Flushed         int            `json:"flushed"`
	Retryable       int            `json:"retryable"`
	Permanent       int            `json:"permanent"`
	Unhandled       int            `json:"unhandled"`
	Skipped         int            `json:"skipped"`
	Written         int            `json:"written"`
	Succeeded       int            `json:"succeeded"`
	Processed       int            `json:"processed"`
	DurationMs      int64          `json:"durationMs"`
	DryRun          bool           `json:"dryRun,omitempty"`
}

// DigestEntry records a single pipeline outcome for one URL.
type DigestEntry struct {
	Timestamp      string        `json:"timestamp"`
	URL            string        `json:"url"`
	BatchID        string        `json:"batchId,omitempty"`
	Stage          DigestStage   `json:"stage"`
	Status         DigestStatus  `json:"status"`
	FailureKind    string        `json:"failureKind,omitempty"`
	FailureReason  FailureReason `json:"failureReason,omitempty"`
	Error          string        `json:"error,omitempty"`
	TopicPath      string        `json:"topicPath,omitempty"`
	SuggestedTopic string        `json:"suggestedTopic,omitempty"`
	Reason         string        `json:"reason,omitempty"`
	OutputPath     string        `json:"outputPath,omitempty"`
}
