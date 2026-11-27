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
	// DefaultFeedFetchConcurrency controls concurrent feed fetches per group.
	DefaultFeedFetchConcurrency = 10
	// DefaultHostFetchConcurrency is per-host in-flight when no hosts[] rule matches.
	DefaultHostFetchConcurrency = 4
	// DefaultFallbackMaxAttempts is intentionally low: the relay is the last
	// resort and a daily run can retry on the next schedule.
	DefaultFallbackMaxAttempts = 1
)

// Fallback relay constants.
const (
	// DefaultFallbackRelay fetches a feed server-side and returns JSON Feed,
	// which gofeed parses natively. The {url} placeholder is URL-encoded.
	DefaultFallbackRelay = "https://feed2json.org/convert?url={url}"
	// fallbackURLPlaceholder is replaced with url.QueryEscape(originalURL).
	fallbackURLPlaceholder = "{url}"
	// FetchedViaDirect / FetchedViaRelay describe how a feed was fetched.
	FetchedViaDirect = "direct"
	FetchedViaRelay  = "relay"
)

// 日志字段常量.
const (
	LogKeyURL      = "url"
	LogKeyAttempts = "attempts"
	LogKeyError    = "error"
	LogKeyRelay    = "relay"
)

// GetScheduleTimeRanges returns the time range mapping.
func GetScheduleTimeRanges() map[string]int {
	return map[string]int{
		Daily:  24,
		Weekly: 7 * 24,
	}
}
