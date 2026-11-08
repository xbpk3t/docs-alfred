package fetch

import (
	"context"
	"fmt"
	"github.com/xbpk3t/docs-alfred/internal/docs/wiki/types"
)

// ContentDriver abstracts content fetching for different environments.
// Each driver encapsulates its own URL routing and extraction logic.
type ContentDriver interface {
	// Name returns the driver name.
	Name() string
	// FetchContent retrieves content for a URL.
	FetchContent(ctx context.Context, urlStr string, contentType string) *types.ContentFetchResult
}

// HTTPFallbackUser is implemented by drivers that only own part of the URL
// space and rely on the static HTTP driver for the rest. It is optional so
// test doubles stay minimal.
type HTTPFallbackUser interface {
	// UsesHTTPFallback reports whether the driver relies on the static HTTP
	// fallback for URLs it does not own.
	UsesHTTPFallback() bool
}

// UsesHTTPFallback reports whether driver wants the static HTTP driver
// installed as a fallback for URLs it does not own.
func UsesHTTPFallback(driver ContentDriver) bool {
	user, ok := driver.(HTTPFallbackUser)

	return ok && user.UsesHTTPFallback()
}

// Compile-time interface assertions.
var (
	_ ContentDriver    = (*openCLIDriver)(nil)
	_ ContentDriver    = (*httpDriver)(nil)
	_ ContentDriver    = (*ytdlpDriver)(nil)
	_ HTTPFallbackUser = (*openCLIDriver)(nil)
	_ HTTPFallbackUser = (*ytdlpDriver)(nil)
)

// NewDriver creates a ContentDriver by name.
func NewDriver(name string, opts DriverOptions) (ContentDriver, error) {
	switch name {
	case "opencli":
		return newOpenCLIDriver(opts), nil
	case "http-readability":
		return newHTTPDriver(opts), nil
	case "ytdlp":
		return newYTDLPDriver(opts), nil
	default:
		return nil, fmt.Errorf("unknown driver: %s", name)
	}
}

// DriverOptions holds shared configuration for drivers.
type DriverOptions struct {
	YTDLPCookiesBrowser string
	YTDLPSubtitleLangs  []string
	MaxBodySize         int
	BrowserConcurrency  int
	MediaEnabled        bool
}
