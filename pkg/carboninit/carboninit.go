// Package carboninit initializes the carbon/v2 timezone and locale for the project.
//
// All binaries that use carbon should call Setup() in their Execute() or main():
//
//	import "github.com/xbpk3t/docs-alfred/pkg/carboninit"
//
//	func Execute() {
//	    carboninit.Setup()
//	    ...
//	}
package carboninit

import (
	"time"

	carbon "github.com/dromara/carbon/v2"
)

// DefaultTimezone is the canonical timezone for the whole project.
// The newsletter/filter/day-boundary logic and reports all render in this
// timezone so they are independent of the runner/process local timezone.
const DefaultTimezone = "Asia/Shanghai"

// Setup sets carbon/v2 defaults for timezone and locale.
// Must be called once per process, typically in Execute() or main().
func Setup() {
	carbon.SetTimezone(DefaultTimezone)
	carbon.SetLocale("zh-CN")
}

// Location returns the project timezone as a *time.Location.
// Asia/Shanghai is a fixed UTC+8 offset with no DST, so this cannot fail; a
// FixedZone fallback keeps the binary independent of the host's tzdata.
func Location() *time.Location {
	loc, err := time.LoadLocation(DefaultTimezone)
	if err != nil {
		return time.FixedZone(DefaultTimezone, 8*3600)
	}

	return loc
}

// In returns a Carbon for t normalized to DefaultTimezone.
// The absolute instant is unchanged, but both the internal time and the
// formatting timezone are set to Asia/Shanghai, so formatting
// (ToDateTimeString/ToDateString) and day-boundary math (StartOfDay) render in
// Asia/Shanghai regardless of the source time's location.
func In(t time.Time) *carbon.Carbon {
	return carbon.CreateFromStdTime(t.In(Location())).SetTimezone(DefaultTimezone)
}
