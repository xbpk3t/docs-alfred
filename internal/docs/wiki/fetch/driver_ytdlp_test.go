package fetch

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xbpk3t/docs-alfred/internal/docs/wiki/types"
)

// writeFakeBin writes an executable fake CLI into dir.
func writeFakeBin(t *testing.T, dir, name, script string) {
	t.Helper()
	path := filepath.Join(dir, name)
	require.NoError(t, os.WriteFile(path, []byte(script), 0o700))
}

// prependPATH puts dir first so the fake binaries win over real ones.
func prependPATH(t *testing.T, dir string) {
	t.Helper()
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

const fakeYTDLPSuccessScript = `#!/bin/sh
if [ -n "$YTDLP_FAKE_LOG" ]; then printf '%s\n' "$*" >> "$YTDLP_FAKE_LOG"; fi
case " $* " in
  *" --dump-json "*)
    printf '%s\n' '{"title":"Test Video","subtitles":{"en":[{"ext":"vtt"}]},"automatic_captions":{}}'
    ;;
  *" --write-subs "*|*" --write-auto-subs "*)
    dir=""
    prev=""
    for a in "$@"; do
      if [ "$prev" = "-o" ]; then dir=$(dirname "$a"); fi
      prev="$a"
    done
    printf 'WEBVTT\n\n00:00:01.000 --> 00:00:03.000\nHello from the first subtitle line.\n\n00:00:04.000 --> 00:00:06.000\nAnd the second subtitle line.\n' > "$dir/vid.en.vtt"
    ;;
esac
`

func TestNewDriverReturnsYTDLPAndHybrid(t *testing.T) {
	ytdlp, err := NewDriver("ytdlp", DriverOptions{MaxBodySize: 2000, MediaEnabled: true})
	require.NoError(t, err)
	assert.Equal(t, "ytdlp", ytdlp.Name())

	hybrid, err := NewDriver("hybrid", DriverOptions{MaxBodySize: 2000, MediaEnabled: true})
	require.NoError(t, err)
	assert.Equal(t, "hybrid", hybrid.Name())
}

func TestYTDLPDriverSubtitleSuccess(t *testing.T) {
	binDir := t.TempDir()
	writeFakeBin(t, binDir, "yt-dlp", fakeYTDLPSuccessScript)
	prependPATH(t, binDir)

	driver := newYTDLPDriver(DriverOptions{MaxBodySize: 20000, MediaEnabled: true})
	result := driver.FetchContent(context.Background(), "https://www.youtube.com/watch?v=abc", types.ContentVideo)

	require.NotNil(t, result)
	require.Empty(t, result.Error)
	assert.Equal(t, "Test Video", result.Title)
	assert.True(t, result.HasTranscript)
	assert.Contains(t, result.Body, types.TranscriptSectionMarker)
	assert.Contains(t, result.Body, "Hello from the first subtitle line")
	assert.Equal(t, "https://www.youtube.com/watch?v=abc", result.SourceURL)
}

// yt-dlp --dump-json emits one very long JSON line. The default go-cmd line
// buffer (64 KiB) overflows and leaves stdout empty; the driver must use a
// large-line runner.
func TestYTDLPDriverLargeMetadataLine(t *testing.T) {
	binDir := t.TempDir()
	metaPath := filepath.Join(t.TempDir(), "meta.json")
	longTitle := strings.Repeat("x", 200*1024)
	require.NoError(t, os.WriteFile(
		metaPath,
		[]byte(`{"title":"`+longTitle+`","subtitles":{"en":[{"ext":"vtt"}]},"automatic_captions":{}}`),
		0o600,
	))
	writeFakeBin(t, binDir, "yt-dlp", `#!/bin/sh
case " $* " in
  *" --dump-json "*) cat "$META_PATH" ;;
  *" --write-subs "*|*" --write-auto-subs "*)
    dir=""; prev=""
    for a in "$@"; do if [ "$prev" = "-o" ]; then dir=$(dirname "$a"); fi; prev="$a"; done
    printf 'WEBVTT\n\n00:00:01.000 --> 00:00:03.000\nLarge metadata subtitle.\n' > "$dir/vid.en.vtt"
    ;;
esac
`)
	prependPATH(t, binDir)
	t.Setenv("META_PATH", metaPath)

	driver := newYTDLPDriver(DriverOptions{MaxBodySize: 20000, MediaEnabled: true})
	result := driver.FetchContent(context.Background(), "https://www.youtube.com/watch?v=abc", types.ContentVideo)

	require.NotNil(t, result)
	require.Empty(t, result.Error)
	assert.True(t, result.HasTranscript)
	assert.Contains(t, result.Body, "Large metadata subtitle")
}

func TestYTDLPDriverCommandFailureIsRetryable(t *testing.T) {
	binDir := t.TempDir()
	writeFakeBin(t, binDir, "yt-dlp", `#!/bin/sh
printf 'ERROR: [youtube] abc: Sign in to confirm you are not a bot. Use --cookies-from-browser\n' >&2
exit 1
`)
	prependPATH(t, binDir)

	driver := newYTDLPDriver(DriverOptions{MaxBodySize: 20000, MediaEnabled: true})
	result := driver.FetchContent(context.Background(), "https://www.youtube.com/watch?v=abc", types.ContentVideo)

	require.NotNil(t, result)
	require.NotEmpty(t, result.Error)
	assert.Equal(t, types.FailureReasonPartialExtract, result.FailureReason)
	assert.Equal(t, types.FailureExtract, result.FailureKind)
}

func TestYTDLPDriverNoSubtitlesIsTooShort(t *testing.T) {
	binDir := t.TempDir()
	writeFakeBin(t, binDir, "yt-dlp", `#!/bin/sh
printf '%s\n' '{"title":"Test Video","subtitles":{},"automatic_captions":{}}'
`)
	prependPATH(t, binDir)

	driver := newYTDLPDriver(DriverOptions{MaxBodySize: 20000, MediaEnabled: true})
	result := driver.FetchContent(context.Background(), "https://www.youtube.com/watch?v=abc", types.ContentVideo)

	require.NotNil(t, result)
	require.NotEmpty(t, result.Error)
	assert.Equal(t, types.FailureReasonTooShort, result.FailureReason)
	assert.False(t, result.HasTranscript)
}

func TestYTDLPDriverUnsupportedNonVideoURL(t *testing.T) {
	driver := newYTDLPDriver(DriverOptions{MaxBodySize: 20000, MediaEnabled: true})
	result := driver.FetchContent(context.Background(), "https://example.com/article", types.ContentText)

	require.NotNil(t, result)
	assert.NotEmpty(t, result.Error)
	assert.Equal(t, types.FailureReasonEmpty, result.FailureReason)
}

func TestYTDLPDriverBilibiliUsesAutoChineseSubtitle(t *testing.T) {
	binDir := t.TempDir()
	writeFakeBin(t, binDir, "yt-dlp", `#!/bin/sh
if [ -n "$YTDLP_FAKE_LOG" ]; then printf '%s\n' "$*" >> "$YTDLP_FAKE_LOG"; fi
case " $* " in
  *" --dump-json "*)
    printf '%s\n' '{"title":"中文视频","subtitles":{},"automatic_captions":{"ai-zh":[{"ext":"srt"}]}}'
    ;;
  *" --write-auto-subs "*)
    dir=""
    prev=""
    for a in "$@"; do
      if [ "$prev" = "-o" ]; then dir=$(dirname "$a"); fi
      prev="$a"
    done
    printf '1\n00:00:01,000 --> 00:00:02,000\n中文字幕内容\n' > "$dir/vid.ai-zh.srt"
    ;;
esac
`)
	prependPATH(t, binDir)
	logPath := filepath.Join(t.TempDir(), "ytdlp.log")
	t.Setenv("YTDLP_FAKE_LOG", logPath)

	driver := newYTDLPDriver(DriverOptions{MaxBodySize: 20000, MediaEnabled: true})
	result := driver.FetchContent(context.Background(), "https://www.bilibili.com/video/BV1xx", types.ContentVideo)

	require.NotNil(t, result)
	require.Empty(t, result.Error)
	assert.True(t, result.HasTranscript)
	assert.Contains(t, result.Body, "中文字幕内容")

	logData, err := os.ReadFile(logPath)
	require.NoError(t, err)
	log := string(logData)
	assert.Contains(t, log, "--sub-langs ai-zh")
	assert.Contains(t, log, "--write-auto-subs")
}

// Bilibili's --dump-json can return an empty subtitles map even when
// --write-subs can fetch the ai-zh track. The driver must try the language
// filter instead of declaring "no captions".
func TestYTDLPDriverBilibiliEmptyMetadataFallsBackToSubtitleFiles(t *testing.T) {
	binDir := t.TempDir()
	writeFakeBin(t, binDir, "yt-dlp", `#!/bin/sh
if [ -n "$YTDLP_FAKE_LOG" ]; then printf '%s\n' "$*" >> "$YTDLP_FAKE_LOG"; fi
case " $* " in
  *" --dump-json "*)
    printf '%s\n' '{"title":"中文视频","subtitles":{},"automatic_captions":{}}'
    ;;
  *" --write-subs "*)
    dir=""; prev=""
    for a in "$@"; do if [ "$prev" = "-o" ]; then dir=$(dirname "$a"); fi; prev="$a"; done
    printf '1\n00:00:01,000 --> 00:00:02,000\n中文字幕回退内容\n' > "$dir/vid.ai-zh.srt"
    ;;
esac
`)
	prependPATH(t, binDir)
	logPath := filepath.Join(t.TempDir(), "ytdlp.log")
	t.Setenv("YTDLP_FAKE_LOG", logPath)

	driver := newYTDLPDriver(DriverOptions{MaxBodySize: 20000, MediaEnabled: true})
	result := driver.FetchContent(context.Background(), "https://www.bilibili.com/video/BV1xx", types.ContentVideo)

	require.NotNil(t, result)
	require.Empty(t, result.Error)
	assert.True(t, result.HasTranscript)
	assert.Contains(t, result.Body, "中文字幕回退内容")
	log := readFileOrEmpty(t, logPath)
	assert.Contains(t, log, "--write-subs")
	assert.Contains(t, log, "ai-zh")
}

func TestHybridDriverRoutesVideoToYTDLPAndTextToOpenCLI(t *testing.T) {
	binDir := t.TempDir()
	writeFakeBin(t, binDir, "yt-dlp", fakeYTDLPSuccessScript)
	writeFakeBin(t, binDir, "opencli", `#!/bin/sh
if [ -n "$OPENCLI_FAKE_LOG" ]; then printf '%s\n' "$*" >> "$OPENCLI_FAKE_LOG"; fi
printf '# Web Article\n\nSome content from opencli.\n'
`)
	prependPATH(t, binDir)
	logDir := t.TempDir()
	ytdlpLog := filepath.Join(logDir, "ytdlp.log")
	opencliLog := filepath.Join(logDir, "opencli.log")
	t.Setenv("YTDLP_FAKE_LOG", ytdlpLog)
	t.Setenv("OPENCLI_FAKE_LOG", opencliLog)

	driver := newHybridDriver(DriverOptions{MaxBodySize: 20000, MediaEnabled: true})

	video := driver.FetchContent(context.Background(), "https://www.youtube.com/watch?v=abc", types.ContentVideo)
	require.NotNil(t, video)
	require.Empty(t, video.Error)
	assert.True(t, video.HasTranscript)

	text := driver.FetchContent(context.Background(), "https://example.com/article", types.ContentText)
	require.NotNil(t, text)
	require.Empty(t, text.Error)
	assert.Equal(t, "Web Article", text.Title)

	ytdlpCalls := readFileOrEmpty(t, ytdlpLog)
	opencliCalls := readFileOrEmpty(t, opencliLog)
	assert.Contains(t, ytdlpCalls, "--dump-json")
	assert.NotContains(t, ytdlpCalls, "example.com")
	assert.Contains(t, opencliCalls, "example.com")
	assert.NotContains(t, opencliCalls, "youtube.com")
}

func TestHybridDriverBilibiliNoSubtitleFallsBackToOpenCLISummary(t *testing.T) {
	binDir := t.TempDir()
	writeFakeBin(t, binDir, "yt-dlp", `#!/bin/sh
printf '%s\n' '{"title":"B站视频","subtitles":{},"automatic_captions":{}}'
`)
	writeFakeBin(t, binDir, "opencli", `#!/bin/sh
if [ -n "$OPENCLI_FAKE_LOG" ]; then printf '%s\n' "$*" >> "$OPENCLI_FAKE_LOG"; fi
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
    esac
    ;;
  *)
    printf 'mock\n'
    ;;
esac
`)
	prependPATH(t, binDir)
	opencliLog := filepath.Join(t.TempDir(), "opencli.log")
	t.Setenv("OPENCLI_FAKE_LOG", opencliLog)

	driver := newHybridDriver(DriverOptions{MaxBodySize: 20000, MediaEnabled: true})
	result := driver.FetchContent(context.Background(), "https://www.bilibili.com/video/BV1xx", types.ContentVideo)

	require.NotNil(t, result)
	require.Empty(t, result.Error)
	assert.True(t, result.HasTranscript)
	assert.Contains(t, result.Body, "AI 总结")
	assert.Contains(t, readFileOrEmpty(t, opencliLog), "bilibili summary")
}

func TestYTDLPDriverMediaDisabled(t *testing.T) {
	driver := newYTDLPDriver(DriverOptions{MaxBodySize: 20000, MediaEnabled: false})
	result := driver.FetchContent(context.Background(), "https://www.youtube.com/watch?v=abc", types.ContentVideo)

	require.NotNil(t, result)
	assert.Contains(t, result.Error, "media extraction disabled")
}

func readFileOrEmpty(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return ""
	}
	require.NoError(t, err)

	return strings.TrimSpace(string(data))
}
