package workspaceops

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xbpk3t/docs-alfred/pkg/checkutil"
)

// --- toSet ---

func TestToSet(t *testing.T) {
	set := toSet([]string{"a", "b", "a"})
	assert.True(t, set["a"])
	assert.True(t, set["b"])
}

func TestToSetEmpty(t *testing.T) {
	set := toSet(nil)
	assert.Empty(t, set)
}

// --- collectExpectedDirs ---

func TestCollectExpectedDirsEmpty(t *testing.T) {
	dirs, err := collectExpectedDirs(t.TempDir())
	require.NoError(t, err)
	assert.Empty(t, dirs)
}

func TestCollectExpectedDirsWithYAML(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "algo", "go"), 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(root, "algo", "go.yml"), []byte("- type: go"), 0o600))
	require.NoError(t, os.MkdirAll(filepath.Join(root, "dev", "cli"), 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(root, "dev", "cli.yml"), []byte("- type: cli"), 0o600))
	require.NoError(t, os.MkdirAll(filepath.Join(root, ".hidden"), 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(root, ".hidden", "x.yml"), []byte("- type: x"), 0o600))

	dirs, err := collectExpectedDirs(root)
	require.NoError(t, err)
	assert.Contains(t, dirs, "algo/go")
	assert.Contains(t, dirs, "dev/cli")
	assert.NotContains(t, dirs, ".hidden/x")
}

// --- collectActualWikiDirs ---

func TestCollectActualWikiDirsEmpty(t *testing.T) {
	dirs, err := collectActualWikiDirs(t.TempDir())
	require.NoError(t, err)
	assert.Empty(t, dirs)
}

func TestCollectActualWikiDirs(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "tech", "research"), 0o700))
	require.NoError(t, os.MkdirAll(filepath.Join(root, ".hidden"), 0o700))

	dirs, err := collectActualWikiDirs(root)
	require.NoError(t, err)
	assert.Contains(t, dirs, "tech")
	assert.Contains(t, dirs, "tech/research")
	assert.NotContains(t, dirs, ".hidden")
}

func TestCollectActualWikiDirsDepth2(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "a", "b", "c"), 0o700))

	dirs, err := collectActualWikiDirs(root)
	require.NoError(t, err)
	assert.Contains(t, dirs, "a")
	assert.Contains(t, dirs, "a/b")
	// depth-3 "a/b/c" should not be collected at wiki level
}

// --- collectActualTopicDirs ---

func TestCollectActualTopicDirsEmpty(t *testing.T) {
	dirs, err := collectActualTopicDirs(t.TempDir())
	require.NoError(t, err)
	assert.Empty(t, dirs)
}

func TestCollectActualTopicDirs(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "folder", "type", "topic"), 0o700))
	require.NoError(t, os.MkdirAll(filepath.Join(root, "folder", "type", ".hidden"), 0o700))

	dirs, err := collectActualTopicDirs(root)
	require.NoError(t, err)
	assert.Contains(t, dirs, "folder/type/topic")
	assert.NotContains(t, dirs, "folder/type/.hidden")
}

// --- collectExpectedTopics ---

func TestCollectExpectedTopicsEmpty(t *testing.T) {
	set, err := collectExpectedTopics(t.TempDir())
	require.NoError(t, err)
	assert.Empty(t, set)
}

// --- computeDirDiff ---

func TestComputeDirDiff(t *testing.T) {
	expected := []string{"a", "b", "c"}
	actual := []string{"b", "c", "d"}
	expectedSet := toSet(expected)
	actualSet := toSet(actual)

	missing, extra := computeDirDiff(expected, actual, expectedSet, actualSet)
	assert.Equal(t, []string{"a"}, missing)
	assert.Equal(t, []string{"d"}, extra)
}

func TestComputeDirDiffNoDiffs(t *testing.T) {
	dirs := []string{"a", "b"}
	set := toSet(dirs)
	missing, extra := computeDirDiff(dirs, dirs, set, set)
	assert.Empty(t, missing)
	assert.Empty(t, extra)
}

// --- filterContainerDirs ---

func TestFilterContainerDirs(t *testing.T) {
	extra := []string{"algo", "unknown"}
	expected := []string{"algo/go", "algo/cli"}
	filtered := filterContainerDirs(extra, expected)
	assert.Equal(t, []string{"unknown"}, filtered)
}

func TestFilterContainerDirsNested(t *testing.T) {
	extra := []string{"a/b", "c"}
	expected := []string{"a/b/c"}
	filtered := filterContainerDirs(extra, expected)
	assert.Equal(t, []string{"a/b", "c"}, filtered) // a/b is not depth-1
}

func TestFilterContainerDirsEmpty(t *testing.T) {
	assert.Empty(t, filterContainerDirs(nil, nil))
}

// --- buildWikiIssues ---

func TestBuildWikiIssues(t *testing.T) {
	issues := buildWikiIssues([]string{"missing1"}, []string{"extra1"})
	require.Len(t, issues, 2)
	assert.Equal(t, checkutil.SeverityError, issues[0].Severity)
	assert.Contains(t, issues[0].Message, "missing wiki dir")
	assert.Equal(t, checkutil.SeverityError, issues[1].Severity)
	assert.Contains(t, issues[1].Message, "extra wiki dir")
}

func TestBuildWikiIssuesEmpty(t *testing.T) {
	assert.Empty(t, buildWikiIssues(nil, nil))
}

// --- RunWikiCheck ---

func TestRunWikiCheckMatchingStructure(t *testing.T) {
	ghRoot := t.TempDir()
	wikiRoot := t.TempDir()

	// Create matching structure
	require.NoError(t, os.MkdirAll(filepath.Join(ghRoot, "algo"), 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(ghRoot, "algo", "go.yml"), []byte("- type: go\n  topics:\n    - topic: test\n"), 0o600))
	require.NoError(t, os.MkdirAll(filepath.Join(wikiRoot, "algo", "go", "test"), 0o700))

	result, err := RunWikiCheck(WikiCheckInput{GhRoot: ghRoot, WikiRoot: wikiRoot})
	require.NoError(t, err)
	assert.NotNil(t, result)
	// No topic mismatch issues
	for _, issue := range result.Issues {
		assert.NotContains(t, issue.Message, "wiki topic dir not in data/gh topics")
	}
}

func TestRunWikiCheckTopicMismatch(t *testing.T) {
	ghRoot := t.TempDir()
	wikiRoot := t.TempDir()

	// YAML defines topic "terminal"
	require.NoError(t, os.MkdirAll(filepath.Join(ghRoot, "desktop"), 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(ghRoot, "desktop", "GUI.yml"), []byte("- type: GUI\n  topics:\n    - topic: terminal\n"), 0o600))
	// Wiki has "terminal-zzz" instead of "terminal"
	require.NoError(t, os.MkdirAll(filepath.Join(wikiRoot, "desktop", "GUI", "terminal-zzz"), 0o700))

	result, err := RunWikiCheck(WikiCheckInput{GhRoot: ghRoot, WikiRoot: wikiRoot})
	require.NoError(t, err)
	require.NotEmpty(t, result.Issues)
	found := false
	for _, issue := range result.Issues {
		if strings.Contains(issue.Message, "wiki topic dir not in data/gh topics") && strings.Contains(issue.Message, "terminal-zzz") {
			found = true
			assert.Equal(t, checkutil.SeverityError, issue.Severity)
		}
	}
	assert.True(t, found, "expected topic mismatch issue for terminal-zzz, got: %+v", result.Issues)
}

func TestRunWikiCheckTopicMatch(t *testing.T) {
	ghRoot := t.TempDir()
	wikiRoot := t.TempDir()

	// YAML defines topic "terminal"
	require.NoError(t, os.MkdirAll(filepath.Join(ghRoot, "desktop"), 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(ghRoot, "desktop", "GUI.yml"), []byte("- type: GUI\n  topics:\n    - topic: terminal\n"), 0o600))
	// Wiki has matching "terminal"
	require.NoError(t, os.MkdirAll(filepath.Join(wikiRoot, "desktop", "GUI", "terminal"), 0o700))

	result, err := RunWikiCheck(WikiCheckInput{GhRoot: ghRoot, WikiRoot: wikiRoot})
	require.NoError(t, err)
	// No topic mismatch issues
	for _, issue := range result.Issues {
		assert.NotContains(t, issue.Message, "wiki topic dir not in data/gh topics")
	}
}

func TestRunWikiCheckMissingWikiDir(t *testing.T) {
	ghRoot := t.TempDir()
	wikiRoot := t.TempDir()

	require.NoError(t, os.MkdirAll(filepath.Join(ghRoot, "algo"), 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(ghRoot, "algo", "go.yml"), []byte("- type: go"), 0o600))

	result, err := RunWikiCheck(WikiCheckInput{GhRoot: ghRoot, WikiRoot: wikiRoot})
	require.NoError(t, err)
	assert.NotEmpty(t, result.MissingWikiDirs)
}

func TestRunWikiCheckExtraWikiDir(t *testing.T) {
	ghRoot := t.TempDir()
	wikiRoot := t.TempDir()

	require.NoError(t, os.MkdirAll(filepath.Join(wikiRoot, "extra"), 0o700))

	result, err := RunWikiCheck(WikiCheckInput{GhRoot: ghRoot, WikiRoot: wikiRoot})
	require.NoError(t, err)
	assert.NotEmpty(t, result.ExtraWikiDirs)
}

func TestWikiCheckResultSummary(t *testing.T) {
	r := &WikiCheckResult{
		ExpectedWikiDirs: []string{"a", "b"},
		ActualWikiDirs:   []string{"b", "c"},
		MissingWikiDirs:  []string{"a"},
		ExtraWikiDirs:    []string{"c"},
	}
	s := r.Summary()
	assert.Equal(t, 2, s["expectedWikiDirs"])
	assert.Equal(t, 2, s["actualWikiDirs"])
	assert.Equal(t, 1, s["missingWikiDirs"])
	assert.Equal(t, 1, s["extraWikiDirs"])
}

// --- FormatImagesDetails ---

func TestFormatImagesDetails(t *testing.T) {
	result := &ImagesCheckResult{
		ExpectedDirs: []string{"a"},
		ExistingDirs: []string{"b"},
		ExtraDirs:    []string{"b"},
	}
	details := FormatImagesDetails(result, ImagesCheckInput{List: true})
	assert.Contains(t, details, "expected:")
	assert.Contains(t, details, "existing:")
}

// --- RunBlogCheck ---

func TestRunBlogCheckMissingBlogDir(t *testing.T) {
	dataDir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dataDir, "cat"), 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(dataDir, "cat", "type.yml"), []byte("- type: t"), 0o600))

	result, err := RunBlogCheck(BlogCheckInput{DataDir: dataDir, BlogDir: "/tmp/nonexistent-blog"})
	require.NoError(t, err)
	assert.NotNil(t, result)
}

func TestRunBlogCheckMatching(t *testing.T) {
	dataDir := t.TempDir()
	blogDir := t.TempDir()

	require.NoError(t, os.MkdirAll(filepath.Join(dataDir, "cat"), 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(dataDir, "cat", "type.yml"), []byte("- type: t"), 0o600))
	require.NoError(t, os.MkdirAll(filepath.Join(blogDir, "cat", "type"), 0o700))

	result, err := RunBlogCheck(BlogCheckInput{DataDir: dataDir, BlogDir: blogDir})
	require.NoError(t, err)
	assert.NotNil(t, result)
}

func TestRunBlogCheckExtraBlogDir(t *testing.T) {
	dataDir := t.TempDir()
	blogDir := t.TempDir()

	require.NoError(t, os.MkdirAll(filepath.Join(blogDir, "cat", "type"), 0o700))

	result, err := RunBlogCheck(BlogCheckInput{DataDir: dataDir, BlogDir: blogDir})
	require.NoError(t, err)
	assert.NotEmpty(t, result.Issues)
}

func TestBlogCheckResultSummary(t *testing.T) {
	r := &BlogCheckResult{GHTypes: 5, BlogDirs: 3}
	s := r.Summary()
	assert.Equal(t, 5, s["ghTypes"])
	assert.Equal(t, 3, s["blogDirs"])
}

// --- ImagesCheckResult ---

func TestImagesCheckResultSummary(t *testing.T) {
	r := &ImagesCheckResult{
		ExpectedDirs:   []string{"a", "b"},
		ExistingDirs:   []string{"b"},
		ExtraDirs:      []string{"c"},
		DuplicateFiles: []string{"dup.jpg"},
		Warnings:       []string{"w1"},
		Errors:         []string{"e1"},
	}
	s := r.Summary()
	assert.Equal(t, 2, s["expectedDirs"])
	assert.Equal(t, 1, s["existingDirs"])
	assert.Equal(t, 1, s["extraDirs"])
	assert.Equal(t, 1, s["duplicateFiles"])
	assert.Equal(t, 1, s["warnings"])
	assert.Equal(t, 1, s["errors"])
}

func TestImagesCheckResultIssues(t *testing.T) {
	r := &ImagesCheckResult{
		ExtraDirs:      []string{"extra"},
		DuplicateFiles: []string{"dup.jpg"},
		Warnings:       []string{"warn"},
		Errors:         []string{"err"},
	}
	issues := r.Issues(ImagesCheckInput{})
	assert.NotEmpty(t, issues)
}

func TestFormatImagesReport(t *testing.T) {
	report := FormatImagesReport(&ImagesCheckResult{}, ImagesCheckInput{})
	assert.NotEmpty(t, report)
}

func TestImagesCheckConfig(t *testing.T) {
	input := ImagesCheckInput{
		DataDir:   "/data",
		ImagesDir: "/images",
		Apply:     true,
		List:      true,
		SkipExtra: true,
	}
	cfg := imagesCheckConfig(input)
	assert.Equal(t, "/data", cfg.DataDir)
	assert.Equal(t, "/images", cfg.ImagesDir)
	assert.True(t, cfg.Apply)
	assert.True(t, cfg.List)
	assert.True(t, cfg.SkipExtra)
}

func TestImagesCheckResultAdapter(t *testing.T) {
	result := &ImagesCheckResult{
		ExpectedDirs:   []string{"a"},
		ExistingDirs:   []string{"b"},
		ExtraDirs:      []string{"b"},
		DuplicateFiles: []string{"dup"},
		Warnings:       []string{"w"},
		Errors:         []string{"e"},
		ApplyActions:   []string{"action"},
	}
	adapted := imagesCheckResult(result)
	assert.Equal(t, result.ExpectedDirs, adapted.ExpectedDirs)
	assert.Equal(t, result.ApplyActions, adapted.ApplyActions)
}

func TestRunImagesCheckEmptyDirs(t *testing.T) {
	result, err := RunImagesCheck(ImagesCheckInput{
		DataDir:   t.TempDir(),
		ImagesDir: t.TempDir(),
	})
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Empty(t, result.ExtraDirs)
}

func TestRunBlogCheckEmpty(t *testing.T) {
	result, err := RunBlogCheck(BlogCheckInput{
		DataDir: t.TempDir(),
		BlogDir: t.TempDir(),
	})
	require.NoError(t, err)
	assert.NotNil(t, result)
}

func TestRunWikiCheckNonExistentGhRoot(t *testing.T) {
	_, err := RunWikiCheck(WikiCheckInput{
		GhRoot:   "/tmp/nonexistent-gh-root-12345",
		WikiRoot: t.TempDir(),
	})
	require.Error(t, err)
}

func TestRunWikiCheckNonExistentWikiRoot(t *testing.T) {
	ghRoot := t.TempDir()
	_, err := RunWikiCheck(WikiCheckInput{
		GhRoot:   ghRoot,
		WikiRoot: "/tmp/nonexistent-wiki-root-12345",
	})
	require.Error(t, err)
}

func TestRunImagesCheckError(t *testing.T) {
	_, err := RunImagesCheck(ImagesCheckInput{
		DataDir:   "/tmp/nonexistent-data-12345",
		ImagesDir: t.TempDir(),
	})
	require.Error(t, err)
}

func TestRunBlogCheckError(t *testing.T) {
	_, err := RunBlogCheck(BlogCheckInput{
		DataDir: "/tmp/nonexistent-data-12345",
		BlogDir: t.TempDir(),
	})
	require.Error(t, err)
}
