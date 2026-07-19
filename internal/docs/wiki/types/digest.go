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

// DigestEntry records a single pipeline outcome for one URL.
type DigestEntry struct {
	Timestamp      string       `json:"timestamp"`
	URL            string       `json:"url"`
	BatchID        string       `json:"batchId,omitempty"`
	Stage          DigestStage  `json:"stage"`
	Status         DigestStatus `json:"status"`
	FailureKind    string       `json:"failureKind,omitempty"`
	Error          string       `json:"error,omitempty"`
	TopicPath      string       `json:"topicPath,omitempty"`
	SuggestedTopic string       `json:"suggestedTopic,omitempty"`
	Reason         string       `json:"reason,omitempty"`
	OutputPath     string       `json:"outputPath,omitempty"`
}
