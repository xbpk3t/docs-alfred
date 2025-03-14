package rss

// DashboardData represents all dashboard information
type DashboardData struct {
	FailedFeeds []FailedFeedInfo `json:"failedFeeds,omitempty"`
	FeedDetails []FeedDetailInfo `json:"feedDetails,omitempty"`
}

// FailedFeedInfo represents information about a failed feed
type FailedFeedInfo struct {
	URL   string `json:"url"`
	Error string `json:"error"`
}

// FeedDetailInfo represents detailed information about a feed
type FeedDetailInfo struct {
	Type  string `json:"type"`
	URL   string `json:"url"`
	Name  string `json:"name"`
	Count int    `json:"count"`
}
