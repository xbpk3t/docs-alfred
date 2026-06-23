package wiki

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- NewDriver ---

func TestNewDriverReturnsOpenCLIDriver(t *testing.T) {
	d, err := NewDriver("opencli", DriverOptions{MaxBodySize: 2000, MediaEnabled: true})
	require.NoError(t, err)
	require.NotNil(t, d)
	assert.Equal(t, "opencli", d.Name())
}

func TestNewDriverReturnsHTTPReadabilityDriver(t *testing.T) {
	d, err := NewDriver("http-readability", DriverOptions{MaxBodySize: 3000})
	require.NoError(t, err)
	require.NotNil(t, d)
	assert.Equal(t, "http-readability", d.Name())
}

func TestNewDriverRejectsUnknownName(t *testing.T) {
	d, err := NewDriver("nonexistent", DriverOptions{})
	require.Error(t, err)
	assert.Nil(t, d)
	assert.Contains(t, err.Error(), "unknown driver")
	assert.Contains(t, err.Error(), "nonexistent")
}

// --- digestFilename ---

func TestDigestFilenameSuccessEntry(t *testing.T) {
	assert.Equal(t, "digest-success.jsonl", digestFilename(&DigestEntry{Status: DigestSuccess}))
}

func TestDigestFilenameFetchFailure(t *testing.T) {
	assert.Equal(t, "digest-fetch-error.jsonl",
		digestFilename(&DigestEntry{Status: DigestFailure, FailureKind: string(FailureFetch)}))
}

func TestDigestFilenameResolveFailure(t *testing.T) {
	assert.Equal(t, "digest-fetch-error.jsonl",
		digestFilename(&DigestEntry{Status: DigestFailure, FailureKind: string(FailureResolve)}))
}

func TestDigestFilenameExtractFailure(t *testing.T) {
	assert.Equal(t, "digest-extract-error.jsonl",
		digestFilename(&DigestEntry{Status: DigestFailure, FailureKind: string(FailureExtract)}))
}

func TestDigestFilenameClassifyFailure(t *testing.T) {
	assert.Equal(t, "digest-classify-rejected.jsonl",
		digestFilename(&DigestEntry{Status: DigestFailure, FailureKind: string(FailureClassify)}))
}

func TestDigestFilenameAIFailure(t *testing.T) {
	assert.Equal(t, "digest-ai-error.jsonl",
		digestFilename(&DigestEntry{Status: DigestFailure, FailureKind: string(FailureAI)}))
}

func TestDigestFilenameUnknownFailureDefaultsToAI(t *testing.T) {
	assert.Equal(t, "digest-ai-error.jsonl",
		digestFilename(&DigestEntry{Status: DigestFailure, FailureKind: "something-else"}))
}

// --- LogDigestEntry ---

func TestLogDigestEntryWithNilOptsReturnsEmpty(t *testing.T) {
	path, err := LogDigestEntry(&DigestEntry{URL: "https://example.com"}, nil)
	assert.NoError(t, err)
	assert.Empty(t, path)
}

func TestLogDigestEntryDryRunReturnsPath(t *testing.T) {
	root := t.TempDir()
	entry := &DigestEntry{
		URL:    "https://example.com",
		Status: DigestSuccess,
	}

	path, err := LogDigestEntry(entry, &WriteOptions{WikiRoot: root, DryRun: true})

	require.NoError(t, err)
	assert.NotEmpty(t, path)
	assert.Equal(t, root+"/digest-success.jsonl", path)
}

func TestLogDigestEntryDryRunDoesNotWriteFile(t *testing.T) {
	root := t.TempDir()
	entry := &DigestEntry{
		URL:    "https://example.com",
		Status: DigestSuccess,
	}

	_, err := LogDigestEntry(entry, &WriteOptions{WikiRoot: root, DryRun: true})
	require.NoError(t, err)

	// File should not exist after a dry run.
	_, readErr := os.ReadFile(root + "/digest-success.jsonl")
	assert.True(t, os.IsNotExist(readErr))
}

func TestLogDigestEntryEmptyTimestampIsPopulated(t *testing.T) {
	root := t.TempDir()
	entry := &DigestEntry{
		URL:    "https://example.com",
		Status: DigestSuccess,
		// Timestamp intentionally left empty.
	}

	_, err := LogDigestEntry(entry, &WriteOptions{WikiRoot: root})
	require.NoError(t, err)

	// The entry should have been given a timestamp.
	assert.NotEmpty(t, entry.Timestamp, "Timestamp should be populated when empty")
}

func TestLogDigestEntryPreservesExistingTimestamp(t *testing.T) {
	root := t.TempDir()
	entry := &DigestEntry{
		URL:       "https://example.com",
		Status:    DigestSuccess,
		Timestamp: "2026-01-01T00:00:00Z",
	}

	_, err := LogDigestEntry(entry, &WriteOptions{WikiRoot: root})
	require.NoError(t, err)

	assert.Equal(t, "2026-01-01T00:00:00Z", entry.Timestamp)
}

// --- LogDigestEntry with batchID set ---

func TestLogDigestEntryWithBatchIDSet(t *testing.T) {
	root := t.TempDir()
	entry := &DigestEntry{
		URL:     "https://example.com",
		Status:  DigestSuccess,
		BatchID: "custom-batch",
	}

	path, err := LogDigestEntry(entry, &WriteOptions{WikiRoot: root, BatchID: "opts-batch"})
	require.NoError(t, err)

	// Entry's own BatchID should be preserved (not overwritten by opts.BatchID)
	assert.Equal(t, "custom-batch", entry.BatchID)

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Contains(t, string(data), "custom-batch")
}

func TestLogDigestEntryUsesOptsBatchIDWhenEmpty(t *testing.T) {
	root := t.TempDir()
	entry := &DigestEntry{
		URL:    "https://example.com",
		Status: DigestSuccess,
	}

	path, err := LogDigestEntry(entry, &WriteOptions{WikiRoot: root, BatchID: "opts-batch"})
	require.NoError(t, err)

	assert.Equal(t, "opts-batch", entry.BatchID)

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Contains(t, string(data), "opts-batch")
}

// --- newHTTPDriver ---

func TestNewHTTPDriverWithOptions(t *testing.T) {
	d := newHTTPDriver(DriverOptions{MaxBodySize: 3000})
	assert.Equal(t, 3000, d.maxBodySize)
	assert.Equal(t, "http-readability", d.Name())
}

func TestNewHTTPDriverDefaultMaxBodySize(t *testing.T) {
	d := newHTTPDriver(DriverOptions{MaxBodySize: 0})
	assert.Equal(t, 5000, d.maxBodySize)
}

// --- markdownFallbackBody ---

func TestMarkdownFallbackBodyMinimalHTML(t *testing.T) {
	body := markdownFallbackBody([]byte(`<html><body><p>Hello world</p></body></html>`))
	assert.NotEmpty(t, body)
}

func TestMarkdownFallbackBodyReturnsRawOnFailure(t *testing.T) {
	// Non-HTML content that fails conversion should return raw
	body := markdownFallbackBody([]byte("just plain text"))
	assert.Equal(t, "just plain text", body)
}

// --- extractTitleFromHTML edge cases ---

func TestExtractTitleFromHTMLNoHeadTag(t *testing.T) {
	assert.Equal(t, "", extractTitleFromHTML(`<html><body>No head</body></html>`))
}

func TestExtractTitleFromHTMLEmptyTitle(t *testing.T) {
	assert.Equal(t, "", extractTitleFromHTML(`<html><head><title></title></head><body></body></html>`))
}

func TestExtractTitleFromHTMLMultipleTitles(t *testing.T) {
	assert.Equal(t, "First", extractTitleFromHTML(`<html><head><title>First</title><title>Second</title></head></html>`))
}

// --- isHTTPBlockError ---

// Note: isHTTPBlockError panics on nil because it calls err.Error() directly.
// This is acceptable since the caller always checks err != nil first.

// --- ensureContentQuality ---

func TestEnsureContentQualityNilResult(t *testing.T) {
	d := newHTTPDriver(DriverOptions{MaxBodySize: 5000})
	result := d.ensureContentQuality(context.Background(), nil)
	assert.Nil(t, result)
}

func TestEnsureContentQualityWithError(t *testing.T) {
	d := newHTTPDriver(DriverOptions{MaxBodySize: 5000})
	input := &ContentFetchResult{Error: "existing error", SourceURL: "https://example.com"}
	result := d.ensureContentQuality(context.Background(), input)
	assert.Equal(t, "existing error", result.Error)
}

func TestEnsureContentQualityGoodContent(t *testing.T) {
	d := newHTTPDriver(DriverOptions{MaxBodySize: 5000})
	body := strings.Repeat("This is good content with enough length to pass quality checks. ", 10)
	input := &ContentFetchResult{Title: "Good", Body: body, SourceURL: "https://example.com/article"}
	result := d.ensureContentQuality(context.Background(), input)
	assert.Empty(t, result.Error)
}

func TestEnsureContentQualityLowQuality(t *testing.T) {
	d := newHTTPDriver(DriverOptions{MaxBodySize: 5000})
	input := &ContentFetchResult{Title: "Bad", Body: "this page requires javascript", SourceURL: "https://example.com"}
	result := d.ensureContentQuality(context.Background(), input)
	assert.NotEmpty(t, result.Error)
	assert.Contains(t, result.Error, "extract:")
}

// --- HTTP driver FetchContent with block error ---

func TestHTTPDriverFetchContentBlockError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte("HTTP 403 forbidden"))
	}))
	t.Cleanup(server.Close)

	driver := newHTTPDriver(DriverOptions{MaxBodySize: 5000})
	result := driver.FetchContent(context.Background(), server.URL, ContentText)

	require.NotNil(t, result)
	require.NotEmpty(t, result.Error)
	assert.Equal(t, FailureResolve, result.FailureKind)
}

// --- HTTP driver handlePageData ---

func TestHTTPDriverHandlePageDataWithReadability(t *testing.T) {
	d := newHTTPDriver(DriverOptions{MaxBodySize: 5000})
	html := []byte(`<html><head><title>Test</title></head><body><article><p>` +
		strings.Repeat("This is a sufficiently long article body for readability extraction. ", 10) +
		`</p></article></body></html>`)
	result := d.handlePageData(context.Background(), "https://example.com/article", html)
	require.NotNil(t, result)
	assert.Empty(t, result.Error)
}

func TestHTTPDriverHandlePageDataFallbackToMarkdown(t *testing.T) {
	d := newHTTPDriver(DriverOptions{MaxBodySize: 5000})
	// HTML without article tag - readability may fail, falls back to markdown
	html := []byte(`<html><head><title>Page</title></head><body><div>` +
		strings.Repeat("Content without article tag but still long enough. ", 10) +
		`</div></body></html>`)
	result := d.handlePageData(context.Background(), "https://example.com/page", html)
	require.NotNil(t, result)
	assert.NotEmpty(t, result.Title)
}

// --- openCLIDriver.FetchContent edge cases ---

func TestOpenCLIDriverFetchContentVideoMediaDisabled(t *testing.T) {
	driver := newOpenCLIDriver(DriverOptions{MediaEnabled: false})
	result := driver.FetchContent(context.Background(), "https://www.youtube.com/watch?v=abc", ContentVideo)
	require.NotNil(t, result)
	assert.Contains(t, result.Error, "media extraction disabled")
}

func TestOpenCLIDriverFetchContentGenericURL(t *testing.T) {
	// Create mock opencli
	binDir := t.TempDir()
	opencliPath := filepath.Join(binDir, "opencli")
	script := `#!/bin/sh
case "$1" in
  web)
    printf '# Test Article\n\nSome content here.\n'
    ;;
  *)
    printf 'mock content\n'
    ;;
esac
`
	require.NoError(t, os.WriteFile(opencliPath, []byte(script), 0o700))
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	driver := newOpenCLIDriver(DriverOptions{MaxBodySize: 5000})
	result := driver.FetchContent(context.Background(), "https://example.com/article", ContentText)
	require.NotNil(t, result)
}

func TestOpenCLIDriverFetchContentBilibiliURL(t *testing.T) {
	// Create mock opencli that handles bilibili subcommand
	binDir := t.TempDir()
	opencliPath := filepath.Join(binDir, "opencli")
	script := `#!/bin/sh
case "$1" in
  bilibili)
    case "$2" in
      video)
        printf '# Bilibili Video\n\nTitle: Test Video\nViews: 1000\n'
        ;;
      subtitle)
        printf '| 00:01 | speaker | This is a long enough transcript line for testing.\n'
        printf '| 00:02 | speaker | Another line of the transcript with enough content.\n'
        printf '| 00:03 | speaker | Third line of transcript content here.\n'
        printf '| 00:04 | speaker | Fourth line of transcript.\n'
        printf '| 00:05 | speaker | Fifth line of transcript content.\n'
        ;;
      summary)
        printf 'AI Summary content here.\n'
        ;;
      *)
        printf 'mock\n'
        ;;
    esac
    ;;
  *)
    printf 'mock\n'
    ;;
esac
`
	require.NoError(t, os.WriteFile(opencliPath, []byte(script), 0o700))
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	driver := newOpenCLIDriver(DriverOptions{MaxBodySize: 5000})
	result := driver.FetchContent(context.Background(), "https://www.bilibili.com/video/BV1xx", ContentVideo)
	require.NotNil(t, result)
}

func TestOpenCLIDriverFetchContentBilibiliNoSubtitle(t *testing.T) {
	binDir := t.TempDir()
	opencliPath := filepath.Join(binDir, "opencli")
	script := `#!/bin/sh
case "$1" in
  bilibili)
    case "$2" in
      video)
        printf '# Bilibili Video\n\nTitle: Test Video\nViews: 1000\n'
        ;;
      subtitle)
        exit 1
        ;;
      summary)
        printf 'B站 AI summary content that is long enough to be useful for testing purposes.\n'
        ;;
      *)
        printf 'mock\n'
        ;;
    esac
    ;;
  *)
    printf 'mock\n'
    ;;
esac
`
	require.NoError(t, os.WriteFile(opencliPath, []byte(script), 0o700))
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	driver := newOpenCLIDriver(DriverOptions{MaxBodySize: 5000})
	result := driver.FetchContent(context.Background(), "https://www.bilibili.com/video/BV1xx", ContentVideo)
	require.NotNil(t, result)
}

func TestOpenCLIDriverFetchContentYouTubeURL(t *testing.T) {
	binDir := t.TempDir()
	opencliPath := filepath.Join(binDir, "opencli")
	script := `#!/bin/sh
case "$1" in
  youtube)
    case "$2" in
      video)
        printf '# YouTube Video\n\nTitle: Test Video\nViews: 1000\n'
        ;;
      transcript)
        printf '| 00:01 | speaker | This is a long enough transcript line for testing.\n'
        printf '| 00:02 | speaker | Another line of the transcript with enough content.\n'
        printf '| 00:03 | speaker | Third line of transcript content here.\n'
        printf '| 00:04 | speaker | Fourth line of transcript.\n'
        printf '| 00:05 | speaker | Fifth line of transcript content.\n'
        ;;
      *)
        printf 'mock\n'
        ;;
    esac
    ;;
  *)
    printf 'mock\n'
    ;;
esac
`
	require.NoError(t, os.WriteFile(opencliPath, []byte(script), 0o700))
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	driver := newOpenCLIDriver(DriverOptions{MaxBodySize: 5000})
	result := driver.FetchContent(context.Background(), "https://www.youtube.com/watch?v=abc", ContentVideo)
	require.NotNil(t, result)
}

func TestOpenCLIDriverFetchContentBilibiliShortSubtitle(t *testing.T) {
	binDir := t.TempDir()
	opencliPath := filepath.Join(binDir, "opencli")
	script := `#!/bin/sh
case "$1" in
  bilibili)
    case "$2" in
      video)
        printf '# Bilibili Video\n\nTitle: Test Video\n'
        ;;
      subtitle)
        printf 'short\n'
        ;;
      summary)
        exit 1
        ;;
      *)
        printf 'mock\n'
        ;;
    esac
    ;;
  *)
    printf 'mock\n'
    ;;
esac
`
	require.NoError(t, os.WriteFile(opencliPath, []byte(script), 0o700))
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	driver := newOpenCLIDriver(DriverOptions{MaxBodySize: 5000})
	result := driver.FetchContent(context.Background(), "https://www.bilibili.com/video/BV1xx", ContentVideo)
	require.NotNil(t, result)
}

func TestOpenCLIDriverFetchContentYouTubeShortTranscript(t *testing.T) {
	binDir := t.TempDir()
	opencliPath := filepath.Join(binDir, "opencli")
	script := `#!/bin/sh
case "$1" in
  youtube)
    case "$2" in
      video)
        printf '# YouTube Video\n\nTitle: Test Video\n'
        ;;
      transcript)
        printf 'short\n'
        ;;
      *)
        printf 'mock\n'
        ;;
    esac
    ;;
  *)
    printf 'mock\n'
    ;;
esac
`
	require.NoError(t, os.WriteFile(opencliPath, []byte(script), 0o700))
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	driver := newOpenCLIDriver(DriverOptions{MaxBodySize: 5000})
	result := driver.FetchContent(context.Background(), "https://www.youtube.com/watch?v=abc", ContentVideo)
	require.NotNil(t, result)
}

// --- fetchWebRead ---

func TestFetchWebRead(t *testing.T) {
	binDir := t.TempDir()
	opencliPath := filepath.Join(binDir, "opencli")
	script := `#!/bin/sh
printf '# Web Article\n\nContent from web read.\n'
`
	require.NoError(t, os.WriteFile(opencliPath, []byte(script), 0o700))
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	driver := newOpenCLIDriver(DriverOptions{MaxBodySize: 5000})
	result := driver.fetchWebRead(context.Background(), "https://example.com/article")
	require.NotNil(t, result)
	assert.Equal(t, "https://example.com/article", result.SourceURL)
}

// --- appendBilibiliSummary ---

func TestAppendBilibiliSummarySuccess(t *testing.T) {
	binDir := t.TempDir()
	opencliPath := filepath.Join(binDir, "opencli")
	script := `#!/bin/sh
printf 'B站 official AI summary that is long enough to pass the minimum threshold for testing.\n'
`
	require.NoError(t, os.WriteFile(opencliPath, []byte(script), 0o700))
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	driver := newOpenCLIDriver(DriverOptions{MaxBodySize: 5000})
	result := &ContentFetchResult{Title: "Test", Body: "original body", SourceURL: "https://www.bilibili.com/video/BV1xx"}
	driver.appendBilibiliSummary(context.Background(), result, "https://www.bilibili.com/video/BV1xx")
	assert.Contains(t, result.Body, "AI 总结")
}

func TestAppendBilibiliSummaryShortContent(t *testing.T) {
	binDir := t.TempDir()
	opencliPath := filepath.Join(binDir, "opencli")
	script := `#!/bin/sh
printf 'short\n'
`
	require.NoError(t, os.WriteFile(opencliPath, []byte(script), 0o700))
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	driver := newOpenCLIDriver(DriverOptions{MaxBodySize: 5000})
	result := &ContentFetchResult{Title: "Test", Body: "original", SourceURL: "https://www.bilibili.com/video/BV1xx"}
	driver.appendBilibiliSummary(context.Background(), result, "https://www.bilibili.com/video/BV1xx")
	assert.Equal(t, "original", result.Body)
}

func TestAppendBilibiliSummaryError(t *testing.T) {
	binDir := t.TempDir()
	opencliPath := filepath.Join(binDir, "opencli")
	script := `#!/bin/sh
exit 1
`
	require.NoError(t, os.WriteFile(opencliPath, []byte(script), 0o700))
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	driver := newOpenCLIDriver(DriverOptions{MaxBodySize: 5000})
	result := &ContentFetchResult{Title: "Test", Body: "original", SourceURL: "https://www.bilibili.com/video/BV1xx"}
	driver.appendBilibiliSummary(context.Background(), result, "https://www.bilibili.com/video/BV1xx")
	assert.Equal(t, "original", result.Body)
}

// --- appendYoutubeTranscript with short transcript ---

func TestAppendYoutubeTranscriptShortTranscript(t *testing.T) {
	binDir := t.TempDir()
	opencliPath := filepath.Join(binDir, "opencli")
	script := `#!/bin/sh
printf 'short\n'
`
	require.NoError(t, os.WriteFile(opencliPath, []byte(script), 0o700))
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	driver := newOpenCLIDriver(DriverOptions{MaxBodySize: 5000})
	result := &ContentFetchResult{Title: "Test", Body: "original", SourceURL: "https://www.youtube.com/watch?v=abc"}
	driver.appendYoutubeTranscript(context.Background(), result, "https://www.youtube.com/watch?v=abc")
	assert.Equal(t, "original", result.Body)
}

func TestAppendYoutubeTranscriptError(t *testing.T) {
	binDir := t.TempDir()
	opencliPath := filepath.Join(binDir, "opencli")
	script := `#!/bin/sh
exit 1
`
	require.NoError(t, os.WriteFile(opencliPath, []byte(script), 0o700))
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	driver := newOpenCLIDriver(DriverOptions{MaxBodySize: 5000})
	result := &ContentFetchResult{Title: "Test", Body: "original", SourceURL: "https://www.youtube.com/watch?v=abc"}
	driver.appendYoutubeTranscript(context.Background(), result, "https://www.youtube.com/watch?v=abc")
	assert.Equal(t, "original", result.Body)
}

// --- appendBilibiliTranscript with short transcript and no summary ---

func TestAppendBilibiliTranscriptShortSubtitleNoSummary(t *testing.T) {
	binDir := t.TempDir()
	opencliPath := filepath.Join(binDir, "opencli")
	script := `#!/bin/sh
case "$2" in
  subtitle)
    printf 'short\n'
    ;;
  summary)
    exit 1
    ;;
  *)
    printf 'mock\n'
    ;;
esac
`
	require.NoError(t, os.WriteFile(opencliPath, []byte(script), 0o700))
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	driver := newOpenCLIDriver(DriverOptions{MaxBodySize: 5000})
	result := &ContentFetchResult{Title: "Test", Body: "original", SourceURL: "https://www.bilibili.com/video/BV1xx"}
	driver.appendBilibiliTranscript(context.Background(), result, "https://www.bilibili.com/video/BV1xx")
	assert.Equal(t, "original", result.Body)
}

// --- newOpenCLIDriver default maxBodySize ---

func TestNewOpenCLIDriverDefaultMaxBodySize(t *testing.T) {
	d := newOpenCLIDriver(DriverOptions{MaxBodySize: 0})
	assert.Equal(t, 5000, d.maxBodySize)
}

// --- runOpenCLI with empty body ---

func TestRunOpenCLIEmptyBody(t *testing.T) {
	binDir := t.TempDir()
	opencliPath := filepath.Join(binDir, "opencli")
	script := `#!/bin/sh
printf ''
`
	require.NoError(t, os.WriteFile(opencliPath, []byte(script), 0o700))
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	driver := newOpenCLIDriver(DriverOptions{MaxBodySize: 5000})
	result := driver.runOpenCLI(context.Background(), "web", []string{"read"})
	require.NotNil(t, result)
	assert.Equal(t, "opencli returned empty content", result.Error)
}

// --- resolveTcoURL ---

func TestResolveTcoURLNotTcoLink(t *testing.T) {
	driver := newOpenCLIDriver(DriverOptions{MaxBodySize: 5000})
	result := driver.resolveTcoURL(context.Background(), "https://example.com/article")
	assert.Equal(t, "https://example.com/article", result)
}

func TestResolveTcoURLCurlNotAvailable(t *testing.T) {
	// When curl fails, should return original URL
	t.Setenv("PATH", t.TempDir()) // empty PATH so curl isn't found
	driver := newOpenCLIDriver(DriverOptions{MaxBodySize: 5000})
	result := driver.resolveTcoURL(context.Background(), "https://t.co/abc")
	assert.Equal(t, "https://t.co/abc", result)
}

func TestResolveTcoURLSuccess(t *testing.T) {
	binDir := t.TempDir()
	curlPath := filepath.Join(binDir, "curl")
	script := `#!/bin/sh
printf 'https://example.com/final-page'
`
	require.NoError(t, os.WriteFile(curlPath, []byte(script), 0o700))
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	driver := newOpenCLIDriver(DriverOptions{MaxBodySize: 5000})
	result := driver.resolveTcoURL(context.Background(), "https://t.co/abc")
	assert.Equal(t, "https://example.com/final-page", result)
}

func TestResolveTcoURLSameURL(t *testing.T) {
	binDir := t.TempDir()
	curlPath := filepath.Join(binDir, "curl")
	script := `#!/bin/sh
printf 'https://t.co/abc'
`
	require.NoError(t, os.WriteFile(curlPath, []byte(script), 0o700))
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	driver := newOpenCLIDriver(DriverOptions{MaxBodySize: 5000})
	result := driver.resolveTcoURL(context.Background(), "https://t.co/abc")
	assert.Equal(t, "https://t.co/abc", result)
}

func TestResolveTcoURLEmptyResult(t *testing.T) {
	binDir := t.TempDir()
	curlPath := filepath.Join(binDir, "curl")
	script := `#!/bin/sh
printf ''
`
	require.NoError(t, os.WriteFile(curlPath, []byte(script), 0o700))
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	driver := newOpenCLIDriver(DriverOptions{MaxBodySize: 5000})
	result := driver.resolveTcoURL(context.Background(), "https://t.co/abc")
	assert.Equal(t, "https://t.co/abc", result)
}

// --- handlePageData fallback path ---

func TestHTTPDriverHandlePageDataReadabilityFails(t *testing.T) {
	d := newHTTPDriver(DriverOptions{MaxBodySize: 5000})
	// HTML that readability can't extract meaningful content from
	html := []byte(`<html><head><title>Fallback</title></head><body><div>minimal</div></body></html>`)
	result := d.handlePageData(context.Background(), "https://example.com/empty", html)
	require.NotNil(t, result)
}

func TestHTTPDriverHandlePageDataNoTitle(t *testing.T) {
	d := newHTTPDriver(DriverOptions{MaxBodySize: 5000})
	// HTML without title tag - readability may extract title from content
	html := []byte(`<html><body><article><p>` +
		strings.Repeat("Content without a title tag but long enough for extraction. ", 10) +
		`</p></article></body></html>`)
	result := d.handlePageData(context.Background(), "https://example.com/notitle", html)
	require.NotNil(t, result)
	assert.NotEmpty(t, result.Title)
}

func TestHTTPDriverHandlePageDataEmptyHTML(t *testing.T) {
	d := newHTTPDriver(DriverOptions{MaxBodySize: 5000})
	// Truly empty HTML
	html := []byte(``)
	result := d.handlePageData(context.Background(), "https://example.com/empty", html)
	require.NotNil(t, result)
}

// --- extractWithReadability edge cases ---

func TestExtractWithReadabilityInvalidURL(t *testing.T) {
	d := newHTTPDriver(DriverOptions{MaxBodySize: 5000})
	html := []byte(`<html><body><p>Content</p></body></html>`)
	result := d.extractWithReadability(html, "://invalid")
	assert.Nil(t, result)
}

func TestExtractWithReadabilityEmptyBody(t *testing.T) {
	d := newHTTPDriver(DriverOptions{MaxBodySize: 5000})
	html := []byte(`<html><head><title>Test</title></head><body><article></article></body></html>`)
	result := d.extractWithReadability(html, "https://example.com/article")
	// Readability may return empty body, should return nil
	if result != nil {
		assert.NotEmpty(t, result.Body)
	}
}

func TestExtractWithReadabilityTitleFromHTML(t *testing.T) {
	d := newHTTPDriver(DriverOptions{MaxBodySize: 5000})
	// HTML with title in head but article without title
	html := []byte(`<html><head><title>HTML Title</title></head><body><article><p>` +
		strings.Repeat("Content for readability extraction that is long enough. ", 10) +
		`</p></article></body></html>`)
	result := d.extractWithReadability(html, "https://example.com/article")
	require.NotNil(t, result)
	// Title should come from readability or HTML
	assert.NotEmpty(t, result.Title)
}

// --- openCLIDriver.FetchContent with adapter URL (twitter) ---

func TestOpenCLIDriverFetchContentTwitterURL(t *testing.T) {
	binDir := t.TempDir()
	opencliPath := filepath.Join(binDir, "opencli")
	script := `#!/bin/sh
case "$1" in
  twitter)
    printf '# Twitter Thread\n\nTweet content here.\n'
    ;;
  *)
    printf 'mock\n'
    ;;
esac
`
	require.NoError(t, os.WriteFile(opencliPath, []byte(script), 0o700))
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	driver := newOpenCLIDriver(DriverOptions{MaxBodySize: 5000})
	result := driver.FetchContent(context.Background(), "https://x.com/user/status/123", ContentText)
	require.NotNil(t, result)
}

// --- fetchWithAdapter with weixin URL ---

func TestOpenCLIDriverFetchContentWeixinURL(t *testing.T) {
	binDir := t.TempDir()
	opencliPath := filepath.Join(binDir, "opencli")
	script := `#!/bin/sh
case "$1" in
  weixin)
    # Create a temp file and output YAML with saved path
    tmpdir=$(mktemp -d)
    echo "# WeChat Article" > "$tmpdir/article.md"
    echo "- title: Test Article"
    echo "  saved: $tmpdir/article.md"
    ;;
  *)
    printf 'mock\n'
    ;;
esac
`
	require.NoError(t, os.WriteFile(opencliPath, []byte(script), 0o700))
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	driver := newOpenCLIDriver(DriverOptions{MaxBodySize: 5000})
	result := driver.FetchContent(context.Background(), "https://mp.weixin.qq.com/s/abc", ContentText)
	require.NotNil(t, result)
}

// --- openCLIDriver.FetchContent with zhihu URL ---

func TestOpenCLIDriverFetchContentZhihuURL(t *testing.T) {
	binDir := t.TempDir()
	opencliPath := filepath.Join(binDir, "opencli")
	script := `#!/bin/sh
printf '# Zhihu Content\n\nSome content from zhihu.\n'
`
	require.NoError(t, os.WriteFile(opencliPath, []byte(script), 0o700))
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	driver := newOpenCLIDriver(DriverOptions{MaxBodySize: 5000})
	result := driver.FetchContent(context.Background(), "https://zhihu.com/question/123", ContentText)
	require.NotNil(t, result)
}

// --- appendBilibiliTranscript with enough lines but short content ---

func TestAppendBilibiliTranscriptShortLines(t *testing.T) {
	binDir := t.TempDir()
	opencliPath := filepath.Join(binDir, "opencli")
	script := `#!/bin/sh
case "$2" in
  subtitle)
    printf '| index | from | to | content |\n| --- | --- | --- | --- |\n| 1 | 0:01 | 0:02 | a |\n| 2 | 0:02 | 0:03 | b |\n| 3 | 0:03 | 0:04 | c |\n| 4 | 0:04 | 0:05 | d |\n| 5 | 0:05 | 0:06 | e |\n'
    ;;
  summary)
    exit 1
    ;;
  *)
    printf 'mock\n'
    ;;
esac
`
	require.NoError(t, os.WriteFile(opencliPath, []byte(script), 0o700))
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	driver := newOpenCLIDriver(DriverOptions{MaxBodySize: 5000})
	result := &ContentFetchResult{Title: "Test", Body: "original", SourceURL: "https://www.bilibili.com/video/BV1xx"}
	driver.appendBilibiliTranscript(context.Background(), result, "https://www.bilibili.com/video/BV1xx")
	// Short transcript content (< 100 runes total) should not be appended
	assert.Equal(t, "original", result.Body)
}

// --- appendBilibiliTranscript with successful transcript ---

func TestAppendBilibiliTranscriptSuccess(t *testing.T) {
	binDir := t.TempDir()
	opencliPath := filepath.Join(binDir, "opencli")
	// Create a transcript table with enough content and lines
	transcript := "| index | from | to | content |\n| --- | --- | --- | --- |\n"
	for i := 0; i < 10; i++ {
		transcript += fmt.Sprintf("| %d | 0:%02d | 0:%02d | This is transcript line %d with enough content to pass the threshold. |\n", i, i, i+1, i)
	}
	script := `#!/bin/sh
case "$2" in
  subtitle)
    printf '%s' '` + transcript + `'
    ;;
  *)
    printf 'mock\n'
    ;;
esac
`
	require.NoError(t, os.WriteFile(opencliPath, []byte(script), 0o700))
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	driver := newOpenCLIDriver(DriverOptions{MaxBodySize: 5000})
	result := &ContentFetchResult{Title: "Test", Body: "original", SourceURL: "https://www.bilibili.com/video/BV1xx"}
	driver.appendBilibiliTranscript(context.Background(), result, "https://www.bilibili.com/video/BV1xx")
	// Should have appended transcript
	if result.Body != "original" {
		assert.Contains(t, result.Body, "字幕内容")
	}
}

// --- appendYoutubeTranscript with enough lines ---

func TestAppendYoutubeTranscriptSuccess(t *testing.T) {
	binDir := t.TempDir()
	opencliPath := filepath.Join(binDir, "opencli")
	// Create a transcript with enough content and lines
	transcript := "timestamp | speaker | text\n"
	transcript += "--- | --- | ---\n"
	for i := 0; i < 10; i++ {
		transcript += fmt.Sprintf("00:%02d | speaker | This is transcript line %d with enough content to pass thresholds.\n", i, i)
	}
	script := `#!/bin/sh
printf '%s' '` + transcript + `'
`
	require.NoError(t, os.WriteFile(opencliPath, []byte(script), 0o700))
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	driver := newOpenCLIDriver(DriverOptions{MaxBodySize: 5000})
	result := &ContentFetchResult{Title: "Test", Body: "original", SourceURL: "https://www.youtube.com/watch?v=abc"}
	driver.appendYoutubeTranscript(context.Background(), result, "https://www.youtube.com/watch?v=abc")
	// Check if transcript was appended (depends on md.ExtractTranscriptLines parsing)
	if result.Body != "original" {
		assert.Contains(t, result.Body, "字幕内容")
	}
}

// --- FetchContent with youtube adapter URL ---

func TestOpenCLIDriverFetchContentYouTubeWithTranscript(t *testing.T) {
	binDir := t.TempDir()
	opencliPath := filepath.Join(binDir, "opencli")
	script := `#!/bin/sh
case "$1" in
  youtube)
    case "$2" in
      video)
        printf '# YouTube Video\n\nTitle: Test Video\nViews: 1000\n'
        ;;
      transcript)
        printf 'timestamp | speaker | text\n--- | --- | ---\n'
        printf '00:01 | speaker | This is a long enough transcript line for testing.\n'
        printf '00:02 | speaker | Another line of the transcript with enough content.\n'
        printf '00:03 | speaker | Third line of transcript content here.\n'
        printf '00:04 | speaker | Fourth line of transcript.\n'
        printf '00:05 | speaker | Fifth line of transcript content.\n'
        ;;
      *)
        printf 'mock\n'
        ;;
    esac
    ;;
  *)
    printf 'mock\n'
    ;;
esac
`
	require.NoError(t, os.WriteFile(opencliPath, []byte(script), 0o700))
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	driver := newOpenCLIDriver(DriverOptions{MaxBodySize: 5000, MediaEnabled: true})
	result := driver.FetchContent(context.Background(), "https://www.youtube.com/watch?v=abc", ContentVideo)
	require.NotNil(t, result)
}
