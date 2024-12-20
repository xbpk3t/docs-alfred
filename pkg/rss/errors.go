package rss

import "fmt"

// 错误定义
const (
	ErrNoFeedsFound    = "no feeds found"
	ErrInvalidSchedule = "invalid schedule type"
)

// FeedError 自定义错误类型
type FeedError struct {
	URL     string
	Message string
	Err     error
}

func (e *FeedError) Error() string {
	return fmt.Sprintf("feed error for %s: %s: %v", e.URL, e.Message, e.Err)
}
