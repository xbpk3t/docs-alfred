package wiki

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMarkdownFallbackBodyConvertsHTML(t *testing.T) {
	body := markdownFallbackBody([]byte(`<html><head><title>Page Title</title></head><body><h1>Hello</h1><p>Read <a href="https://example.com">more</a>.</p></body></html>`))

	assert.Contains(t, body, "Hello")
	assert.Contains(t, body, "[more](https://example.com)")
	assert.False(t, strings.Contains(body, "<h1>"), "fallback body should not be raw HTML")
}

func TestFetchHTTPPageRejectsLowQualitySuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`<html><head><title>X</title></head><body>This page requires JavaScript.</body></html>`))
	}))
	t.Cleanup(server.Close)

	fetcher := NewFetcher(WithOpenCLIFallback(false))
	result := fetcher.FetchContent(context.Background(), server.URL, ContentText)

	require.NotNil(t, result)
	require.Contains(t, result.Error, "extract:")
	require.Contains(t, result.Error, "low quality")
}

func TestFetchVideoTranscriptMissingYTDLPSoftFails(t *testing.T) {
	fetcher := NewFetcher(WithSubtitleCLIPath(filepath.Join(t.TempDir(), "missing-yt-dlp")))

	result := fetcher.FetchContent(context.Background(), "https://www.youtube.com/watch?v=abc", ContentVideo)

	require.NotNil(t, result)
	require.Contains(t, result.Error, "extract:")
	require.Contains(t, result.Error, "subtitle CLI not found")
}

func TestFetchVideoTranscriptUsesPreferredSubtitle(t *testing.T) {
	subtitleServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", subtitleContentTypeVTT)
		_, _ = w.Write([]byte("WEBVTT\n\n00:00:00.000 --> 00:00:01.000\n你好，世界\n"))
	}))
	t.Cleanup(subtitleServer.Close)

	ytdlp := filepath.Join(t.TempDir(), "yt-dlp")
	script := `#!/bin/sh
has_ignore_config=0
for arg in "$@"; do
	if [ "$arg" = "--ignore-config" ]; then
		has_ignore_config=1
	fi
done
if [ "$has_ignore_config" -ne 1 ]; then
	echo "missing --ignore-config" >&2
	exit 2
fi
cat <<'JSON'
` + `{"title":"测试视频","language":"` + subtitleLangZhCN + `","subtitles":{"` + subtitleLangZhHans + `":[{"url":"` + subtitleServer.URL + `/sub.vtt","ext":"` + subtitleExtVTT + `"}]}}` + `
JSON
`
	require.NoError(t, os.WriteFile(ytdlp, []byte(script), 0o700))

	fetcher := NewFetcher(WithSubtitleCLIPath(ytdlp))
	result := fetcher.FetchContent(context.Background(), "https://www.bilibili.com/video/BV1xx", ContentVideo)

	require.NotNil(t, result)
	require.Empty(t, result.Error)
	assert.Equal(t, "测试视频", result.Title)
	assert.Contains(t, result.Body, "你好，世界")
	assert.Contains(t, result.Body, "subtitle:manual:zh-Hans")
}

func TestPickSubtitleFromMapMatchesLanguageTags(t *testing.T) {
	subtitles := map[string][]ytdlpSubtitle{
		"en-US":            {{URL: "https://example.com/en.vtt", Ext: subtitleExtVTT}},
		subtitleLangZhHant: {{URL: "https://example.com/zh.vtt", Ext: subtitleExtVTT}},
	}

	lang, item, ok := pickSubtitleFromMap(subtitles, []string{subtitleLangZhTW, subtitleLangEnglish})

	require.True(t, ok)
	assert.Equal(t, subtitleLangZhHant, lang)
	assert.Equal(t, "https://example.com/zh.vtt", item.URL)
}

func TestPickSubtitleFromMapPrefersEnglishTags(t *testing.T) {
	subtitles := map[string][]ytdlpSubtitle{
		"fr":    {{URL: "https://example.com/fr.vtt", Ext: subtitleExtVTT}},
		"en-GB": {{URL: "https://example.com/en.vtt", Ext: subtitleExtVTT}},
	}

	lang, item, ok := pickSubtitleFromMap(subtitles, []string{subtitleLangEnglish})

	require.True(t, ok)
	assert.Equal(t, "en-GB", lang)
	assert.Equal(t, "https://example.com/en.vtt", item.URL)
}

func TestPickSubtitleFromMapFallsBackDeterministically(t *testing.T) {
	subtitles := map[string][]ytdlpSubtitle{
		"zz": {{URL: "https://example.com/zz.vtt", Ext: subtitleExtVTT}},
		"aa": {{URL: "https://example.com/aa.vtt", Ext: subtitleExtVTT}},
	}

	lang, item, ok := pickSubtitleFromMap(subtitles, []string{subtitleLangEnglish})

	require.True(t, ok)
	assert.Equal(t, "aa", lang)
	assert.Equal(t, "https://example.com/aa.vtt", item.URL)
}

func TestMetadataLooksChineseUsesLanguageTagsAndTextFallback(t *testing.T) {
	assert.True(t, metadataLooksChinese(&ytdlpMetadata{Language: subtitleLangMandarin}))
	assert.True(t, metadataLooksChinese(&ytdlpMetadata{Title: "测试视频"}))
	assert.False(t, metadataLooksChinese(&ytdlpMetadata{Language: subtitleLangEnglish, Title: "demo"}))
}

func TestFetchPodcastTranscriptUsesRSSTranscript(t *testing.T) {
	transcriptServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte("This is a long enough podcast transcript pulled directly from the RSS transcript tag."))
	}))
	t.Cleanup(transcriptServer.Close)

	feedServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/rss+xml")
		_, _ = w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0" xmlns:podcast="https://podcastindex.org/namespace/1.0">
  <channel>
    <title>Example Podcast</title>
    <item>
      <title>Episode One</title>
      <link>https://example.com/episode-one</link>
      <podcast:transcript url="` + transcriptServer.URL + `/transcript.txt" type="text/plain" />
    </item>
  </channel>
</rss>`))
	}))
	t.Cleanup(feedServer.Close)

	fetcher := NewFetcher()
	result := fetcher.FetchContent(context.Background(), feedServer.URL+"/feed.xml", ContentAudio)

	require.NotNil(t, result)
	require.Empty(t, result.Error)
	assert.Equal(t, "Episode One", result.Title)
	assert.Contains(t, result.Body, "Transcript source: rss-transcript")
	assert.Contains(t, result.Body, "long enough podcast transcript")
}

func TestFetchDirectAudioDoesNotRunASR(t *testing.T) {
	fetcher := NewFetcher()
	result := fetcher.FetchContent(context.Background(), "https://example.com/audio.mp3", ContentAudio)

	require.NotNil(t, result)
	require.Contains(t, result.Error, "extract:")
	require.Contains(t, result.Error, "direct audio URL has no RSS metadata")
}

func TestMediaContentResultTruncatesUTF8Safely(t *testing.T) {
	result := mediaContentResult("title", "https://example.com/audio", "rss-transcript", strings.Repeat("你好", 100), 5)

	require.NotNil(t, result)
	assert.True(t, utf8.ValidString(result.Body))
	assert.Contains(t, result.Body, "...")
}

func TestExtractWithReadabilityTruncatesUTF8Safely(t *testing.T) {
	fetcher := NewFetcher()
	fetcher.MaxBodySize = 5
	body := strings.Repeat("你好世界。", 60)
	result := fetcher.extractWithReadability([]byte(`<html><head><title>测试</title></head><body><article><p>`+body+`</p></article></body></html>`), "https://example.com/a")

	require.NotNil(t, result)
	assert.True(t, utf8.ValidString(result.Body))
	assert.Contains(t, result.Body, "...")
}

func TestFetchWithOpenCLITruncatesUTF8Safely(t *testing.T) {
	binDir := t.TempDir()
	opencli := filepath.Join(binDir, "opencli")
	script := `#!/bin/sh
printf '# 测试\n'
printf '` + strings.Repeat("你好", 100) + `\n'
`
	require.NoError(t, os.WriteFile(opencli, []byte(script), 0o700))
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	fetcher := NewFetcher()
	fetcher.MaxBodySize = 5

	result := fetcher.fetchWithOpenCLI(context.Background(), "https://example.com/a")

	require.NotNil(t, result)
	require.Empty(t, result.Error)
	assert.True(t, utf8.ValidString(result.Body))
	assert.Contains(t, result.Body, "...")
}

func TestDetectContentTypeOnlyTreatsConcreteVideoURLsAsVideo(t *testing.T) {
	tests := []struct {
		name string
		url  string
		want string
	}{
		{name: "youtube watch", url: "https://www.youtube.com/watch?v=abc", want: ContentVideo},
		{name: "youtube shorts", url: "https://youtube.com/shorts/abc", want: ContentVideo},
		{name: "youtube embed", url: "https://www.youtube.com/embed/abc", want: ContentVideo},
		{name: "youtu be", url: "https://youtu.be/abc", want: ContentVideo},
		{name: "youtube channel", url: "https://www.youtube.com/@VirtualizationHowto/videos", want: ContentText},
		{name: "youtube playlist", url: "https://www.youtube.com/playlist?list=abc", want: ContentText},
		{name: "bilibili video", url: "https://www.bilibili.com/video/BV1xx", want: ContentVideo},
		{name: "bilibili space", url: "https://space.bilibili.com/123", want: ContentText},
		{name: "bilibili homepage", url: "https://www.bilibili.com/", want: ContentText},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, DetectContentType(tt.url))
		})
	}
}
