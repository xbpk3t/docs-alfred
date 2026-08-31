package rankings

import (
	"time"
)

// AllTimeWindow 无时间窗口，所有数据在同一个 key 中。
// 适合累计榜等不需要按时间重置的场景。
type AllTimeWindow struct {
	Name string
}

func (w *AllTimeWindow) CurrentKey() string {
	return w.Name
}

// DailyWindow 按天划分窗口，每天 UTC+tz 自然日重置。
// CurrentKey 返回 "YYYYMMDD" 格式的日期字符串。
type DailyWindow struct {
	Timezone *time.Location
}

func NewDailyWindow(tz *time.Location) *DailyWindow {
	return &DailyWindow{Timezone: tz}
}

func (w *DailyWindow) CurrentKey() string {
	return time.Now().In(w.Timezone).Format("20060102")
}
