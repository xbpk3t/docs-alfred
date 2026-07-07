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
		original  string
		wantMatch bool
	}{
		{"name__1.jpg matches", "photo__1.jpg", "photo.jpg", true},
		{"name__999.ext", "file__999.txt", "file.txt", true},
		{"no number", "photo.jpg", "", false},
		{"single underscore", "photo_1.jpg", "", false},
		{"triple underscore", "photo___1.jpg", "", false},
		{"multiple dots", "archive.tar__1.gz", "archive.tar.gz", true},
		{"no extension", "file__1", "", false},
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
      sub:
        - topic: child
          meta:
            slug: child-slug
    - topic: no-pic
  using:
    url: https://github.com/acme/tool
    topics:
      - topic: using-topic
  repo:
    - url: https://github.com/acme/repo
      topics:
        - topic: repo-topic
          meta:
            slug: repo-slug
  record: []
`), 0644))

	dirs, err := collectExpectedImageDirs(dataDir)

	require.NoError(t, err)
	assert.ElementsMatch(t, []string{
		"algo/go/root",
		"algo/go/root/child-slug",
		"algo/go/no-pic",
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
	assert.GreaterOrEqual(t, len(issues), 5)
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

// --- isParentOfExpected ---

func TestIsParentOfExpected_ReturnsTrueForDirectParent(t *testing.T) {
	assert.True(t, isParentOfExpected("a", []string{"a/b"}))
	assert.True(t, isParentOfExpected("a/b", []string{"a/b/c"}))
	assert.True(t, isParentOfExpected("algo", []string{"algo/algo/scheduler"}))
	assert.True(t, isParentOfExpected("algo/algo", []string{"algo/algo/scheduler"}))
}

func TestIsParentOfExpected_ReturnsTrueForDeepNesting(t *testing.T) {
	assert.True(t, isParentOfExpected("x", []string{"x/y/z/w"}))
	assert.True(t, isParentOfExpected("x/y", []string{"x/y/z/w"}))
	assert.True(t, isParentOfExpected("x/y/z", []string{"x/y/z/w"}))
}

func TestIsParentOfExpected_ReturnsFalseForExactMatch(t *testing.T) {
	assert.False(t, isParentOfExpected("a/b", []string{"a/b"}))
}

func TestIsParentOfExpected_ReturnsFalseForUnrelatedDir(t *testing.T) {
	assert.False(t, isParentOfExpected("other", []string{"a/b"}))
	assert.False(t, isParentOfExpected("a/other", []string{"a/b"}))
}

func TestIsParentOfExpected_ReturnsFalseForPrefixLikeNoSlash(t *testing.T) {
	assert.False(t, isParentOfExpected("a-new", []string{"a/b"}))
}

func TestIsParentOfExpected_EmptyExpectedList(t *testing.T) {
	assert.False(t, isParentOfExpected("a", []string{}))
}

func TestIsParentOfExpected_EmptyDir(t *testing.T) {
	assert.False(t, isParentOfExpected("", []string{"a/b"}))
}

// --- dirHasDirectFiles ---

func TestDirHasDirectFiles_ReturnsTrueForFileInDir(t *testing.T) {
	files := []string{"dir/file.txt", "dir/sub/file2.txt"}
	assert.True(t, dirHasDirectFiles("dir", files))
}

func TestDirHasDirectFiles_ReturnsFalseForFileOnlyInSubdir(t *testing.T) {
	files := []string{"dir/sub/file.txt"}
	assert.False(t, dirHasDirectFiles("dir", files))
}

func TestDirHasDirectFiles_ReturnsFalseForNoFiles(t *testing.T) {
	files := []string{"other/file.txt"}
	assert.False(t, dirHasDirectFiles("dir", files))
}

func TestDirHasDirectFiles_EmptyFileList(t *testing.T) {
	assert.False(t, dirHasDirectFiles("dir", []string{}))
}

func TestDirHasDirectFiles_MultipleNestedNoDirect(t *testing.T) {
	files := []string{"a/b/c/d.txt", "a/b/c/e.txt"}
	assert.False(t, dirHasDirectFiles("a/b", files))
	assert.True(t, dirHasDirectFiles("a/b/c", files))
}

func TestDirHasDirectFiles_FileWithSimilarPrefix(t *testing.T) {
	files := []string{"dir-other/file.txt"}
	assert.False(t, dirHasDirectFiles("dir", files))
}

// --- RunImagesCheck integration: structural dir filtering ---

func TestRunImagesCheck_SkipsStructuralDirsWithoutDirectFiles(t *testing.T) {
	dir := t.TempDir()

	require.NoError(t, os.MkdirAll(filepath.Join(dir, "algo", "algo", "scheduler"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "algo", "algo", "scheduler", "file.png"), nil, 0644))

	dataDir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dataDir, "algo"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(dataDir, "algo", "go.yml"), []byte(`- type: algo
  topics:
    - topic: scheduler
`), 0644))

	result, err := RunImagesCheck(CheckConfig{DataDir: dataDir, ImagesDir: dir})
	require.NoError(t, err)

	assert.NotContains(t, result.ExtraDirs, "algo")
	assert.NotContains(t, result.ExtraDirs, "algo/algo")
	assert.Contains(t, result.ExistingDirs, "algo/algo/scheduler")
	assert.Contains(t, result.ExpectedDirs, "algo/algo/scheduler")
}

func TestRunImagesCheck_FlagsStructuralDirsWithDirectFiles(t *testing.T) {
	dir := t.TempDir()

	require.NoError(t, os.MkdirAll(filepath.Join(dir, "algo", "algo", "scheduler"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "algo", "algo", "scheduler", "file.png"), nil, 0644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "algo", "algo", "orphan.png"), nil, 0644))

	dataDir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dataDir, "algo"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(dataDir, "algo", "go.yml"), []byte(`- type: algo
  topics:
    - topic: scheduler
`), 0644))

	result, err := RunImagesCheck(CheckConfig{DataDir: dataDir, ImagesDir: dir})
	require.NoError(t, err)

	assert.NotContains(t, result.ExtraDirs, "algo")
	assert.Contains(t, result.ExtraDirs, "algo/algo")
}

func TestRunImagesCheck_FlagsGenuinelyExtraDirs(t *testing.T) {
	dir := t.TempDir()

	require.NoError(t, os.MkdirAll(filepath.Join(dir, "totally-extra", "sub"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "totally-extra", "sub", "file.png"), nil, 0644))

	dataDir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dataDir, "algo"), 0755))

	result, err := RunImagesCheck(CheckConfig{DataDir: dataDir, ImagesDir: dir})
	require.NoError(t, err)

	assert.Contains(t, result.ExtraDirs, "totally-extra")
}

func TestRunImagesCheck_SkipsExpectedDirWithMissingParent(t *testing.T) {
	dir := t.TempDir()

	require.NoError(t, os.MkdirAll(filepath.Join(dir, "db", "redis", "cache-problems"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "db", "redis", "cache-problems", "file.png"), nil, 0644))

	dataDir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dataDir, "db"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(dataDir, "db", "storage.yml"), []byte(`- type: redis
  topics:
    - topic: cache-problems
`), 0644))

	result, err := RunImagesCheck(CheckConfig{DataDir: dataDir, ImagesDir: dir})
	require.NoError(t, err)

	assert.NotContains(t, result.ExtraDirs, "db")
	assert.NotContains(t, result.ExtraDirs, "db/redis")
	assert.Contains(t, result.ExpectedDirs, "db/redis/cache-problems")
	assert.NotContains(t, result.MissingDirs, "db/redis/cache-problems")
}

func TestRunImagesCheck_FlagsL2FilesInStructuralDir(t *testing.T) {
	dir := t.TempDir()

	require.NoError(t, os.MkdirAll(filepath.Join(dir, "ai", "skills"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "ai", "skills", "agent-skills.webp"), nil, 0644))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "ai", "skills", "sub-topic"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "ai", "skills", "sub-topic", "file.png"), nil, 0644))

	dataDir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dataDir, "ai"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(dataDir, "ai", "skills.yml"), []byte(`- type: skills
  topics:
    - topic: sub-topic
`), 0644))

	result, err := RunImagesCheck(CheckConfig{DataDir: dataDir, ImagesDir: dir})
	require.NoError(t, err)

	assert.Contains(t, result.ExtraDirs, "ai/skills",
		"structural dir with direct files should be flagged (files at wrong layer)")
}

func TestRunImagesCheck_NestedIntermediateDirsAllSkipped(t *testing.T) {
	dir := t.TempDir()

	require.NoError(t, os.MkdirAll(filepath.Join(dir, "x", "y", "z", "w"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "x", "y", "z", "w", "file.png"), nil, 0644))

	dataDir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dataDir, "x"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(dataDir, "x", "deep.yml"), []byte(`- type: y
  topics:
    - topic: z
      sub:
        - topic: w
          meta:
            slug: w
`), 0644))

	result, err := RunImagesCheck(CheckConfig{DataDir: dataDir, ImagesDir: dir})
	require.NoError(t, err)

	assert.NotContains(t, result.ExtraDirs, "x")
	assert.NotContains(t, result.ExtraDirs, "x/y")
	assert.NotContains(t, result.ExtraDirs, "x/y/z")
	assert.Contains(t, result.ExpectedDirs, "x/y/z/w")
	assert.NotContains(t, result.MissingDirs, "x/y/z/w")
}

func TestRunImagesCheck_ExpectedDirItselfNotExtra(t *testing.T) {
	dir := t.TempDir()

	// Structural dir with ONLY subdir content — no direct files
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "desktop", "browser", "desktop-browser"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "desktop", "browser", "desktop-browser", "file.png"), nil, 0644))

	dataDir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dataDir, "desktop"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(dataDir, "desktop", "browser.yml"), []byte(`- type: browser
  topics:
    - topic: desktop-browser
      meta:
        slug: desktop-browser
`), 0644))

	result, err := RunImagesCheck(CheckConfig{DataDir: dataDir, ImagesDir: dir})
	require.NoError(t, err)

	// Pure structural dirs without direct files should be skipped
	assert.NotContains(t, result.ExtraDirs, "desktop")
	assert.NotContains(t, result.ExtraDirs, "desktop/browser")
}

func TestRunImagesCheck_SingleDepthExpectedIsNotParent(t *testing.T) {
	dir := t.TempDir()

	// Create expected directory: x/single/repo/a-topic
	// The repo creates the 'repo' subdirectory automatically
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "x", "single", "repo", "a-topic"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "x", "single", "repo", "a-topic", "file.png"), nil, 0644))

	dataDir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dataDir, "x"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(dataDir, "x", "single.yml"), []byte(`- type: single
  repo:
    - url: https://github.com/acme/repo
      topics:
        - topic: a-topic
          meta:
            slug: a-topic
`), 0644))

	result, err := RunImagesCheck(CheckConfig{DataDir: dataDir, ImagesDir: dir})
	require.NoError(t, err)

	// Pure structural dirs without direct files should be skipped
	assert.NotContains(t, result.ExtraDirs, "x")
	assert.NotContains(t, result.ExtraDirs, "x/single")
	// Subdirs with no direct files should also be skipped
	assert.NotContains(t, result.ExtraDirs, "x/single/repo")

	// Expected leaf should exist
	assert.Contains(t, result.ExpectedDirs, "x/single/repo/a-topic")
	assert.NotContains(t, result.MissingDirs, "x/single/repo/a-topic")
}
