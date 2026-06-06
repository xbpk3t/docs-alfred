package rss

import "fmt"

// 错误定义.
const (
	ErrNoFeedsFound    = "no feeds found"
	ErrInvalidSchedule = "invalid schedule type"
)

// FeedError 自定义错误类型.
type FeedError struct {
	Err     error
	URL     string
	Message string
	Kind    FeedFailureKind
	// Transient marks source/network failures that should be grouped and
	// escalated only after repeated runs.
	Transient bool
}

func (e *FeedError) Error() string {
	return fmt.Sprintf("feed error for %s: %s: %v", e.URL, e.Message, e.Err)
}
