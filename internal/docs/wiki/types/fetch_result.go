package types

// ContentFetchResult holds fetched content metadata and body.
type ContentFetchResult struct {
	Title       string      `json:"title"`
	Body        string      `json:"body"`
	SourceURL   string      `json:"sourceUrl"`
	Error       string      `json:"error,omitempty"`
	FailureKind FailureKind `json:"-"`
}
