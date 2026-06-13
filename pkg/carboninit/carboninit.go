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

import carbon "github.com/dromara/carbon/v2"

// Setup sets carbon/v2 defaults for timezone and locale.
// Must be called once per process, typically in Execute() or main().
func Setup() {
	carbon.SetTimezone("Asia/Shanghai")
	carbon.SetLocale("zh-CN")
}
