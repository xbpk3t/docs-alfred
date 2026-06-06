package workspace

import (
	"log/slog"

	"github.com/xbpk3t/docs-alfred/pkg/blog"
	"github.com/xbpk3t/docs-alfred/pkg/checkutil"
	pkgdata "github.com/xbpk3t/docs-alfred/pkg/data"
	"github.com/xbpk3t/docs-alfred/pkg/dotfiles"
	"github.com/xbpk3t/docs-alfred/pkg/images"
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

// RunImagesCheck checks docs-images against data/gh expectations.
func RunImagesCheck(input ImagesCheckInput) (*ImagesCheckResult, error) {
	slog.Info("Checking docs-images", "data-dir", input.DataDir, "images-dir", input.ImagesDir)

	result, err := images.RunImagesCheck(images.CheckConfig{
		DataDir:     input.DataDir,
		ImagesDir:   input.ImagesDir,
		Apply:       input.Apply,
		List:        input.List,
		SkipExtra:   input.SkipExtra,
		SkipMissing: input.SkipMissing,
	})
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
	cfg := images.CheckConfig{
		DataDir:     input.DataDir,
		ImagesDir:   input.ImagesDir,
		Apply:       input.Apply,
		List:        input.List,
		SkipExtra:   input.SkipExtra,
		SkipMissing: input.SkipMissing,
	}
	checkResult := &images.CheckResult{
		ExpectedDirs:   result.ExpectedDirs,
		ExistingDirs:   result.ExistingDirs,
		MissingDirs:    result.MissingDirs,
		ExtraDirs:      result.ExtraDirs,
		DuplicateFiles: result.DuplicateFiles,
		Warnings:       result.Warnings,
		Errors:         result.Errors,
		ApplyActions:   result.ApplyActions,
	}

	return checkResult.ReportResult(cfg)
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

// DotfilesCheckInput holds input for dotfiles check.
type DotfilesCheckInput struct {
	DotfilesPath string
	DataDir      string
}

// DotfilesCheckResult holds dotfiles check results.
type DotfilesCheckResult struct {
	Issues      []checkutil.Issue
	SharedCount int
	DfOnlyCount int
	GhOnlyCount int
}

// RunDotfilesCheck checks dotfiles/data consistency.
func RunDotfilesCheck(input DotfilesCheckInput) (*DotfilesCheckResult, error) {
	dataDir := input.DataDir
	if dataDir == "" {
		dataDir = pkgdata.DefaultPathForDomain(pkgdata.DomainGH)
	}

	result, err := dotfiles.RunCheck(input.DotfilesPath, dataDir)
	if err != nil {
		return nil, err
	}

	return &DotfilesCheckResult{
		Issues:      result.Issues,
		SharedCount: result.SharedCount,
		DfOnlyCount: result.DfOnlyCount,
		GhOnlyCount: result.GhOnlyCount,
	}, nil
}

// DotfilesSyncPlanInput holds input for dotfiles sync-plan.
type DotfilesSyncPlanInput struct {
	DotfilesPath string
	JSON         bool
}

// DotfilesSyncPlanResult holds sync-plan results.
type DotfilesSyncPlanResult struct {
	DotfilesPath string                `json:"dotfilesPath"`
	Error        string                `json:"error,omitempty"`
	ChangedFiles []dotfiles.ChangeFile `json:"changedFiles"`
	OK           bool                  `json:"ok"`
}

// RunDotfilesSyncPlan plans dotfiles synchronization.
func RunDotfilesSyncPlan(input DotfilesSyncPlanInput) *DotfilesSyncPlanResult {
	result := dotfiles.RunSyncPlan(dotfiles.SyncPlanOptions{
		DotfilesPath: input.DotfilesPath,
		JSON:         input.JSON,
	})

	return &DotfilesSyncPlanResult{
		OK:           result.OK,
		Error:        result.Error,
		DotfilesPath: result.DotfilesPath,
		ChangedFiles: result.ChangedFiles,
	}
}
