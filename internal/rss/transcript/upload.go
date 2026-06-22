// Package transcript handles podcast transcription and summarization.
// Upload functionality delegates to pkg/litter.
package transcript

import (
	"context"

	"github.com/xbpk3t/docs-alfred/pkg/litter"
)

// LitterboxUploader is a thin wrapper around litter.Litterbox for backward compatibility.
// New code should use litter.Uploader / litter.Fallback directly.
type LitterboxUploader struct {
	inner *litter.Litterbox
}

// UploadResult holds the result of a Litterbox upload.
// Kept for backward compatibility; new code should use litter.Result.
type UploadResult = litter.Result

// NewLitterboxUploader creates an uploader with the given expiration duration.
func NewLitterboxUploader(expiration string) *LitterboxUploader {
	return &LitterboxUploader{inner: litter.NewLitterbox(expiration)}
}

// Upload delegates to litter.Litterbox.
func (u *LitterboxUploader) Upload(ctx context.Context, filename, content string) (*UploadResult, error) {
	return u.inner.Upload(ctx, filename, content)
}
