package wiki

import (
	"context"
	"fmt"
)

// ContentDriver abstracts content fetching for different environments.
// Each driver encapsulates its own URL routing and extraction logic.
type ContentDriver interface {
	// Name returns the driver name.
	Name() string
	// FetchContent retrieves content for a URL.
	FetchContent(ctx context.Context, urlStr string, contentType string) *ContentFetchResult
}

// NewDriver creates a ContentDriver by name.
func NewDriver(name string, opts DriverOptions) (ContentDriver, error) {
	switch name {
	case "opencli":
		return newOpenCLIDriver(opts), nil
	case "http-readability":
		return newHTTPDriver(opts), nil
	default:
		return nil, fmt.Errorf("unknown driver: %s", name)
	}
}

// DriverOptions holds shared configuration for drivers.
type DriverOptions struct {
	MaxBodySize  int
	MediaEnabled bool
}
