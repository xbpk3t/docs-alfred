package wikiingest

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	wikisvc "github.com/xbpk3t/docs-alfred/internal/docs/wiki"
)

// --- RunDigestLocal ---

func TestRunDigestLocalNilConfig(t *testing.T) {
	_, err := RunDigestLocal(context.Background(), DigestLocalInput{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "config is required")
}

func TestRunDigestLocalNonexistentFromDir(t *testing.T) {
	cfg := testConfig(t)
	_, err := RunDigestLocal(context.Background(), DigestLocalInput{
		Config:  cfg,
		FromDir: "/tmp/nonexistent-dir-digest-local-12345",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "source dir")
}

func TestRunDigestLocalEmptyFromDir(t *testing.T) {
	cfg := testConfig(t)
	emptyDir := t.TempDir()
	result, err := RunDigestLocal(context.Background(), DigestLocalInput{
		Config:  cfg,
		FromDir: emptyDir,
		deps:    newFakeDeps().dependencies(),
	})
	require.NoError(t, err)
	assert.Empty(t, result.URLResults)
}

func TestRunDigestLocalWithTranscriptFiles(t *testing.T) {
	cfg := testConfig(t)
	deps := newFakeDeps()
	deps.classifier.results["https://www.bilibili.com/video/BV1abc123/"] = &wikisvc.ClassifyResult{
		TopicPath:   "tech/ai",
		WikiType:    wikisvc.TypeDeepDive,
		ContentType: wikisvc.ContentVideo,
		Summary:     &wikisvc.StructuredSummary{Overview: "great video summary"},
	}

	fromDir := t.TempDir()
	subDir := filepath.Join(fromDir, "BV1abc123_SomeTitle")
	require.NoError(t, os.MkdirAll(subDir, 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(subDir, "bv.txt"), []byte("BV1abc123"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(subDir, "title.txt"), []byte("Some Title"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(subDir, "transcript.md"), []byte(strings.Repeat("transcript content here. ", 50)), 0o600))

	result, err := RunDigestLocal(context.Background(), DigestLocalInput{
		Config:  cfg,
		FromDir: fromDir,
		deps:    deps.dependencies(),
	})

	require.NoError(t, err)
	assert.Len(t, result.URLResults, 1)
	assert.Equal(t, StatusSummaryWritten, result.URLResults[0].Status)
}

// --- readLocalInputs ---

func TestReadLocalInputsMissingBV(t *testing.T) {
	dir := t.TempDir()
	_, err := readLocalInputs(dir)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "read bv.txt")
}

func TestReadLocalInputsEmptyBV(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "bv.txt"), []byte("  \n"), 0o600))
	_, err := readLocalInputs(dir)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "empty bv.txt")
}

func TestReadLocalInputsMissingTitle(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "bv.txt"), []byte("BV123"), 0o600))
	_, err := readLocalInputs(dir)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "read title.txt")
}

func TestReadLocalInputsEmptyTitleFallsBackToBV(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "bv.txt"), []byte("BV123"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "title.txt"), []byte("  \n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "transcript.md"), []byte(strings.Repeat("long transcript content. ", 50)), 0o600))

	inputs, err := readLocalInputs(dir)
	require.NoError(t, err)
	assert.Equal(t, "BV123", inputs.title)
	assert.Equal(t, "BV123", inputs.bv)
}

func TestReadLocalInputsMissingTranscript(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "bv.txt"), []byte("BV123"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "title.txt"), []byte("Title"), 0o600))
	_, err := readLocalInputs(dir)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "read transcript.md")
}

func TestReadLocalInputsTranscriptTooShort(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "bv.txt"), []byte("BV123"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "title.txt"), []byte("Title"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "transcript.md"), []byte("short"), 0o600))

	_, err := readLocalInputs(dir)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "too short")
}

func TestReadLocalInputsSuccess(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "bv.txt"), []byte("BV1abc\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "title.txt"), []byte("My Title\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "transcript.md"), []byte(strings.Repeat("This is a long enough transcript for testing. ", 20)), 0o600))

	inputs, err := readLocalInputs(dir)
	require.NoError(t, err)
	assert.Equal(t, "BV1abc", inputs.bv)
	assert.Equal(t, "My Title", inputs.title)
	assert.NotEmpty(t, inputs.content)
}

// --- processLocalDir ---

func TestProcessLocalDirValidInputs(t *testing.T) {
	deps := newFakeDeps()
	url := "https://www.bilibili.com/video/BV1abc123/"
	deps.classifier.results[url] = &wikisvc.ClassifyResult{
		TopicPath:   "tech/ai",
		WikiType:    wikisvc.TypeDeepDive,
		ContentType: wikisvc.ContentVideo,
		Summary:     &wikisvc.StructuredSummary{Overview: "great summary"},
	}

	dir := t.TempDir()
	wikiRoot := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "bv.txt"), []byte("BV1abc123"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "title.txt"), []byte("Title"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "transcript.md"), []byte(strings.Repeat("transcript content. ", 50)), 0o600))

	result := processLocalDir(context.Background(), deps.dependencies(), wikiRoot, dir)
	assert.Equal(t, StatusSummaryWritten, result.Status)
	assert.True(t, result.Handled)
	assert.Equal(t, url, result.URL)
}

func TestProcessLocalDirClassifierReturnsNil(t *testing.T) {
	deps := newFakeDeps()
	// No classifier result set → returns nil

	dir := t.TempDir()
	wikiRoot := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "bv.txt"), []byte("BV1abc123"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "title.txt"), []byte("Title"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "transcript.md"), []byte(strings.Repeat("transcript content. ", 50)), 0o600))

	result := processLocalDir(context.Background(), deps.dependencies(), wikiRoot, dir)
	assert.Equal(t, StatusUnhandledError, result.Status)
	assert.Contains(t, result.Error, "classification failed")
}

func TestProcessLocalDirClassifierNilEmptyContent(t *testing.T) {
	deps := newFakeDeps()
	// BV URL with bilibili prefix → video content, 200 chars < 600 → video too short extract failure.
	// This exercises the video content quality gate path.

	dir := t.TempDir()
	wikiRoot := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "bv.txt"), []byte("BV1abc123"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "title.txt"), []byte("Title"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "transcript.md"), []byte(strings.Repeat("x", 200)), 0o600))

	result := processLocalDir(context.Background(), deps.dependencies(), wikiRoot, dir)
	assert.Equal(t, StatusFailureWritten, result.Status)
	assert.Equal(t, wikisvc.FailureExtract, result.FailureType)
	assert.True(t, result.Handled)
}

func TestProcessLocalDirClassifyFailure(t *testing.T) {
	deps := newFakeDeps()
	url := "https://www.bilibili.com/video/BV1abc123/"
	deps.classifier.results[url] = &wikisvc.ClassifyResult{
		TopicPath:    "none",
		WikiType:     wikisvc.TypeInbox,
		ContentType:  wikisvc.ContentVideo,
		Summary:      &wikisvc.StructuredSummary{Overview: "something"},
	}

	dir := t.TempDir()
	wikiRoot := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "bv.txt"), []byte("BV1abc123"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "title.txt"), []byte("Title"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "transcript.md"), []byte(strings.Repeat("transcript content. ", 50)), 0o600))

	result := processLocalDir(context.Background(), deps.dependencies(), wikiRoot, dir)
	// Good summary with none path → uncat success, not failure JSONL
	assert.Equal(t, StatusSummaryWritten, result.Status)
	assert.Contains(t, result.OutputPath, "uncat.md")
}

func TestProcessLocalDirVideoTooShort(t *testing.T) {
	deps := newFakeDeps()
	// Content will be < 600 runes → extract failure

	dir := t.TempDir()
	wikiRoot := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "bv.txt"), []byte("BV1abc123"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "title.txt"), []byte("Title"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "transcript.md"), []byte(strings.Repeat("x", 200)), 0o600))

	result := processLocalDir(context.Background(), deps.dependencies(), wikiRoot, dir)
	assert.Equal(t, StatusFailureWritten, result.Status)
	assert.Equal(t, wikisvc.FailureExtract, result.FailureType)
	assert.True(t, result.Handled)
}

func TestProcessLocalDirReadInputsError(t *testing.T) {
	deps := newFakeDeps()
	dir := t.TempDir()
	// No bv.txt → readLocalInputs fails → skipResult
	result := processLocalDir(context.Background(), deps.dependencies(), t.TempDir(), dir)
	assert.Equal(t, StatusFailureWritten, result.Status)
	assert.True(t, result.Handled)
}

// --- copyTranscriptToWiki ---

func TestCopyTranscriptToWikiSuccess(t *testing.T) {
	srcDir := t.TempDir()
	wikiRoot := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(srcDir, "transcript.md"), []byte("transcript data"), 0o600))

	err := copyTranscriptToWiki(srcDir, wikiRoot, "tech/ai", "BV1abc")
	require.NoError(t, err)

	dst := filepath.Join(wikiRoot, "tech", "ai", "transcript", "transcript-BV1abc.md")
	data, readErr := os.ReadFile(dst)
	require.NoError(t, readErr)
	assert.Equal(t, "transcript data", string(data))
}

func TestCopyTranscriptToWikiPathTraversal(t *testing.T) {
	srcDir := t.TempDir()
	wikiRoot := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(srcDir, "transcript.md"), []byte("data"), 0o600))

	// Attempt path traversal via topicPath
	err := copyTranscriptToWiki(srcDir, wikiRoot, "../../../etc", "BV1abc")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "escapes wiki root")
}

func TestCopyTranscriptToWikiPathTraversalBV(t *testing.T) {
	srcDir := t.TempDir()
	wikiRoot := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(srcDir, "transcript.md"), []byte("data"), 0o600))

	// Attempt path traversal via bv (filepath.Base is used, so ../.. becomes just the base)
	// Actually filepath.Base("../../etc") = "etc", so this won't traverse.
	// The dst check is: !strings.HasPrefix(dst, dstDir)
	// We need to construct a bv that when used with filepath.Base produces something
	// that escapes dstDir. But filepath.Base strips directory components, so this
	// path is actually safe. Test that it works fine.
	err := copyTranscriptToWiki(srcDir, wikiRoot, "tech/ai", "../../etc")
	require.NoError(t, err)
}

func TestCopyTranscriptToWikiMissingSource(t *testing.T) {
	srcDir := t.TempDir()
	wikiRoot := t.TempDir()

	err := copyTranscriptToWiki(srcDir, wikiRoot, "tech/ai", "BV1abc")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "read source transcript")
}

// --- skipResult ---

func TestSkipResult(t *testing.T) {
	r := skipResult("myDir", "some reason")
	assert.Equal(t, "myDir", r.URL)
	assert.Equal(t, StatusFailureWritten, r.Status)
	assert.True(t, r.Handled)
}
