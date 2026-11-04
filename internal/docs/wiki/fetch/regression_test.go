package fetch

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xbpk3t/docs-alfred/internal/docs/wiki/types"
	"github.com/xbpk3t/docs-alfred/pkg/fileutil"
)

func reviewFakeCLI(t *testing.T, script string) string {
	t.Helper()
	binDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(binDir, "opencli"), []byte(script), 0o700))
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	return binDir
}

// opencli 2.0.2 emits typed failures as a JSON envelope on STDOUT and leaves
// stderr empty. cmdutil.RunSeparate discards stdout for a nonzero exit, so the
// driver never sees the message and cannot classify/retry it.
func TestReviewOpenCLI202ErrorEnvelopeRetainsMessageAndRetries(t *testing.T) {
	zeroNavRetryBackoff(t)
	binDir := t.TempDir()
	counter := filepath.Join(binDir, "count")
	script := `#!/bin/sh
n=0
[ -f "$OPENCLI_REVIEW_COUNT" ] && n=$(cat "$OPENCLI_REVIEW_COUNT")
n=$((n + 1))
printf '%s' "$n" > "$OPENCLI_REVIEW_COUNT"
if [ "$n" -le 99 ]; then
  printf '{"ok":false,"error":{"code":"TEMPFAIL","message":"Navigation rejected"}}\n'
  exit 1
fi
printf '# Title\n\nbody content\n'
`
	reviewFakeCLI(t, script)
	t.Setenv("OPENCLI_REVIEW_COUNT", counter)

	driver := newOpenCLIDriver(DriverOptions{MaxBodySize: 1000})
	result := driver.runOpenCLI(context.Background(), "web", []string{"read", "--url", "https://example.com"})

	require.NotNil(t, result)
	assert.Contains(t, result.Error, "Navigation rejected",
		"opencli 2.0.2 puts the failure envelope on stdout; the driver must preserve it")
	assert.Equal(t, "4", countInvocations(t, counter), "1 initial attempt + 3 retries")
}

// EMPTY_RESULT is how opencli 2.0.2 reports "video has no summary". The driver
// must see the code; after a stdout envelope it currently sees only the exit code.
func TestReviewBilibiliEmptyResultEnvelopeIsContentAbsence(t *testing.T) {
	script := `#!/bin/sh
printf '{"ok":false,"error":{"code":"EMPTY_RESULT","message":"summary returned no data"}}\n'
exit 66
`
	_ = reviewFakeCLI(t, script)

	driver := newOpenCLIDriver(DriverOptions{MaxBodySize: 5000})
	result := &types.ContentFetchResult{Title: "Test", Body: "original", SourceURL: "https://www.bilibili.com/video/BV1xx"}
	failure := driver.appendBilibiliSummary(context.Background(), result, "https://www.bilibili.com/video/BV1xx")

	assert.Nil(t, failure, "a missing summary is content absence, not a retryable service failure")
}

// opencli 2.0.2 wraps successful JSON rows in {"ok":true,"data":[...]}.
// The Twitter driver expects a bare array and falls back to raw-body mode.
func TestReviewTwitterJSONEnvelopeIsParsed(t *testing.T) {
	script := `#!/bin/sh
case "$1" in
  twitter)
    printf '{"ok":true,"data":[{"id":"123","author":"alice","text":"https://example.com/post","url":"https://x.com/alice/status/123"}]}\n'
    ;;
  web)
    printf '# Linked Article\n\nThis is the linked article body with enough detail to pass the quality gate. It has several sentences. Another sentence follows here.\n'
    ;;
  *)
    printf 'mock\n'
    ;;
esac
`
	reviewFakeCLI(t, script)

	driver := newOpenCLIDriver(DriverOptions{MaxBodySize: 5000})
	result := driver.FetchContent(context.Background(), "https://x.com/alice/status/123", types.ContentText)

	require.NotNil(t, result)
	require.Empty(t, result.Error, "the link-only tweet should be followed and parsed from the JSON envelope")
	assert.Contains(t, result.Body, "Linked Article")
}

// opencli 2.0.2 wraps zhihu download rows in the same JSON envelope. The driver
// expects a bare array, so it never reads the file the adapter saved.
func TestReviewZhihuDownloadJSONEnvelopeIsParsed(t *testing.T) {
	script := `#!/bin/sh
outdir=""
while [ $# -gt 0 ]; do
  if [ "$1" = "--output" ]; then outdir="$2"; shift 2; continue; fi
  shift
done
mkdir -p "$outdir"
printf '# Zhihu Article\n\nThis is the zhihu article body with enough detail to pass the quality gate. It has several sentences. Another sentence follows here.\n' > "$outdir/article.md"
printf '{"ok":true,"data":[{"title":"Zhihu Article","author":"author","status":"success","saved":"%s"}]}\n' "$outdir/article.md"
`
	reviewFakeCLI(t, script)

	driver := newOpenCLIDriver(DriverOptions{MaxBodySize: 5000})
	result := driver.FetchContent(context.Background(), "https://zhuanlan.zhihu.com/p/123", types.ContentText)

	require.NotNil(t, result)
	require.Empty(t, result.Error)
	assert.Contains(t, result.Body, "zhihu article body", "the driver must read the saved markdown file")
}

// A standard tweet is at most 280 characters. The social shell heuristic
// rejects every body under 300 runes as an anti-bot shell, regardless of
// whether it has real sentences.
func TestReviewShortTweetIsNotAnAntiBotShell(t *testing.T) {
	body := strings.Repeat("这是真实推文内容，包含完整句子。", 12) // ~180 runes
	q := assessContentQuality("推文", body, "https://x.com/user/status/1")

	assert.True(t, q.OK, "a normal short tweet must not be rejected: %s", q.Reason)
}

// zhihuVerificationReason has no length guard, and "安全验证" already contains
// "验证", so any article mentioning the phrase is rejected.
func TestReviewLongArticleMentioningZhihuVerificationPasses(t *testing.T) {
	body := strings.Repeat("这篇文章讨论安全验证机制如何影响用户体验，并给出改进建议。", 60)
	q := assessContentQuality("安全验证机制分析", body, "https://example.com/article")

	assert.True(t, q.OK, "a long article discussing verification pages must not be rejected: %s", q.Reason)
}

// browserErrorReason has no length guard either.
func TestReviewLongArticleMentioningBrowserErrorPasses(t *testing.T) {
	body := strings.Repeat("When the browser shows ERR_CONNECTION_RESET, check the proxy and retry. ", 60)
	q := assessContentQuality("Debugging ERR_CONNECTION_RESET", body, "https://example.com/article")

	assert.True(t, q.OK, "a long troubleshooting article must not be rejected: %s", q.Reason)
}

// The gate must reject a real anti-bot shell that a driver returned as a
// successful 200 response.
func TestReviewQualityGateRejectsAntiBotShellFromDriver(t *testing.T) {
	driver := &recordingDriver{
		body: "Just a moment... Performing security verification...",
	}
	f := NewFetcher(WithDriver(driver))
	result := f.FetchContent(context.Background(), "https://example.com/article", types.ContentText)

	require.NotNil(t, result)
	assert.NotEmpty(t, result.Error)
	assert.Equal(t, types.FailureReasonCloudflare, result.FailureReason)
}

// media.enabled=false is treated as a permanent empty-content failure before
// the pipeline can fall through to the ordinary drivers.
func TestReviewMediaDisabledPodcastLikeArticleIsLost(t *testing.T) {
	driver := &recordingDriver{body: strings.Repeat("A real article body with enough detail. ", 20)}
	f := NewFetcher(WithDriver(driver), WithMediaEnabled(false))

	result := f.FetchContent(context.Background(), "https://example.com/podcast/article", "")
	require.NotNil(t, result)
	assert.Empty(t, result.Error, "a podcast-looking but non-feed URL should fall through to the drivers")
}

// When a podcast-like URL is not an RSS feed, the fetcher falls through while
// keeping contentType=audio. The quality gate is skipped for non-text types,
// so an anti-bot page from the driver is accepted as article content.
func TestReviewPodcastFallbackStillAppliesQualityGate(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("this is not an rss feed"))
	}))
	t.Cleanup(server.Close)

	driver := &recordingDriver{body: "Just a moment... Performing security verification..."}
	f := NewFetcher(WithDriver(driver))
	result := f.FetchContent(context.Background(), server.URL+"/podcast/ep1", "")

	require.NotNil(t, result)
	assert.NotEmpty(t, result.Error, "the anti-bot shell must not bypass the quality gate")
	assert.Equal(t, types.FailureReasonCloudflare, result.FailureReason)
}

// The API normalizer must only rewrite the exact repo endpoint, not issue or
// file subpaths, which would silently archive the repo home instead.
func TestReviewGitHubAPISubpathIsNotCollapsedToRepoRoot(t *testing.T) {
	assert.Equal(t,
		"https://api.github.com/repos/o/r/issues/1",
		normalizeGitHubAPIURL("https://api.github.com/repos/o/r/issues/1"))
}

// A path under /pdf/ is not necessarily a PDF document.
func TestReviewPDFPathPrefixFalsePositive(t *testing.T) {
	assert.False(t, isPDFURL("https://example.com/pdf/viewer.html"))
}

// The driver must set the opencli browser command timeout for its subprocesses.
func TestReviewOpenCLIBrowserTimeoutEnv(t *testing.T) {
	t.Setenv("OPENCLI_BROWSER_COMMAND_TIMEOUT", "")
	newOpenCLIDriver(DriverOptions{MaxBodySize: 1000})
	assert.Equal(t, "180", os.Getenv("OPENCLI_BROWSER_COMMAND_TIMEOUT"))
}

// opencli shares one browser tab. The driver must cap concurrent subprocesses
// at browserConcurrency, including release on every return path.
func TestReviewBrowserSemaphoreCapsConcurrentOpenCLI(t *testing.T) {
	binDir := t.TempDir()
	trace := filepath.Join(binDir, "trace")
	script := `#!/bin/sh
s=$(python3 -c 'import time;print(time.time())')
sleep 0.4
e=$(python3 -c 'import time;print(time.time())')
echo "$s $e" >> "$OPENCLI_REVIEW_TRACE"
printf '# ok\n\nbody body body\n'
`
	require.NoError(t, os.WriteFile(filepath.Join(binDir, "opencli"), []byte(script), 0o700))
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("OPENCLI_REVIEW_TRACE", trace)

	driver := newOpenCLIDriver(DriverOptions{MaxBodySize: 1000, BrowserConcurrency: 2})
	const workers = 8
	var wg sync.WaitGroup
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _, err := driver.execOpenCLI(context.Background(), []string{"web", "read", "--url", "https://example.com"}, nil)
			assert.NoError(t, err)
		}()
	}
	wg.Wait()

	data, err := os.ReadFile(trace)
	require.NoError(t, err)
	var intervals [][2]float64
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		fields := strings.Fields(line)
		if len(fields) != 2 {
			continue
		}
		start, err1 := strconv.ParseFloat(fields[0], 64)
		end, err2 := strconv.ParseFloat(fields[1], 64)
		if err1 == nil && err2 == nil {
			intervals = append(intervals, [2]float64{start, end})
		}
	}
	require.Len(t, intervals, workers)

	maxOverlap := 0
	for i, a := range intervals {
		overlap := 0
		for j, b := range intervals {
			if i == j {
				continue
			}
			if b[0] < a[1] && b[1] > a[0] {
				overlap++
			}
		}
		if overlap+1 > maxOverlap {
			maxOverlap = overlap + 1
		}
	}
	assert.LessOrEqual(t, maxOverlap, 2, "browserConcurrency=2 must cap concurrent opencli processes; intervals=%v", intervals)
}

// A short subtitle body currently returns before the summary fallback, so a
// video with unusable subtitles but an available official summary stays
// metadata-only.
func TestReviewShortSubtitleStillFallsBackToSummary(t *testing.T) {
	script := `#!/bin/sh
case "$2" in
  subtitle)
    printf 'short\n'
    ;;
  summary)
    printf 'B站 official AI summary that is long enough to pass the minimum threshold and contain real content.\n'
    ;;
  *)
    printf 'mock\n'
    ;;
esac
`
	reviewFakeCLI(t, script)
	driver := newOpenCLIDriver(DriverOptions{MaxBodySize: 5000})
	result := &types.ContentFetchResult{Title: "Test", Body: "original", SourceURL: "https://www.bilibili.com/video/BV1xx"}
	failure := driver.appendBilibiliTranscript(context.Background(), result, "https://www.bilibili.com/video/BV1xx")

	assert.Nil(t, failure)
	assert.Contains(t, result.Body, "AI 总结", "unusable subtitles should still try the official summary")
}

// A weixin status that is not an anti-bot wall must not be retried forever.
func TestReviewWeixinNonRetryableStatusIsPermanent(t *testing.T) {
	script := `#!/bin/sh
printf -- "- title: Error\n  status: failed — no title\n  saved: '-'\n"
`
	reviewFakeCLI(t, script)
	driver := newOpenCLIDriver(DriverOptions{MaxBodySize: 5000})
	result := driver.FetchContent(context.Background(), "https://mp.weixin.qq.com/s/abc", types.ContentText)

	require.NotNil(t, result)
	require.NotEmpty(t, result.Error)
	assert.NotEqual(t, types.FailureReasonAntiBot, result.FailureReason,
		"failed — no title is a permanent content failure, not an anti-bot wall")
}

// Adapter fallback must trigger on a quality-gate failure, not only on an
// explicit driver error. A 200 anti-bot page is a failed primary extraction.
func TestReviewAdapterFallbackTriggersOnQualityFailure(t *testing.T) {
	primary := &recordingDriver{body: "Just a moment... Performing security verification..."}
	fallback := &recordingDriver{body: strings.Repeat("fallback article body with real content. ", 30)}
	f := NewFetcher(WithDriver(primary), WithFallbackDriver(fallback))

	result := f.FetchContent(context.Background(), "https://zhuanlan.zhihu.com/p/123", types.ContentText)
	require.NotNil(t, result)
	assert.Empty(t, result.Error, "the fallback should rescue a primary quality-gate failure")
	assert.Contains(t, result.Body, "fallback article body")
}

// pathWithin is lexical; a symlink inside the temp dir can point outside it.
func TestReviewPathWithinRejectsSymlinkEscape(t *testing.T) {
	dir := t.TempDir()
	outside := filepath.Join(t.TempDir(), "secret.md")
	require.NoError(t, os.WriteFile(outside, []byte("secret"), 0o600))
	link := filepath.Join(dir, "link.md")
	require.NoError(t, os.Symlink(outside, link))

	assert.False(t, fileutil.PathWithin(dir, link), "a symlink escaping the temp dir must be rejected")
}

// The saved path comes from an external process; a value outside the temp dir
// must never be read.
func TestReviewWeixinSavedPathEscapeRejected(t *testing.T) {
	script := `#!/bin/sh
printf -- "- title: Error\n  status: success\n  saved: /etc/passwd\n"
`
	reviewFakeCLI(t, script)
	driver := newOpenCLIDriver(DriverOptions{MaxBodySize: 5000})
	result := driver.FetchContent(context.Background(), "https://mp.weixin.qq.com/s/abc", types.ContentText)

	require.NotNil(t, result)
	assert.NotEmpty(t, result.Error, "a saved path outside the temp dir must be rejected")
}

// Each marker must independently classify its error; "cloneNode" is currently
// masked by the broader "Cannot read properties of null" marker.
func TestReviewTransientMarkersIndependently(t *testing.T) {
	assert.True(t, isTransientOpenCLIError("TypeError: cloneNode is not a function"))
	assert.True(t, isTransientOpenCLIError("Page not found: abc — stale page identity"))
	assert.True(t, isTransientOpenCLIError("Browser connection dropped after the navigate command"))
	assert.True(t, isTransientOpenCLIError("TypeError: Failed to fetch"))
}

// Cancellation while workers are queued on the browser semaphore must not
// deadlock or leak slots.
func TestReviewBrowserSemaphoreContextCancelNoDeadlock(t *testing.T) {
	script := `#!/bin/sh
exec sleep 2
`
	reviewFakeCLI(t, script)
	driver := newOpenCLIDriver(DriverOptions{MaxBodySize: 1000, BrowserConcurrency: 2})
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()

	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _, _ = driver.execOpenCLI(ctx, []string{"web", "read", "--url", "https://example.com"}, nil)
		}()
	}
	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("workers did not exit after context cancellation")
	}
	assert.Empty(t, driver.browserSem, "all browser slots must be released")
}

// Web-read routes handled through the adapter switch (bilibili opus/read,
// reddit) should use the same --wait as the generic web path.
func TestReviewAdapterWebReadGetsWait(t *testing.T) {
	binDir := t.TempDir()
	argsFile := filepath.Join(binDir, "args")
	script := `#!/bin/sh
printf '%s\n' "$@" > "$OPENCLI_REVIEW_ARGS"
printf '# Opus\n\nA real article body with enough detail. It has several sentences. Another sentence follows here.\n'
`
	require.NoError(t, os.WriteFile(filepath.Join(binDir, "opencli"), []byte(script), 0o700))
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("OPENCLI_REVIEW_ARGS", argsFile)

	driver := newOpenCLIDriver(DriverOptions{MaxBodySize: 5000})
	result := driver.FetchContent(context.Background(), "https://www.bilibili.com/opus/123", types.ContentText)
	require.NotNil(t, result)
	require.Empty(t, result.Error)

	args, err := os.ReadFile(argsFile)
	require.NoError(t, err)
	assert.Contains(t, string(args), "--wait", "adapter web read should wait for client-rendered content")
}

// Long X threads about programming routinely mention "javascript" / "log in".
// socialShellPatterns is applied regardless of length once >=300 runes, so a
// real technical thread is rejected as a login shell.
func TestReviewLongTweetThreadMentioningJavaScriptPasses(t *testing.T) {
	body := strings.Repeat("JavaScript is a programming language with many uses. It powers the web. Here is another sentence. ", 20)
	q := assessContentQuality("A thread about JavaScript", body, "https://x.com/user/status/1")

	assert.True(t, q.OK, "a long technical thread must not be rejected as a login shell: %s", q.Reason)
}

// The curated legacy matcher still contains phrases that legitimate short
// articles discuss ("404 not found", "page not found", "please log in").
func TestReviewShortArticleAbout404Passes(t *testing.T) {
	body := strings.Repeat("A 404 Not Found page means the requested resource is missing. Here is how to diagnose it. ", 10)
	q := assessContentQuality("Fixing 404 Not Found errors", body, "https://example.com/article")

	assert.True(t, q.OK, "a short article about HTTP errors must not be rejected: %s", q.Reason)
}

// HTTP-first is only as safe as the quality gate. A short navigation/landing
// page (>120 runes, no error keyword) passes the gate and suppresses the
// primary driver that would have rendered the article.
func TestReviewHTTPFirstRejectsThinNavigationPage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(`<html><head><title>Example Site</title></head><body>
<nav><a href="/">Home</a> <a href="/about">About</a> <a href="/contact">Contact</a></nav>
<p>Welcome to our site. Choose a section from the navigation above to continue. We publish updates every week. Browse the archive for older posts.</p>
</body></html>`))
	}))
	t.Cleanup(server.Close)

	primary := &recordingDriver{body: strings.Repeat("This is the real rendered article body. ", 30)}
	fallback, err := NewDriver("http-readability", DriverOptions{MaxBodySize: 5000})
	require.NoError(t, err)
	f := NewFetcher(WithDriver(primary), WithFallbackDriver(fallback))

	result := f.FetchContent(context.Background(), server.URL+"/article", types.ContentText)
	require.NotNil(t, result)
	assert.NotEmpty(t, primary.url, "a thin fallback page should not suppress the primary renderer")
}

// zhihu download's permanent extraction statuses are currently mapped to
// retryable partial_extract, same as weixin's.
func TestReviewZhihuNoTitleStatusIsPermanent(t *testing.T) {
	script := `#!/bin/sh
printf '[{"title":"Error","status":"failed — no title","saved":"-"}]\n'
`
	reviewFakeCLI(t, script)
	driver := newOpenCLIDriver(DriverOptions{MaxBodySize: 5000})
	result := driver.FetchContent(context.Background(), "https://zhuanlan.zhihu.com/p/123", types.ContentText)

	require.NotNil(t, result)
	require.NotEmpty(t, result.Error)
	assert.NotEqual(t, types.FailureReasonPartialExtract, result.FailureReason,
		"failed — no title is permanent content absence, not a retryable partial extract")
}

// execOpenCLI retries with the same --output dir. A failed first attempt can
// leave a file behind; a second attempt that reports it as saved makes the
// driver archive that stale partial content.
func TestReviewWeixinRetryDoesNotReadStalePartialFile(t *testing.T) {
	binDir := t.TempDir()
	counter := filepath.Join(binDir, "count")
	script := `#!/bin/sh
n=0
[ -f "$OPENCLI_REVIEW_COUNT" ] && n=$(cat "$OPENCLI_REVIEW_COUNT")
n=$((n + 1))
printf '%s' "$n" > "$OPENCLI_REVIEW_COUNT"
outdir=""
while [ $# -gt 0 ]; do
  if [ "$1" = "--output" ]; then outdir="$2"; shift 2; continue; fi
  shift
done
if [ "$n" -eq 1 ]; then
  printf 'STALE PARTIAL CONTENT %s\n' "that should never be archived because the first attempt failed and its file is incomplete but long enough to pass the quality gate" > "$outdir/partial.md"
  printf 'Navigation rejected\n' >&2
  exit 1
fi
printf -- "- title: Stale\n  status: success\n  saved: %s\n" "$outdir/partial.md"
`
	require.NoError(t, os.WriteFile(filepath.Join(binDir, "opencli"), []byte(script), 0o700))
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("OPENCLI_REVIEW_COUNT", counter)

	driver := newOpenCLIDriver(DriverOptions{MaxBodySize: 5000})
	result := driver.FetchContent(context.Background(), "https://mp.weixin.qq.com/s/abc", types.ContentText)
	require.NotNil(t, result)
	assert.NotContains(t, result.Body, "STALE PARTIAL CONTENT",
		"content written by a failed attempt must not be treated as this attempt's output")
}

// ensureFetchedQuality returns early for non-text content. A video URL whose
// driver returns a long anti-bot shell therefore bypasses the gate entirely.
func TestReviewVideoAntiBotShellIsRejected(t *testing.T) {
	driver := &recordingDriver{
		body: strings.Repeat("Just a moment... Performing security verification... ", 20),
	}
	f := NewFetcher(WithDriver(driver))
	result := f.FetchContent(context.Background(), "https://www.youtube.com/watch?v=abc", "")
	require.NotNil(t, result)
	assert.NotEmpty(t, result.Error, "a video anti-bot shell must not bypass the quality gate")
}
