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
)

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
