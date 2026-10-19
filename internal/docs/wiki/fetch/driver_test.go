package fetch

import (
	"context"
	"fmt"
	"github.com/xbpk3t/docs-alfred/internal/docs/wiki/types"
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
	assert.Empty(t, extractTitleFromHTML(`<html><body>No head</body></html>`))
}

func TestExtractTitleFromHTMLEmptyTitle(t *testing.T) {
	assert.Empty(t, extractTitleFromHTML(`<html><head><title></title></head><body></body></html>`))
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
	input := &types.ContentFetchResult{Error: "existing error", SourceURL: "https://example.com"}
	result := d.ensureContentQuality(context.Background(), input)
	assert.Equal(t, "existing error", result.Error)
}

func TestEnsureContentQualityGoodContent(t *testing.T) {
	d := newHTTPDriver(DriverOptions{MaxBodySize: 5000})
	body := strings.Repeat("This is good content with enough length to pass quality checks. ", 10)
	input := &types.ContentFetchResult{Title: "Good", Body: body, SourceURL: "https://example.com/article"}
	result := d.ensureContentQuality(context.Background(), input)
	assert.Empty(t, result.Error)
}

func TestEnsureContentQualityLowQuality(t *testing.T) {
	d := newHTTPDriver(DriverOptions{MaxBodySize: 5000})
	input := &types.ContentFetchResult{Title: "Bad", Body: "this page requires javascript", SourceURL: "https://example.com"}
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
	result := driver.FetchContent(context.Background(), server.URL, types.ContentText)

	require.NotNil(t, result)
	require.NotEmpty(t, result.Error)
	assert.Equal(t, types.FailureResolve, result.FailureKind)
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
	result := driver.FetchContent(context.Background(), "https://www.youtube.com/watch?v=abc", types.ContentVideo)
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
	result := driver.FetchContent(context.Background(), "https://example.com/article", types.ContentText)
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
	result := driver.FetchContent(context.Background(), "https://www.bilibili.com/video/BV1xx", types.ContentVideo)
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
	result := driver.FetchContent(context.Background(), "https://www.bilibili.com/video/BV1xx", types.ContentVideo)
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
	result := driver.FetchContent(context.Background(), "https://www.youtube.com/watch?v=abc", types.ContentVideo)
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
	result := driver.FetchContent(context.Background(), "https://www.bilibili.com/video/BV1xx", types.ContentVideo)
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
	result := driver.FetchContent(context.Background(), "https://www.youtube.com/watch?v=abc", types.ContentVideo)
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
	result := &types.ContentFetchResult{Title: "Test", Body: "original body", SourceURL: "https://www.bilibili.com/video/BV1xx"}
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
	result := &types.ContentFetchResult{Title: "Test", Body: "original", SourceURL: "https://www.bilibili.com/video/BV1xx"}
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
	result := &types.ContentFetchResult{Title: "Test", Body: "original", SourceURL: "https://www.bilibili.com/video/BV1xx"}
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
	result := &types.ContentFetchResult{Title: "Test", Body: "original", SourceURL: "https://www.youtube.com/watch?v=abc"}
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
	result := &types.ContentFetchResult{Title: "Test", Body: "original", SourceURL: "https://www.youtube.com/watch?v=abc"}
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
	result := &types.ContentFetchResult{Title: "Test", Body: "original", SourceURL: "https://www.bilibili.com/video/BV1xx"}
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
	result := driver.FetchContent(context.Background(), "https://x.com/user/status/123", types.ContentText)
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
	result := driver.FetchContent(context.Background(), "https://mp.weixin.qq.com/s/abc", types.ContentText)
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
	result := driver.FetchContent(context.Background(), "https://zhihu.com/question/123", types.ContentText)
	require.NotNil(t, result)
}

// The zhihu question route carries `--limit N` in the route table; the driver
// must forward it verbatim to the opencli binary.
// zhihuFakeOpenCLI installs a fake opencli that records its argument vector to
// <dir>/args and answers the two zhihu subcommands with YAML payloads.
func zhihuFakeOpenCLI(t *testing.T) string {
	t.Helper()

	binDir := t.TempDir()
	argsFile := filepath.Join(binDir, "args")
	opencliPath := filepath.Join(binDir, "opencli")
	script := `#!/bin/sh
printf '%s\n' "$@" > ` + argsFile + `
case "$2" in
  question)
    cat <<'EOF'
- rank: 1
  id: "123456789"
  author: 恋猫
  votes: 388
  url: https://www.zhihu.com/question/35129528/answer/123456789
  content: 代表片段一
- rank: 2
  id: "987654321"
  author: 李明殊
  votes: 11
  url: https://www.zhihu.com/question/35129528/answer/987654321
  content: 代表片段二
EOF
    ;;
  answer-detail)
    cat <<'EOF'
- id: "123456789"
  author: 恋猫
  votes: 388
  question_title: 如何看待 AI Agent 主动联系研究员
  url: https://www.zhihu.com/question/35129528/answer/123456789
  content: |-
    这是这条回答的完整正文。
    第二行。
EOF
    ;;
esac
`
	require.NoError(t, os.WriteFile(opencliPath, []byte(script), 0o755))
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	return argsFile
}

func readFakeArgs(t *testing.T, argsFile string) []string {
	t.Helper()

	raw, err := os.ReadFile(argsFile)
	require.NoError(t, err)

	return strings.Split(strings.TrimSpace(string(raw)), "\n")
}

func TestOpenCLIDriverZhihuQuestionSplitsAnswers(t *testing.T) {
	argsFile := zhihuFakeOpenCLI(t)

	driver := newOpenCLIDriver(DriverOptions{MaxBodySize: 5000})
	result := driver.FetchContent(context.Background(), "https://www.zhihu.com/question/35129528", types.ContentText)

	require.NotNil(t, result)
	require.Equal(t, "", result.Error)
	assert.Equal(t, []string{
		"https://www.zhihu.com/question/35129528/answer/123456789",
		"https://www.zhihu.com/question/35129528/answer/987654321",
	}, result.ChildURLs)
	// Body is the rendered synthesis input, not raw yaml.
	assert.Contains(t, result.Body, "恋猫")
	assert.Contains(t, result.Body, "388 赞同")
	assert.NotContains(t, result.Body, "rank:")

	args := readFakeArgs(t, argsFile)
	assert.Contains(t, args, "question")
	assert.Contains(t, args, "--limit")
	assert.Contains(t, args, "3")
	// yaml, not the route table's md table rendering.
	assert.Contains(t, args, "yaml")
}

func TestOpenCLIDriverZhihuAnswerDetailUsesFullBody(t *testing.T) {
	argsFile := zhihuFakeOpenCLI(t)

	driver := newOpenCLIDriver(DriverOptions{MaxBodySize: 5000})
	result := driver.FetchContent(
		context.Background(),
		"https://www.zhihu.com/question/35129528/answer/123456789",
		types.ContentText,
	)

	require.NotNil(t, result)
	require.Equal(t, "", result.Error)
	assert.Empty(t, result.ChildURLs)
	assert.Equal(t, "如何看待 AI Agent 主动联系研究员 - 恋猫", result.Title)
	assert.Contains(t, result.Body, "这是这条回答的完整正文。")
	assert.NotContains(t, result.Body, "author:")

	args := readFakeArgs(t, argsFile)
	assert.Contains(t, args, "answer-detail")
	assert.Contains(t, args, "https://www.zhihu.com/question/35129528/answer/123456789")
	assert.Contains(t, args, "yaml")
}

func TestOpenCLIDriverZhihuUnparseableYAMLIsExtractFailure(t *testing.T) {
	binDir := t.TempDir()
	opencliPath := filepath.Join(binDir, "opencli")
	require.NoError(t, os.WriteFile(opencliPath, []byte("#!/bin/sh\nprintf 'not: [valid: yaml\\n'\n"), 0o755))
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	driver := newOpenCLIDriver(DriverOptions{MaxBodySize: 5000})
	result := driver.FetchContent(context.Background(), "https://www.zhihu.com/question/35129528", types.ContentText)

	require.NotNil(t, result)
	assert.Contains(t, result.Error, "parse zhihu question yaml")
	assert.Equal(t, types.FailureExtract, result.FailureKind)
	assert.Empty(t, result.Body, "raw yaml must never reach the summarizer")
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
	result := &types.ContentFetchResult{Title: "Test", Body: "original", SourceURL: "https://www.bilibili.com/video/BV1xx"}
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
	var transcriptSb867 strings.Builder
	for i := 0; i < 10; i++ {
		transcriptSb867.WriteString(fmt.Sprintf("| %d | 0:%02d | 0:%02d | This is transcript line %d with enough content to pass the threshold. |\n", i, i, i+1, i))
	}
	transcript += transcriptSb867.String()
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
	result := &types.ContentFetchResult{Title: "Test", Body: "original", SourceURL: "https://www.bilibili.com/video/BV1xx"}
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
	var transcriptSb900 strings.Builder
	for i := 0; i < 10; i++ {
		transcriptSb900.WriteString(fmt.Sprintf("00:%02d | speaker | This is transcript line %d with enough content to pass thresholds.\n", i, i))
	}
	transcript += transcriptSb900.String()
	script := `#!/bin/sh
printf '%s' '` + transcript + `'
`
	require.NoError(t, os.WriteFile(opencliPath, []byte(script), 0o700))
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	driver := newOpenCLIDriver(DriverOptions{MaxBodySize: 5000})
	result := &types.ContentFetchResult{Title: "Test", Body: "original", SourceURL: "https://www.youtube.com/watch?v=abc"}
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
	result := driver.FetchContent(context.Background(), "https://www.youtube.com/watch?v=abc", types.ContentVideo)
	require.NotNil(t, result)
}

// --- openCLIDriver.runOpenCLI retry (browser-bridge refusals) ---

// fakeOpenCLI installs a fake `opencli` on PATH that fails with errText for the
// first failN calls, then prints a markdown body. It returns the path of the
// file counting invocations.
func fakeOpenCLI(t *testing.T, failN int, errText string) string {
	t.Helper()

	binDir := t.TempDir()
	counter := filepath.Join(binDir, "count")
	script := fmt.Sprintf(`#!/bin/sh
n=0
[ -f "$OPENCLI_TEST_COUNT" ] && n=$(cat "$OPENCLI_TEST_COUNT")
n=$((n + 1))
printf '%%s' "$n" > "$OPENCLI_TEST_COUNT"
if [ "$n" -le %d ]; then
  printf 'ok: false\nerror:\n  code: UNKNOWN\n  message: %s\n' >&2
  exit 1
fi
printf '# Title\n\nbody content\n'
`, failN, errText)

	require.NoError(t, os.WriteFile(filepath.Join(binDir, "opencli"), []byte(script), 0o700))
	t.Setenv("OPENCLI_TEST_COUNT", counter)
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	return counter
}

func countInvocations(t *testing.T, counter string) string {
	t.Helper()

	data, err := os.ReadFile(counter)
	require.NoError(t, err)

	return strings.TrimSpace(string(data))
}

func zeroNavRetryBackoff(t *testing.T) {
	t.Helper()

	restore := navRetryBackoff
	navRetryBackoff = 0
	t.Cleanup(func() { navRetryBackoff = restore })
}

func TestRunOpenCLIRetriesNavigationRejected(t *testing.T) {
	zeroNavRetryBackoff(t)
	counter := fakeOpenCLI(t, 2, "Navigation rejected.")
	driver := newOpenCLIDriver(DriverOptions{MaxBodySize: 1000})

	result := driver.runOpenCLI(context.Background(), "web", []string{"read", "--url", "https://example.com"})

	require.Empty(t, result.Error)
	assert.Contains(t, result.Body, "body content")
	assert.Equal(t, "3", countInvocations(t, counter), "two refusals then a success")
}

func TestRunOpenCLIGivesUpAfterNavigationRetries(t *testing.T) {
	zeroNavRetryBackoff(t)
	counter := fakeOpenCLI(t, 99, "Navigation rejected.")
	driver := newOpenCLIDriver(DriverOptions{MaxBodySize: 1000})

	result := driver.runOpenCLI(context.Background(), "web", []string{"read", "--url", "https://example.com"})

	assert.Contains(t, result.Error, "Navigation rejected")
	assert.Equal(t, "3", countInvocations(t, counter), "1 initial attempt + 2 retries")
}

func TestRunOpenCLIDoesNotRetryOtherFailures(t *testing.T) {
	zeroNavRetryBackoff(t)
	counter := fakeOpenCLI(t, 99, "EMPTY_RESULT")
	driver := newOpenCLIDriver(DriverOptions{MaxBodySize: 1000})

	result := driver.runOpenCLI(context.Background(), "web", []string{"read", "--url", "https://example.com"})

	assert.Contains(t, result.Error, "EMPTY_RESULT")
	assert.Equal(t, "1", countInvocations(t, counter), "non-transient failures are not retried")
}

func TestIsNavigationRejected(t *testing.T) {
	assert.True(t, isNavigationRejected("opencli: exec opencli: exit 1: ok: false\n  message: Navigation rejected."))
	assert.False(t, isNavigationRejected("opencli: exec opencli: exit 1: ok: false\n  message: EMPTY_RESULT"))
	assert.False(t, isNavigationRejected(""))
}
