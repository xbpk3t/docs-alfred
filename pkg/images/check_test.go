package images

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDuplicateFileRe(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantMatch bool
		original string
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

func TestCollectExpectedDirsOnly(t *testing.T) {
	// Test with non-existent data dir
	result, err := CollectExpectedDirsOnly(CheckConfig{
		DataDir: "/tmp/nonexistent-data-dir-12345",
	})
	require.Error(t, err, "expected error for non-existent dir")
	assert.Nil(t, result)
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
	// Report should not panic with apply actions
	r.Report(CheckConfig{Apply: true})
}

func TestCheckResult_ReportNoApply(t *testing.T) {
	r := &CheckResult{
		MissingDirs: []string{"some/dir"},
	}
	// Report should not panic
	r.Report(CheckConfig{})
}
