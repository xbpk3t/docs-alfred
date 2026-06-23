package images

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	ghdata "github.com/xbpk3t/docs-alfred/internal/gh/data"
)

func TestDuplicateFileRe(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantMatch bool
		original  string
	}{
		{"name__1.jpg matches", "photo__1.jpg", true, "photo.jpg"},
		{"name__999.ext", "file__999.txt", true, "file.txt"},
		{"no number", "photo.jpg", false, ""},
		{"single underscore", "photo_1.jpg", false, ""},
		{"triple underscore", "photo___1.jpg", false, ""},
		{"multiple dots", "archive.tar__1.gz", true, "archive.tar.gz"},
		{"no extension", "file__1", false, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matches := duplicateFileRe.FindStringSubmatch(tt.input)
			if tt.wantMatch {
				require.NotNil(t, matches, "expected match for %q", tt.input)
				original := matches[1] + matches[2]
				assert.Equal(t, tt.original, original)
			} else {
				assert.Nil(t, matches, "expected no match for %q", tt.input)
			}
		})
	}
}

func TestRemoveDuplicateFiles(t *testing.T) {
	dir := t.TempDir()

	// Create original file
	original := filepath.Join(dir, "photo.jpg")
	require.NoError(t, os.WriteFile(original, []byte("original"), 0644))

	// Create duplicate file
	dup := filepath.Join(dir, "photo__1.jpg")
	require.NoError(t, os.WriteFile(dup, []byte("duplicate"), 0644))

	// Create unrelated file (should not be removed)
	normal := filepath.Join(dir, "readme.txt")
	require.NoError(t, os.WriteFile(normal, []byte("readme"), 0644))

	// Create duplicate without original (should not be removed)
	orphan := filepath.Join(dir, "orphan__2.png")
	require.NoError(t, os.WriteFile(orphan, []byte("orphan"), 0644))

	files := []string{"photo.jpg", "photo__1.jpg", "readme.txt", "orphan__2.png"}
	removed := removeDuplicateFiles(dir, files)
	assert.Equal(t, 1, removed, "should remove one duplicate file")

	// Verify photo.jpg still exists
	_, err := os.Stat(original)
	assert.NoError(t, err, "original should still exist")

	// Verify photo__1.jpg was removed
	_, err = os.Stat(dup)
	assert.True(t, os.IsNotExist(err), "duplicate should be removed")

	// Verify unrelated and orphan files still exist
	_, err = os.Stat(normal)
	assert.NoError(t, err, "unrelated file should exist")
	_, err = os.Stat(orphan)
	assert.NoError(t, err, "orphan file should exist")
}

func TestFindDuplicateFiles(t *testing.T) {
	actualFiles := []string{
		"photo.jpg",
		"photo__1.jpg",
		"nested/archive.tar.gz",
		"nested/archive.tar__2.gz",
		"orphan__3.png",
		"plain.txt",
	}

	assert.Equal(t, []string{"photo__1.jpg", "nested/archive.tar__2.gz"}, findDuplicateFiles(actualFiles))
}

func TestCollectExistingFilesAndDirsSkipsHiddenEntries(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "visible"), 0755))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, ".hidden"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "visible", "keep.txt"), []byte("keep"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".hidden", "skip.txt"), []byte("skip"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".root-hidden.txt"), []byte("skip"), 0644))

	dirs, files, err := collectExistingFilesAndDirs(dir)
	require.NoError(t, err)

	assert.Contains(t, dirs, "visible")
	assert.NotContains(t, dirs, ".hidden")
	assert.Contains(t, files, "visible/keep.txt")
	assert.NotContains(t, files, ".hidden/skip.txt")
	assert.NotContains(t, files, ".root-hidden.txt")
}

func TestCollectExpectedImageDirsFromTypedGhData(t *testing.T) {
	dataDir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dataDir, "algo"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(dataDir, "algo", "go.yml"), []byte(`- type: go
  topics:
    - topic: root
      hasPic: true
      sub:
        - topic: child
          meta:
            slug: child-slug
            hasPic: true
    - topic: no-pic
  using:
    url: https://github.com/acme/tool
    topics:
      - topic: using-topic
        hasPic: true
  repo:
    - url: https://github.com/acme/repo
      topics:
        - topic: repo-topic
          meta:
            slug: repo-slug
            hasPic: true
  record: []
`), 0644))

	dirs, err := collectExpectedImageDirs(dataDir)

	require.NoError(t, err)
	assert.ElementsMatch(t, []string{
		"algo/go/root",
		"algo/go/root/child-slug",
		"algo/go/using-topic",
		"algo/go/repo/repo-slug",
	}, dirs)
}

func TestHideExtraDirs(t *testing.T) {
	dir := t.TempDir()

	// Create an "extra" directory
	extraDir := filepath.Join(dir, "extra-folder")
	require.NoError(t, os.MkdirAll(extraDir, 0755))

	// Create a subfile to verify it's a real dir
	require.NoError(t, os.WriteFile(filepath.Join(extraDir, "test.txt"), []byte("test"), 0644))

	extraDirs := []string{"extra-folder"}
	hidden := hideExtraDirs(dir, extraDirs)
	assert.Equal(t, 1, hidden, "should hide one extra directory")

	// Verify original name no longer exists
	_, err := os.Stat(extraDir)
	assert.True(t, os.IsNotExist(err), "original dir should not exist")

	// Verify hidden name exists
	hiddenDir := filepath.Join(dir, ".extra-folder")
	_, err = os.Stat(hiddenDir)
	assert.NoError(t, err, "hidden dir should exist")
}

func TestHideExtraDirs_Conflict(t *testing.T) {
	dir := t.TempDir()

	// Create extra dir and a pre-existing hidden dir with the same name
	extraDir := filepath.Join(dir, "my-dir")
	require.NoError(t, os.MkdirAll(extraDir, 0755))

	existingHidden := filepath.Join(dir, ".my-dir")
	require.NoError(t, os.MkdirAll(existingHidden, 0755))

	extraDirs := []string{"my-dir"}
	hidden := hideExtraDirs(dir, extraDirs)
	assert.Equal(t, 1, hidden, "should hide dir with .1 suffix on conflict")

	// Verify the extra dir was renamed to .my-dir.1
	renamed := filepath.Join(dir, ".my-dir.1")
	_, err := os.Stat(renamed)
	assert.NoError(t, err, "renamed dir should exist")
}

func TestMoveExtraFiles(t *testing.T) {
	dir := t.TempDir()

	// Create a "loose" file at root level
	looseFile := filepath.Join(dir, "loose.txt")
	require.NoError(t, os.WriteFile(looseFile, []byte("loose"), 0644))

	// Create an expected directory with a file (simulated)
	expectedDir := filepath.Join(dir, "expected")
	require.NoError(t, os.MkdirAll(expectedDir, 0755))
	expectedFile := filepath.Join(expectedDir, "keep.txt")
	require.NoError(t, os.WriteFile(expectedFile, []byte("keep"), 0644))

	actualFiles := []string{"loose.txt", "expected/keep.txt"}

	moved := moveExtraFiles(dir, nil, actualFiles)
	assert.Equal(t, 1, moved, "should move one loose file")

	// Verify loose file was moved to .temp
	tempDir := filepath.Join(filepath.Dir(dir), ".temp")
	_, err := os.Stat(filepath.Join(tempDir, "loose.txt"))
	assert.NoError(t, err, "moved file should be in .temp")

	// Verify expected file still in place
	_, err = os.Stat(expectedFile)
	assert.NoError(t, err, "expected file should remain")
}

func TestRunImagesCheck_NonExistentDir(t *testing.T) {
	result, err := RunImagesCheck(CheckConfig{
		DataDir:   "/tmp/nonexistent-data-dir-12345",
		ImagesDir: t.TempDir(),
	})
	require.Error(t, err)
	assert.Nil(t, result)
}

func TestCheckResult_ReportApply(t *testing.T) {
	r := &CheckResult{
		ApplyActions: []string{"Removed 2 duplicate file(s)", "Hidden 1 extra director(ies)"},
	}
	report := r.ReportResult(CheckConfig{Apply: true})
	assert.Contains(t, report, "[apply]")
	assert.Contains(t, report, "Removed 2 duplicate file(s)")
}

func TestCheckResult_ReportNoApply(t *testing.T) {
	r := &CheckResult{
		MissingDirs: []string{"some/dir"},
	}
	report := r.ReportResult(CheckConfig{})
	assert.True(t, strings.Contains(report, "ERROR some/dir") || strings.Contains(report, "some/dir"))
}

func TestRunImagesCheck_WithApply(t *testing.T) {
	dir := t.TempDir()

	// Create a duplicate file pair
	require.NoError(t, os.WriteFile(filepath.Join(dir, "photo.jpg"), []byte("original"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "photo__1.jpg"), []byte("duplicate"), 0644))

	result, err := RunImagesCheck(CheckConfig{
		DataDir:   dir,
		ImagesDir: dir,
		Apply:     true,
	})
	require.NoError(t, err)
	assert.NotNil(t, result)
}

func TestRunImagesCheck_WithApplyExtraDir(t *testing.T) {
	dir := t.TempDir()
	extraDir := filepath.Join(dir, "extra")
	require.NoError(t, os.MkdirAll(extraDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(extraDir, "file.txt"), []byte("data"), 0644))

	result, err := RunImagesCheck(CheckConfig{
		DataDir:   dir,
		ImagesDir: dir,
		Apply:     true,
	})
	require.NoError(t, err)
	assert.NotNil(t, result)
}

func TestCheckResult_IssuesNoFlags(t *testing.T) {
	r := &CheckResult{
		Warnings:       []string{"warn1"},
		Errors:         []string{"err1"},
		DuplicateFiles: []string{"dup.jpg"},
		MissingDirs:    []string{"missing"},
		ExtraDirs:      []string{"extra"},
	}
	issues := r.Issues(CheckConfig{})
	assert.NotEmpty(t, issues)
	// Should have warn + error + duplicate + missing + extra
	assert.True(t, len(issues) >= 5)
}

func TestCheckResult_ReportWithList(t *testing.T) {
	r := &CheckResult{
		ExpectedDirs: []string{"a", "b"},
		ExistingDirs: []string{"c"},
	}
	report := r.ReportResult(CheckConfig{List: true})
	assert.Contains(t, report, "expected: a")
	assert.Contains(t, report, "existing: c")
}

func TestApplyFixes_NoFixesNeeded(t *testing.T) {
	result := &CheckResult{
		ActualFiles: []string{},
	}
	applyFixes(result, CheckConfig{Apply: true})
	assert.Contains(t, result.ApplyActions, "No fixes needed")
}

func TestMoveExtraFilesSkipsHiddenFiles(t *testing.T) {
	dir := t.TempDir()
	// Create a hidden file at root level
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".hidden-file"), []byte("hidden"), 0o644))
	// Create a visible file at root level
	require.NoError(t, os.WriteFile(filepath.Join(dir, "visible.txt"), []byte("visible"), 0o644))

	actualFiles := []string{".hidden-file", "visible.txt"}
	moved := moveExtraFiles(dir, nil, actualFiles)
	assert.Equal(t, 1, moved, "should move only visible file")

	// Verify hidden file was NOT moved
	_, err := os.Stat(filepath.Join(dir, ".hidden-file"))
	assert.NoError(t, err, "hidden file should remain")
}

func TestCollectExistingFilesAndDirsNonExistentDir(t *testing.T) {
	_, _, err := collectExistingFilesAndDirs("/tmp/nonexistent-images-dir-12345")
	require.Error(t, err)
}

func TestCollectExpectedImageDirsNoType(t *testing.T) {
	dataDir := t.TempDir()
	// YAML without type field should be skipped
	require.NoError(t, os.MkdirAll(filepath.Join(dataDir, "cat"), 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(dataDir, "cat", "notype.yml"), []byte(`- name: test`), 0o600))

	dirs, err := collectExpectedImageDirs(dataDir)
	require.NoError(t, err)
	assert.Empty(t, dirs)
}

func TestCollectExpectedImageDirsShortPath(t *testing.T) {
	dataDir := t.TempDir()
	// YAML at root level (dirParts < 2) should be skipped
	require.NoError(t, os.WriteFile(filepath.Join(dataDir, "root.yml"), []byte(`- type: test`), 0o600))

	dirs, err := collectExpectedImageDirs(dataDir)
	require.NoError(t, err)
	assert.Empty(t, dirs)
}

func TestCollectRepoTopicDirsEmptyURL(t *testing.T) {
	var dirs []string
	// Empty URL should result in empty repoName, causing early return
	collectRepoTopicDirs(&ghdata.Repo{URL: ""}, "base", &dirs, false)
	assert.Empty(t, dirs)
}

func TestCollectTopicDirsEmptyDirName(t *testing.T) {
	var dirs []string
	// Topic with empty DirName should be skipped
	collectTopicDirs([]ghdata.Topic{{}}, "base", &dirs)
	assert.Empty(t, dirs)
}

func TestRunImagesCheckNoDataYAML(t *testing.T) {
	dir := t.TempDir()
	result, err := RunImagesCheck(CheckConfig{
		DataDir:   dir,
		ImagesDir: dir,
	})
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Empty(t, result.ExpectedDirs)
	assert.Empty(t, result.MissingDirs)
}

func TestCheckResult_ReportNoApplyNoActions(t *testing.T) {
	r := &CheckResult{}
	report := r.ReportResult(CheckConfig{Apply: true})
	// Should still produce valid output even with no actions
	assert.NotEmpty(t, report)
}
