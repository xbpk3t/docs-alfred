package workspaceops

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/xbpk3t/docs-alfred/internal/docs/workspace/blog"
	"github.com/xbpk3t/docs-alfred/internal/docs/workspace/images"
	"github.com/xbpk3t/docs-alfred/pkg/checkutil"
)

// ImagesCheckInput holds input for images check.
type ImagesCheckInput struct {
	DataDir     string
	ImagesDir   string
	Apply       bool
	List        bool
	SkipExtra   bool
	SkipMissing bool
}

// ImagesCheckResult holds images check results.
type ImagesCheckResult struct {
	ExpectedDirs   []string
	ExistingDirs   []string
	MissingDirs    []string
	ExtraDirs      []string
	DuplicateFiles []string
	Warnings       []string
	Errors         []string
	ApplyActions   []string
}

func imagesCheckConfig(input ImagesCheckInput) images.CheckConfig {
	return images.CheckConfig{
		DataDir:     input.DataDir,
		ImagesDir:   input.ImagesDir,
		Apply:       input.Apply,
		List:        input.List,
		SkipExtra:   input.SkipExtra,
		SkipMissing: input.SkipMissing,
	}
}

func imagesCheckResult(result *ImagesCheckResult) *images.CheckResult {
	return &images.CheckResult{
		ExpectedDirs:   result.ExpectedDirs,
		ExistingDirs:   result.ExistingDirs,
		MissingDirs:    result.MissingDirs,
		ExtraDirs:      result.ExtraDirs,
		DuplicateFiles: result.DuplicateFiles,
		Warnings:       result.Warnings,
		Errors:         result.Errors,
		ApplyActions:   result.ApplyActions,
	}
}

// Issues returns images check issues using the common checkutil shape.
func (r *ImagesCheckResult) Issues(input ImagesCheckInput) []checkutil.Issue {
	return imagesCheckResult(r).Issues(imagesCheckConfig(input))
}

// Summary returns count-oriented images check details for structured output.
func (r *ImagesCheckResult) Summary() map[string]any {
	return map[string]any{
		"expectedDirs":   len(r.ExpectedDirs),
		"existingDirs":   len(r.ExistingDirs),
		"missingDirs":    len(r.MissingDirs),
		"extraDirs":      len(r.ExtraDirs),
		"duplicateFiles": len(r.DuplicateFiles),
		"warnings":       len(r.Warnings),
		"errors":         len(r.Errors),
	}
}

// RunImagesCheck checks docs-images against data/gh expectations.
func RunImagesCheck(input ImagesCheckInput) (*ImagesCheckResult, error) {
	slog.Info("Checking docs-images", "data-dir", input.DataDir, "images-dir", input.ImagesDir)

	result, err := images.RunImagesCheck(imagesCheckConfig(input))
	if err != nil {
		return nil, err
	}

	return &ImagesCheckResult{
		ExpectedDirs:   result.ExpectedDirs,
		ExistingDirs:   result.ExistingDirs,
		MissingDirs:    result.MissingDirs,
		ExtraDirs:      result.ExtraDirs,
		DuplicateFiles: result.DuplicateFiles,
		Warnings:       result.Warnings,
		Errors:         result.Errors,
		ApplyActions:   result.ApplyActions,
	}, nil
}

// FormatImagesReport formats images check result for display.
func FormatImagesReport(result *ImagesCheckResult, input ImagesCheckInput) string {
	return imagesCheckResult(result).ReportResult(imagesCheckConfig(input))
}

// FormatImagesDetails formats non-status images check details for text output.
func FormatImagesDetails(result *ImagesCheckResult, input ImagesCheckInput) string {
	var out strings.Builder
	fmt.Fprintf(&out, "summary: expected=%d existing=%d missing=%d extra=%d duplicates=%d\n",
		len(result.ExpectedDirs), len(result.ExistingDirs), len(result.MissingDirs),
		len(result.ExtraDirs), len(result.DuplicateFiles))

	if input.List {
		for _, d := range result.ExpectedDirs {
			fmt.Fprintf(&out, "expected: %s\n", d)
		}
		for _, d := range result.ExistingDirs {
			fmt.Fprintf(&out, "existing: %s\n", d)
		}
	}

	return out.String()
}

// BlogCheckInput holds input for blog check.
type BlogCheckInput struct {
	DataDir string
	BlogDir string
}

// BlogCheckResult holds blog check results.
type BlogCheckResult struct {
	Issues   []checkutil.Issue
	GHTypes  int
	BlogDirs int
}

// Summary returns count-oriented blog check details for structured output.
func (r *BlogCheckResult) Summary() map[string]any {
	return map[string]any{
		"ghTypes":  r.GHTypes,
		"blogDirs": r.BlogDirs,
	}
}

// RunBlogCheck checks blog/data consistency.
func RunBlogCheck(input BlogCheckInput) (*BlogCheckResult, error) {
	result, err := blog.RunCheck(input.DataDir, input.BlogDir)
	if err != nil {
		return nil, err
	}

	return &BlogCheckResult{
		Issues:   result.Issues,
		GHTypes:  result.GHTypes,
		BlogDirs: result.BlogDirs,
	}, nil
}
