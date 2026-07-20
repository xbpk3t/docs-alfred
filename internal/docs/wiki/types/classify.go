package types

// RouteReason values explain why an item went to uncat.md (or related routes).
const (
	RouteReasonNeedsManualReview = "needs_manual_review"
	RouteReasonNoTopicMatch      = "no_topic_match"
	RouteReasonInvalidTopicPath  = "invalid_topic_path"
	RouteReasonAIUnavailable     = "ai_classify_unavailable"
)

// ClassifyType represents the wiki entry type.
type ClassifyType string

const (
	TypeRepoEval ClassifyType = "review"
	TypeDeepDive ClassifyType = "research"
	TypeInbox    ClassifyType = "inbox"
)

// ClassifyItem holds the full classification result for a URL.
type ClassifyItem struct {
	URL               string             `json:"url"`
	Title             string             `json:"title"`
	ContentType       string             `json:"contentType"`
	TopicPath         string             `json:"topicPath"`
	Type              ClassifyType       `json:"type"`
	Summary           *StructuredSummary `json:"summary"`
	MetadataBlock     string             `json:"metadataBlock,omitempty"`
	SuggestedTopic    string             `json:"suggestedTopic,omitempty"`
	RouteReason       string             `json:"routeReason,omitempty"`
	Confidence        float64            `json:"confidence,omitempty"`
	NeedsManualReview bool               `json:"needsManualReview,omitempty"`
}

// ClassifyResult is the structured output from classifyItem.
type ClassifyResult struct {
	TopicPath         string             `json:"topicPath"`
	WikiType          ClassifyType       `json:"wikiType"`
	ContentType       string             `json:"contentType"`
	Summary           *StructuredSummary `json:"summary"`
	MetadataBlock     string             `json:"metadataBlock,omitempty"`
	RejectReason      string             `json:"rejectReason,omitempty"`
	SuggestedTopic    string             `json:"suggestedTopic,omitempty"`
	RouteReason       string             `json:"routeReason,omitempty"`
	Confidence        float64            `json:"confidence,omitempty"`
	NeedsManualReview bool               `json:"needsManualReview,omitempty"`
}

// StructuredSummary holds the AI-generated summary broken into sections.
type StructuredSummary struct {
	Overview         string   `json:"overview"                   validate:"required"`
	Detail           string   `json:"detail,omitempty"`
	KeyQuotes        []string `json:"keyQuotes,omitempty"`
	WorthNoting      string   `json:"worthNoting,omitempty"`
	CriticalThinking string   `json:"criticalThinking,omitempty"`
	KeyPoints        []string `json:"keyPoints"                  validate:"required|min_len:1"`
	ActionableAdvice []string `json:"actionableAdvice,omitempty"`
}

// EntryMetadata holds additional metadata fields from AI classification.
type EntryMetadata struct {
	ContentType       string   `json:"contentType"                 validate:"required|in:text,media,repo"`
	Quality           string   `json:"quality,omitempty"           validate:"quality"`
	Author            string   `json:"author,omitempty"`
	Uncertainties     string   `json:"uncertainties,omitempty"`
	Duration          string   `json:"duration,omitempty"          validate:"duration"`
	TranscriptQuality string   `json:"transcriptQuality,omitempty" validate:"in:good,fair,poor"`
	Verdict           string   `json:"verdict,omitempty"           validate:"in:watch,skip,try"`
	Language          string   `json:"language,omitempty"`
	// [2026-07-21] Drop max_len:8 on tags: AI often returns >8 keywords; hard-failing
	// classification for that alone forced full retries and sank digest success rate.
	// Soft upper bound still lives in classify-json.txt prompt (3-8); only enforce min.
	Tags              []string `json:"tags,omitempty"              validate:"required|min_len:3"`
	Stars             int      `json:"stars,omitempty"`
}
