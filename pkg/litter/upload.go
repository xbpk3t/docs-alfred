// Package litter provides temporary file upload with automatic fallback across
// multiple hosting services (Litterbox, 0x0.st, file.io). Drivers are configured
// via rss2nl.yml; the Fallback uploader tries each in order until one succeeds.
package litter

import (
	"context"
	"errors"
	"fmt"
)

// Result holds the outcome of a successful upload.
type Result struct {
	URL        string `json:"url"`
	Driver     string `json:"driver"`
	Expiration string `json:"expiration,omitempty"`
}

// Uploader is the interface every upload driver implements.
type Uploader interface {
	// Upload posts content and returns a publicly accessible URL.
	Upload(ctx context.Context, filename, content string) (*Result, error)
	// Name returns the driver identifier (e.g. "litterbox", "zerox0", "fileio").
	Name() string
}

// Fallback tries each uploader in order, returning the first success.
// If all fail, it returns a combined error with every driver's failure reason.
type Fallback struct {
	uploaders []Uploader
}

// NewFallback creates a Fallback from the given drivers.
func NewFallback(uploaders ...Uploader) *Fallback {
	return &Fallback{uploaders: uploaders}
}

// Upload tries each uploader sequentially. On the first success it returns;
// on failure it logs the error and tries the next.
func (f *Fallback) Upload(ctx context.Context, filename, content string) (*Result, error) {
	var errs []error
	for _, u := range f.uploaders {
		result, err := u.Upload(ctx, filename, content)
		if err == nil {
			return result, nil
		}
		errs = append(errs, fmt.Errorf("%s: %w", u.Name(), err))
	}

	return nil, fmt.Errorf("all upload drivers failed: %w", errors.Join(errs...))
}

// Name returns "fallback".
func (f *Fallback) Name() string { return "fallback" }

// NewFromNames creates a Fallback from driver names and expiration.
// Supported names: "litterbox", "zerox0", "fileio".
// Unknown names are silently skipped.
func NewFromNames(names []string, expiration string) *Fallback {
	var uploaders []Uploader
	for _, name := range names {
		switch name {
		case "litterbox":
			uploaders = append(uploaders, NewLitterbox(expiration))
		case "zerox0":
			uploaders = append(uploaders, &ZeroX0{})
		case "fileio":
			uploaders = append(uploaders, &FileIO{})
		}
	}
	if len(uploaders) == 0 {
		// Default chain when config is empty.
		uploaders = []Uploader{
			NewLitterbox(expiration),
			&ZeroX0{},
			&FileIO{},
		}
	}

	return NewFallback(uploaders...)
}
