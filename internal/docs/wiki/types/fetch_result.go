package types

// ContentFetchResult holds fetched content metadata and body.
type ContentFetchResult struct {
	Title       string      `json:"title"`
	Body        string      `json:"body"`
	SourceURL   string      `json:"sourceUrl"`
	Error       string      `json:"error,omitempty"`
	FailureKind FailureKind `json:"-"`
	// ChildURLs are independent sub-posts discovered in this page, each with
	// its own canonical permalink (one zhihu answer inside a question page).
	// The pipeline digests every one as its own entry, fetched by URL, instead
	// of folding them into Body. Empty for the ordinary single-post page.
	//
	// Contract: a child inherits the container's resolved topic and type, and
	// is claimed in the digest history so it is never digested twice. That
	// suits a container whose children are all on its subject (a Q&A thread);
	// a page whose children are unrelated (an index or feed) should not
	// populate this field.
	ChildURLs []string `json:"childUrls,omitempty"`
}
