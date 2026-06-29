package cmd

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	workspaceuc "github.com/xbpk3t/docs-alfred/internal/docs/check"
	"github.com/xbpk3t/docs-alfred/pkg/checkutil"
)

// --- writeImagesCheckResult: text format ---

func TestWriteImagesCheckResult_Passed(t *testing.T) {
	stdout := captureStdout(t)

	err := writeImagesCheckResult("images check", &workspaceuc.ImagesCheckResult{}, workspaceuc.ImagesCheckInput{}, "text", nil)
	require.NoError(t, err)

	out := stdout()
	assert.Contains(t, out, "images check passed")
}

func TestWriteImagesCheckResult_WithErrors(t *testing.T) {
	err := writeImagesCheckResult("images check", &workspaceuc.ImagesCheckResult{
		Errors: []string{"something went wrong"},
	}, workspaceuc.ImagesCheckInput{}, "text", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "images check failed")
}

func TestWriteImagesCheckResult_WarningOnly(t *testing.T) {
	stdout := captureStdout(t)

	err := writeImagesCheckResult("images check", &workspaceuc.ImagesCheckResult{
		Warnings: []string{"something suspicious"},
	}, workspaceuc.ImagesCheckInput{}, "text", nil)
	require.NoError(t, err, "warnings should not cause failure")

	out := stdout()
	assert.Contains(t, out, "something suspicious")
}

func TestWriteImagesCheckResult_MissingDirsFail(t *testing.T) {
	err := writeImagesCheckResult("images check", &workspaceuc.ImagesCheckResult{
		MissingDirs: []string{"some/missing/dir"},
	}, workspaceuc.ImagesCheckInput{}, "text", nil)
	require.Error(t, err, "missing dirs are errors and should cause failure")
}

func TestWriteImagesCheckResult_WithActions(t *testing.T) {
	stdout := captureStdout(t)

	err := writeImagesCheckResult("images fix", &workspaceuc.ImagesCheckResult{
		ApplyActions: []string{"Hidden 1 extra director(ies)", "Moved 2 file(s) to .temp"},
	}, workspaceuc.ImagesCheckInput{}, "text", []string{"Hidden 1 extra director(ies)", "Moved 2 file(s) to .temp"})
	require.NoError(t, err)

	out := stdout()
	assert.Contains(t, out, "[actions]")
	assert.Contains(t, out, "Hidden 1 extra director(ies)")
	assert.Contains(t, out, "Moved 2 file(s) to .temp")
}

func TestWriteImagesCheckResult_DuplicateFiles(t *testing.T) {
	stdout := captureStdout(t)

	err := writeImagesCheckResult("images check", &workspaceuc.ImagesCheckResult{
		DuplicateFiles: []string{"algo/algo/photo__1.jpg"},
	}, workspaceuc.ImagesCheckInput{}, "text", nil)
	require.NoError(t, err)

	out := stdout()
	assert.Contains(t, out, "WARN algo/algo/photo__1.jpg")
	assert.Contains(t, out, "duplicate image file")
}

// --- writeImagesCheckResult: JSON format ---

func TestWriteImagesCheckResult_JSONPassed(t *testing.T) {
	stdout := captureStdout(t)

	err := writeImagesCheckResult("images check", &workspaceuc.ImagesCheckResult{
		ExpectedDirs: []string{"a/b"},
		ExistingDirs: []string{"a/b"},
	}, workspaceuc.ImagesCheckInput{}, "json", nil)
	require.NoError(t, err)

	var result map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout()), &result))
	assert.Equal(t, "images check", result["name"])
	assert.Equal(t, true, result["ok"])
}

func TestWriteImagesCheckResult_JSONWithErrors(t *testing.T) {
	stdout := captureStdout(t)

	err := writeImagesCheckResult("images check", &workspaceuc.ImagesCheckResult{
		Errors: []string{"uh oh"},
	}, workspaceuc.ImagesCheckInput{}, "json", nil)
	require.Error(t, err)

	var result map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout()), &result))
	assert.Equal(t, false, result["ok"])
}

func TestWriteImagesCheckResult_JSONSummary(t *testing.T) {
	stdout := captureStdout(t)

	err := writeImagesCheckResult("images check", &workspaceuc.ImagesCheckResult{
		ExpectedDirs: []string{"a/b"},
		ExistingDirs: []string{"a/b", "extra"},
		MissingDirs:  []string{"b/c"},
		ExtraDirs:    []string{"extra"},
	}, workspaceuc.ImagesCheckInput{}, "json", nil)
	require.Error(t, err)

	var result map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout()), &result))
	assert.Equal(t, float64(1), result["summary"].(map[string]any)["expectedDirs"])
	assert.Equal(t, float64(2), result["summary"].(map[string]any)["existingDirs"])
	assert.Equal(t, float64(1), result["summary"].(map[string]any)["missingDirs"])
	assert.Equal(t, float64(1), result["summary"].(map[string]any)["extraDirs"])
}

// --- writeImagesCheckResult: SkipExtra / SkipMissing ---

func TestWriteImagesCheckResult_SkipMissingHidesErrors(t *testing.T) {
	// Missing dirs are ERROR severity. With SkipMissing=true,
	// they should be excluded from issues, so HasIssueErrors returns false.
	err := writeImagesCheckResult("images check", &workspaceuc.ImagesCheckResult{
		MissingDirs: []string{"some/missing/dir"},
	}, workspaceuc.ImagesCheckInput{SkipMissing: true}, "text", nil)
	require.NoError(t, err, "SkipMissing should suppress missing-dir errors")
}

func TestWriteImagesCheckResult_SkipExtraNoiseGone(t *testing.T) {
	stdout := captureStdout(t)

	err := writeImagesCheckResult("images check", &workspaceuc.ImagesCheckResult{
		ExtraDirs: []string{"some/extra/dir"},
	}, workspaceuc.ImagesCheckInput{SkipExtra: true}, "text", nil)
	require.NoError(t, err, "extra dirs are warnings, not errors anyway")

	out := stdout()
	assert.NotContains(t, out, "some/extra/dir", "SkipExtra should suppress extra-dir warnings in output")
}

func TestWriteImagesCheckResult_MissingWithoutSkipStillErrors(t *testing.T) {
	err := writeImagesCheckResult("images check", &workspaceuc.ImagesCheckResult{
		MissingDirs: []string{"needed/missing"},
	}, workspaceuc.ImagesCheckInput{SkipMissing: false}, "text", nil)
	require.Error(t, err, "missing dirs should still error when SkipMissing=false")
}

// --- writeImagesCheckResult: issues with mixed severity ---

func TestWriteImagesCheckResult_MixedWarnAndError(t *testing.T) {
	err := writeImagesCheckResult("images check", &workspaceuc.ImagesCheckResult{
		Warnings: []string{"warn1"},
		Errors:   []string{"err1"},
	}, workspaceuc.ImagesCheckInput{}, "text", nil)
	require.Error(t, err, "presence of any error should fail regardless of warnings")

	stdout := captureStdout(t)
	_ = writeImagesCheckResult("images check", &workspaceuc.ImagesCheckResult{
		Warnings: []string{"warn1"},
		Errors:   []string{"err1"},
	}, workspaceuc.ImagesCheckInput{}, "text", nil)

	out := stdout()
	assert.Contains(t, out, "warn1")
	assert.Contains(t, out, "err1")
}

// --- writeImagesCheckResult: list flag ---

func TestWriteImagesCheckResult_ListShowsExpectedAndExisting(t *testing.T) {
	stdout := captureStdout(t)

	err := writeImagesCheckResult("images check", &workspaceuc.ImagesCheckResult{
		ExpectedDirs: []string{"x/y/z"},
		ExistingDirs: []string{"x", "x/y", "x/y/z"},
	}, workspaceuc.ImagesCheckInput{List: true}, "text", nil)
	require.NoError(t, err)

	out := stdout()
	assert.Contains(t, out, "expected: x/y/z")
	assert.Contains(t, out, "existing: x")
}

func TestWriteImagesCheckResult_ListShowNothingWhenEmpty(t *testing.T) {
	stdout := captureStdout(t)

	err := writeImagesCheckResult("images check", &workspaceuc.ImagesCheckResult{}, workspaceuc.ImagesCheckInput{List: true}, "text", nil)
	require.NoError(t, err)

	out := stdout()
	assert.NotContains(t, out, "expected:")
	assert.NotContains(t, out, "existing:")
}

// --- writeImagesCheckResult: invalid format ---

func TestWriteImagesCheckResult_InvalidFormat(t *testing.T) {
	err := writeImagesCheckResult("images check", &workspaceuc.ImagesCheckResult{}, workspaceuc.ImagesCheckInput{}, "yaml", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported output format")
}

// --- writeImagesCheckResult: Summary text output ---

func TestWriteImagesCheckResult_SummaryLine(t *testing.T) {
	stdout := captureStdout(t)

	err := writeImagesCheckResult("images check", &workspaceuc.ImagesCheckResult{
		ExpectedDirs:   []string{"a", "b", "c"},
		ExistingDirs:   []string{"a", "b", "c", "extra"},
		MissingDirs:    []string{},
		ExtraDirs:      []string{"extra"},
		DuplicateFiles: nil,
		Warnings:       nil,
		Errors:         nil,
		ApplyActions:   nil,
	}, workspaceuc.ImagesCheckInput{}, "text", nil)
	require.NoError(t, err)

	out := stdout()
	assert.Contains(t, out, "expected=3")
	assert.Contains(t, out, "existing=4")
	assert.Contains(t, out, "missing=0")
	assert.Contains(t, out, "extra=1")
	assert.Contains(t, out, "duplicates=0")
}

// --- runImagesCheck: error propagation ---

func TestRunImagesCheck_NonExistentDataDir(t *testing.T) {
	err := runImagesCheck(workspaceuc.ImagesCheckInput{
		DataDir:   "/tmp/nonexistent-data-dir-" + t.Name(),
		ImagesDir: t.TempDir(),
	}, "text")
	require.Error(t, err)
}

// --- writeImagesCheckResult: Fix result text ---

func TestWriteImagesCheckResult_FixWithActions(t *testing.T) {
	stdout := captureStdout(t)

	err := writeImagesCheckResult("images fix", &workspaceuc.ImagesCheckResult{
		ApplyActions: []string{"No fixes needed"},
	}, workspaceuc.ImagesCheckInput{}, "text", []string{"No fixes needed"})
	require.NoError(t, err)

	out := stdout()
	assert.Contains(t, out, "images fix passed")
	assert.Contains(t, out, "[actions]")
	assert.Contains(t, out, "No fixes needed")
}

func TestWriteImagesCheckResult_FixWithErrorsAndActions(t *testing.T) {
	err := writeImagesCheckResult("images fix", &workspaceuc.ImagesCheckResult{
		Errors:       []string{"something wrong"},
		ApplyActions: []string{"did something"},
	}, workspaceuc.ImagesCheckInput{}, "text", []string{"did something"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "images fix failed")
}

// --- edge cases ---

func TestWriteImagesCheckResult_EmptyFormat(t *testing.T) {
	stdout := captureStdout(t)

	err := writeImagesCheckResult("images check", &workspaceuc.ImagesCheckResult{}, workspaceuc.ImagesCheckInput{}, "", nil)
	require.NoError(t, err, "empty format defaults to text")

	out := stdout()
	assert.Contains(t, out, "images check passed")
}

func TestWriteImagesCheckResult_JSONWithActions(t *testing.T) {
	stdout := captureStdout(t)

	err := writeImagesCheckResult("images fix", &workspaceuc.ImagesCheckResult{
		ApplyActions: []string{"moved 2 files"},
	}, workspaceuc.ImagesCheckInput{}, "json", []string{"moved 2 files"})
	require.NoError(t, err)

	var result map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout()), &result))
	assert.Equal(t, "images fix", result["name"])
	assert.Equal(t, true, result["ok"])
}

func TestWriteImagesCheckResult_BothExtraAndMissingDefaultFlags(t *testing.T) {
	stdout := captureStdout(t)

	err := writeImagesCheckResult("images check", &workspaceuc.ImagesCheckResult{
		MissingDirs: []string{"missing/dir"},
		ExtraDirs:   []string{"extra/dir"},
	}, workspaceuc.ImagesCheckInput{}, "text", nil)
	require.Error(t, err)

	out := stdout()
	assert.Contains(t, out, "ERROR missing/dir")
	assert.Contains(t, out, "WARN extra/dir")
}

func TestWriteImagesCheckResult_BothExtraAndMissingSkipExtra(t *testing.T) {
	stdout := captureStdout(t)

	err := writeImagesCheckResult("images check", &workspaceuc.ImagesCheckResult{
		MissingDirs: []string{"missing/dir"},
		ExtraDirs:   []string{"extra/dir"},
	}, workspaceuc.ImagesCheckInput{SkipExtra: true}, "text", nil)
	require.Error(t, err, "missing dirs still error even with SkipExtra")

	out := stdout()
	assert.Contains(t, out, "ERROR missing/dir")
	assert.NotContains(t, out, "extra/dir")
}

func TestWriteImagesCheckResult_BothExtraAndMissingSkipBoth(t *testing.T) {
	stdout := captureStdout(t)

	err := writeImagesCheckResult("images check", &workspaceuc.ImagesCheckResult{
		MissingDirs: []string{"missing/dir"},
		ExtraDirs:   []string{"extra/dir"},
		Warnings:    []string{"some warn"},
		Errors:      []string{"some err"},
	}, workspaceuc.ImagesCheckInput{SkipMissing: true, SkipExtra: true}, "text", nil)
	require.Error(t, err, "Errors field still causes failure regardless of SkipExtra/SkipMissing")

	out := stdout()
	assert.NotContains(t, out, "missing/dir")
	assert.NotContains(t, out, "extra/dir")
	assert.Contains(t, out, "some err")
	assert.Contains(t, out, "some warn")
}

func TestWriteImagesCheckResult_JSONWithSkipMissing(t *testing.T) {
	stdout := captureStdout(t)

	err := writeImagesCheckResult("images check", &workspaceuc.ImagesCheckResult{
		MissingDirs: []string{"missing/dir"},
	}, workspaceuc.ImagesCheckInput{SkipMissing: true}, "json", nil)
	require.NoError(t, err)

	var result map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout()), &result))
	assert.Equal(t, true, result["ok"])
}

func TestWriteImagesCheckResult_ListWithIssues(t *testing.T) {
	stdout := captureStdout(t)

	err := writeImagesCheckResult("images check", &workspaceuc.ImagesCheckResult{
		ExpectedDirs: []string{"a/b/c"},
		ExistingDirs: []string{"a", "a/b", "a/b/c"},
		MissingDirs:  []string{"x/y"},
	}, workspaceuc.ImagesCheckInput{List: true}, "text", nil)
	require.Error(t, err, "missing dirs still error")

	out := stdout()
	assert.Contains(t, out, "expected: a/b/c")
	assert.Contains(t, out, "existing: a")
	assert.Contains(t, out, "ERROR x/y")
}

func TestWriteImagesCheckResult_EmptyName(t *testing.T) {
	stdout := captureStdout(t)

	err := writeImagesCheckResult("", &workspaceuc.ImagesCheckResult{}, workspaceuc.ImagesCheckInput{}, "text", nil)
	require.NoError(t, err)

	out := stdout()
	assert.Contains(t, out, "passed")
}

// --- checkutil helpers ---

func TestHasIssueErrors_TrueForErrors(t *testing.T) {
	assert.True(t, workspaceuc.HasIssueErrors([]checkutil.Issue{
		{Severity: checkutil.SeverityError, Message: "bad"},
	}))
}

func TestHasIssueErrors_FalseForWarnings(t *testing.T) {
	assert.False(t, workspaceuc.HasIssueErrors([]checkutil.Issue{
		{Severity: checkutil.SeverityWarn, Message: "warn"},
	}))
}

func TestHasIssueErrors_EmptyList(t *testing.T) {
	assert.False(t, workspaceuc.HasIssueErrors(nil))
}
