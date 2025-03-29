package rss

import "time"

// 时间相关常量
const (
	Daily  = "daily"
	Weekly = "weekly"

	// Cron types
	CronTypeDaily   = "daily"
	CronTypeWeekly  = "weekly"
	CronTypeMonthly = "monthly"
	CronTypeYearly  = "yearly"

	// 重试相关常量
	DefaultRetryDelay = 5 * time.Second
	DefaultMaxRetries = 3
	DefaultFeedLimit  = 10
)

// 日志字段常量
const (
	LogKeyURL       = "url"
	LogKeyAttempts  = "attempts"
	LogKeyError     = "error"
	LogKeyFeedTitle = "feed_title"
)

// 时间范围映射
var scheduleTimeRanges = map[string]int{
	Daily:  24,
	Weekly: 7 * 24,
}
