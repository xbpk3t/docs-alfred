package rss

import "time"

// 时间相关常量.
const (
	Daily  = "daily"
	Weekly = "weekly"
)

// 重试相关常量.
const (
	// DefaultRetryDelay 默认重试延迟时间.
	DefaultRetryDelay = 5 * time.Second
	// DefaultMaxRetries 默认最大重试次数.
	DefaultMaxRetries = 3
	// DefaultFeedLimit 默认Feed数量限制.
	DefaultFeedLimit = 10
)

// 日志字段常量.
const (
	LogKeyURL       = "url"
	LogKeyAttempts  = "attempts"
	LogKeyError     = "error"
	LogKeyFeedTitle = "feed_title"
)

// GetScheduleTimeRanges returns the time range mapping.
func GetScheduleTimeRanges() map[string]int {
	return map[string]int{
		Daily:  24,
		Weekly: 7 * 24,
	}
}
