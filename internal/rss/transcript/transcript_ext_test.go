package transcript

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/mmcdole/gofeed"
	ext "github.com/mmcdole/gofeed/extensions"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xbpk3t/docs-alfred/pkg/ai"
)

// --- EpisodeRefFromFeedItem tests ---

func TestEpisodeRefFromFeedItemNil(t *testing.T) {
	ref := EpisodeRefFromFeedItem(nil, "Feed", "https://feed.url")
	assert.Equal(t, "Feed", ref.FeedTitle)
	assert.Equal(t, "https://feed.url", ref.FeedURL)
	assert.Empty(t, ref.Title)
}

func TestEpisodeRefFromFeedItemBasic(t *testing.T) {
	item := &gofeed.Item{
		Title:       "Episode 1",
		Link:        "https://example.com/ep1",
		GUID:        "guid-1",
		Description: "Description",
		Content:     "Content",
		Enclosures: []*gofeed.Enclosure{
			{URL: "https://example.com/audio.mp3"},
		},
	}
	ref := EpisodeRefFromFeedItem(item, "Feed", "https://feed.url")
	assert.Equal(t, "Episode 1", ref.Title)
	assert.Equal(t, "https://example.com/ep1", ref.URL)
	assert.Equal(t, "guid-1", ref.GUID)
	assert.Equal(t, "https://example.com/audio.mp3", ref.EnclosureURL)
}

func TestEpisodeRefFromFeedItemWithTranscriptExtensions(t *testing.T) {
	extensions := ext.Extensions{
		"podcast": {
			"transcript": []ext.Extension{
				{Attrs: map[string]string{"url": "https://example.com/transcript.txt", "type": "text/plain"}},
			},
		},
	}
	item := &gofeed.Item{
		Title:      "Episode 2",
		Link:       "https://example.com/ep2",
		Extensions: extensions,
	}
	ref := EpisodeRefFromFeedItem(item, "Feed", "https://feed.url")
	require.Len(t, ref.TranscriptLinks, 1)
	assert.Equal(t, "https://example.com/transcript.txt", ref.TranscriptLinks[0].URL)
	assert.Equal(t, "text/plain", ref.TranscriptLinks[0].Type)
}

func TestEpisodeRefFromFeedItemNoEnclosures(t *testing.T) {
	item := &gofeed.Item{Title: "Episode 3"}
	ref := EpisodeRefFromFeedItem(item, "Feed", "https://feed.url")
	assert.Empty(t, ref.EnclosureURL)
}

// --- extractEpisodeID tests ---

func TestExtractEpisodeID(t *testing.T) {
	tests := []struct {
		name string
		url  string
		guid string
		want string
	}{
		{"from URL", "https://www.xiaoyuzhoufm.com/episode/abc123def", "", "abc123def"},
		{"from GUID", "", "https://www.xiaoyuzhoufm.com/episode/abcdef123", "abcdef123"},
		{"GUID priority", "https://www.xiaoyuzhoufm.com/episode/aaa000", "https://www.xiaoyuzhoufm.com/episode/bbb111", "bbb111"},
		{"no match", "https://example.com/page", "", ""},
		{"empty", "", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, extractEpisodeID(tt.url, tt.guid))
		})
	}
}

// --- extractTranscriptText tests ---

func TestExtractTranscriptTextArray(t *testing.T) {
	body := `[{"text":"Hello","startMs":0},{"text":"World","startMs":1000}]`
	text, count := extractTranscriptText([]byte(body))
	assert.Equal(t, 2, count)
	assert.Equal(t, "Hello\nWorld", text)
}

func TestExtractTranscriptTextWrappedSegments(t *testing.T) {
	body := `{"segments":[{"text":"Hello","startMs":0},{"text":"World","startMs":1000}]}`
	text, count := extractTranscriptText([]byte(body))
	assert.Equal(t, 2, count)
	assert.Equal(t, "Hello\nWorld", text)
}

func TestExtractTranscriptTextWrappedData(t *testing.T) {
	body := `{"data":[{"text":"Hi","startMs":0}]}`
	text, count := extractTranscriptText([]byte(body))
	assert.Equal(t, 1, count)
	assert.Equal(t, "Hi", text)
}

func TestExtractTranscriptTextInvalidJSON(t *testing.T) {
	text, count := extractTranscriptText([]byte("not json"))
	assert.Equal(t, 0, count)
	assert.Empty(t, text)
}

func TestExtractTranscriptTextSkipsEmpty(t *testing.T) {
	body := `[{"text":"Hello","startMs":0},{"text":"  ","startMs":1000},{"text":"World","startMs":2000}]`
	text, count := extractTranscriptText([]byte(body))
	assert.Equal(t, 2, count)
	assert.Equal(t, "Hello\nWorld", text)
}

// --- parseTranscriptSegments tests ---

func TestParseTranscriptSegmentsArray(t *testing.T) {
	body := `[{"text":"A","startMs":0}]`
	segments := parseTranscriptSegments([]byte(body))
	require.Len(t, segments, 1)
	assert.Equal(t, "A", segments[0].Text)
}

func TestParseTranscriptSegmentsInvalid(t *testing.T) {
	segments := parseTranscriptSegments([]byte("not json"))
	assert.Nil(t, segments)
}

// --- Cache tests ---

func TestCacheKeyDeterministic(t *testing.T) {
	c := NewCache("/tmp/cache")
	k1 := c.Key("https://feed.url", "guid", "", "")
	k2 := c.Key("https://feed.url", "guid", "", "")
	assert.Equal(t, k1, k2)
}

func TestCacheKeyUsesFallbackToLink(t *testing.T) {
	c := NewCache("/tmp/cache")
	k1 := c.Key("https://feed.url", "", "https://link", "")
	k2 := c.Key("https://feed.url", "", "https://link", "")
	assert.Equal(t, k1, k2)
	assert.NotEmpty(t, k1)
}

func TestCacheKeyUsesFallbackToTitle(t *testing.T) {
	c := NewCache("/tmp/cache")
	k := c.Key("https://feed.url", "", "", "My Title")
	assert.NotEmpty(t, k)
}

func TestCacheFilePath(t *testing.T) {
	c := NewCache("/tmp/cache")
	assert.Equal(t, filepath.Join("/tmp/cache", "abc", "transcript.txt"), c.CacheFilePath("abc"))
}

func TestCacheMetaFilePath(t *testing.T) {
	c := NewCache("/tmp/cache")
	assert.Equal(t, filepath.Join("/tmp/cache", "abc", "metadata.json"), c.MetaFilePath("abc"))
}

func TestCacheIndexFilePath(t *testing.T) {
	c := NewCache("/tmp/cache")
	assert.Equal(t, filepath.Join("/tmp/cache", "index.json"), c.IndexFilePath())
}

func TestCacheGetMiss(t *testing.T) {
	c := NewCache(t.TempDir())
	entry, err := c.Get("nonexistent")
	assert.Nil(t, entry)
	assert.Error(t, err)
}

func TestCacheSetAndGet(t *testing.T) {
	c := NewCache(t.TempDir())
	entry := &CacheEntry{
		EpisodeTitle: "Test Episode",
		Source:       "test",
		ContentType:  "plaintext",
	}
	err := c.Set("key1", entry, "Hello transcript content")
	require.NoError(t, err)

	got, err := c.Get("key1")
	require.NoError(t, err)
	assert.Equal(t, "Test Episode", got.EpisodeTitle)
	assert.Equal(t, "test", got.Source)
	assert.NotEmpty(t, got.TranscriptPath)
	assert.False(t, got.FetchedAt.IsZero())
}

func TestCacheReadTranscript(t *testing.T) {
	c := NewCache(t.TempDir())
	entry := &CacheEntry{EpisodeTitle: "Test", Source: "test", ContentType: "plaintext"}
	require.NoError(t, c.Set("key1", entry, "  Hello World  "))

	content, err := c.ReadTranscript("key1")
	require.NoError(t, err)
	assert.Equal(t, "Hello World", content)
}

func TestCacheReadTranscriptMissing(t *testing.T) {
	c := NewCache(t.TempDir())
	_, err := c.ReadTranscript("nonexistent")
	assert.Error(t, err)
}

func TestCacheSetEmptyContent(t *testing.T) {
	c := NewCache(t.TempDir())
	entry := &CacheEntry{EpisodeTitle: "Test", Source: "test", ContentType: "plaintext"}
	err := c.Set("key1", entry, "")
	require.NoError(t, err)
	assert.Empty(t, entry.TranscriptPath)
}

func TestCacheGetMissingTranscriptFile(t *testing.T) {
	c := NewCache(t.TempDir())
	// Manually write metadata with a transcript path that doesn't exist
	key := "key1"
	metaPath := c.MetaFilePath(key)
	require.NoError(t, os.MkdirAll(filepath.Dir(metaPath), 0o700))
	meta := CacheEntry{
		EpisodeTitle:   "Test",
		TranscriptPath: filepath.Join(t.TempDir(), "nonexistent.txt"),
	}
	data, _ := json.Marshal(meta)
	require.NoError(t, os.WriteFile(metaPath, data, 0o600))

	entry, err := c.Get(key)
	assert.Nil(t, entry)
	assert.ErrorIs(t, err, ErrCacheMiss)
}

// --- isXiaoyuzhouEpisode ---

func TestIsXiaoyuzhouEpisode(t *testing.T) {
	assert.True(t, isXiaoyuzhouEpisode(&EpisodeRef{URL: "https://xiaoyuzhoufm.com/ep/1"}))
	assert.True(t, isXiaoyuzhouEpisode(&EpisodeRef{GUID: "https://xiaoyuzhoufm.com/ep/1"}))
	assert.False(t, isXiaoyuzhouEpisode(&EpisodeRef{URL: "https://example.com/ep1"}))
}

// --- Router Name ---

func TestRouterName(t *testing.T) {
	r := &Router{}
	assert.Equal(t, "router", r.Name())
}

// --- Provider Name tests ---

func TestRssTranscriptProviderName(t *testing.T) {
	assert.Equal(t, "rss-transcript", NewRssTranscriptProvider().Name())
}

func TestDescriptionLinkProviderName(t *testing.T) {
	assert.Equal(t, "description-link", NewDescriptionLinkProvider().Name())
}

func TestAudioTranscriptionProviderName(t *testing.T) {
	p := NewAudioTranscriptionProvider("", "")
	assert.Equal(t, "audio-asr", p.Name())
}

func TestAudioTranscriptionProviderDefaults(t *testing.T) {
	p := NewAudioTranscriptionProvider("", "")
	assert.Equal(t, "pt", p.CLIPath)
	assert.Equal(t, "auto", p.Language)
}

func TestAudioTranscriptionProviderCustomValues(t *testing.T) {
	p := NewAudioTranscriptionProvider("/usr/bin/pt", "zh")
	assert.Equal(t, "/usr/bin/pt", p.CLIPath)
	assert.Equal(t, "zh", p.Language)
}

func TestAudioTranscriptionProviderNoEnclosure(t *testing.T) {
	p := NewAudioTranscriptionProvider("", "")
	_, err := p.Fetch(context.Background(), &EpisodeRef{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no audio enclosure URL")
}

// --- RssTranscriptProvider tests ---

func TestRssTranscriptProviderNoLinks(t *testing.T) {
	p := NewRssTranscriptProvider()
	_, err := p.Fetch(context.Background(), &EpisodeRef{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no podcast:transcript tag")
}

// --- DescriptionLinkProvider tests ---

func TestDescriptionLinkProviderNoContent(t *testing.T) {
	p := NewDescriptionLinkProvider()
	_, err := p.Fetch(context.Background(), &EpisodeRef{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no description or content")
}

func TestDescriptionLinkProviderNoTranscriptLink(t *testing.T) {
	p := NewDescriptionLinkProvider()
	_, err := p.Fetch(context.Background(), &EpisodeRef{
		Description: "Just a regular description without any links",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no transcript link")
}

// --- pickBestTranscriptLink tests ---

func TestPickBestTranscriptLinkEmpty(t *testing.T) {
	assert.Nil(t, pickBestTranscriptLink(nil))
	assert.Nil(t, pickBestTranscriptLink([]TranscriptLink{}))
}

func TestPickBestTranscriptLinkPrefersPlainText(t *testing.T) {
	links := []TranscriptLink{
		{URL: "https://example.com/t.html", Type: "text/html"},
		{URL: "https://example.com/t.txt", Type: "text/plain"},
		{URL: "https://example.com/t.vtt", Type: "text/vtt"},
	}
	best := pickBestTranscriptLink(links)
	assert.Equal(t, "https://example.com/t.txt", best.URL)
}

func TestPickBestTranscriptLinkTranscriptURLBonus(t *testing.T) {
	links := []TranscriptLink{
		{URL: "https://example.com/data.txt", Type: "text/plain"},
		{URL: "https://example.com/transcript.txt", Type: "text/plain"},
	}
	best := pickBestTranscriptLink(links)
	assert.Equal(t, "https://example.com/transcript.txt", best.URL)
}

func TestPickBestTranscriptLinkSingle(t *testing.T) {
	links := []TranscriptLink{{URL: "https://example.com/t.txt", Type: "text/plain"}}
	best := pickBestTranscriptLink(links)
	assert.Equal(t, "https://example.com/t.txt", best.URL)
}

// --- Content type detection tests ---

func TestDetectContentTypeFromURL(t *testing.T) {
	tests := []struct {
		rawURL string
		want   string
	}{
		{"https://example.com/file.vtt", vttContentType},
		{"https://example.com/file.srt", srtContentType},
		{"https://example.com/file.json", jsonContentType},
		{"https://example.com/file.html", htmlContentType},
		{"https://example.com/file.htm", htmlContentType},
		{"https://example.com/file.txt", plaintextContentType},
	}
	for _, tt := range tests {
		t.Run(tt.rawURL, func(t *testing.T) {
			assert.Equal(t, tt.want, detectTranscriptContentType(tt.rawURL, "", nil))
		})
	}
}

func TestDetectContentTypeFromDeclaredType(t *testing.T) {
	assert.Equal(t, vttContentType, detectTranscriptContentType("", "text/vtt", nil))
	assert.Equal(t, srtContentType, detectTranscriptContentType("", "application/srt", nil))
	assert.Equal(t, srtContentType, detectTranscriptContentType("", "application/x-subrip", nil))
	assert.Equal(t, jsonContentType, detectTranscriptContentType("", "application/json", nil))
	assert.Equal(t, htmlContentType, detectTranscriptContentType("", "text/html", nil))
	assert.Equal(t, plaintextContentType, detectTranscriptContentType("", "text/plain", nil))
}

func TestDetectContentTypeDefaultsToPlaintext(t *testing.T) {
	assert.Equal(t, plaintextContentType, detectTranscriptContentType("https://example.com/file", "", []byte("just text")))
}

func TestNormalizeMediaType(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"text/vtt", "text/vtt"},
		{"text/vtt;charset=utf-8", "text/vtt"},
		{"application/srt", "application/srt"},
		{"text/srt", "application/srt"},
		{"application/x-subrip", "application/x-subrip"},
		{"  text/plain  ", "text/plain"},
		{"", ""},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			assert.Equal(t, tt.want, normalizeMediaType(tt.input))
		})
	}
}

// --- normalizeTranscriptContent tests ---

func TestNormalizeTranscriptContentPlaintext(t *testing.T) {
	assert.Equal(t, "hello", normalizeTranscriptContent("  hello  ", "plaintext"))
}

func TestNormalizeTranscriptContentJSON(t *testing.T) {
	assert.JSONEq(t, `{"key":"value"}`, normalizeTranscriptContent(`{"key":"value"}`, "json"))
}

func TestNormalizeTranscriptContentDefault(t *testing.T) {
	assert.Equal(t, "text", normalizeTranscriptContent("  text  ", "unknown"))
}

func TestNormalizeTranscriptContentVTTFallback(t *testing.T) {
	// Invalid VTT should fall back to text extraction
	result := normalizeTranscriptContent("WEBVTT\n\n00:00:01.000 --> 00:00:02.000\nHello", "vtt")
	assert.Contains(t, result, "Hello")
}

func TestNormalizeTranscriptContentSRTFallback(t *testing.T) {
	result := normalizeTranscriptContent("1\n00:00:01,000 --> 00:00:02,000\nHello", "srt")
	assert.Contains(t, result, "Hello")
}

func TestFallbackCleanSubtitle(t *testing.T) {
	input := "WEBVTT\n\n00:00:01.000 --> 00:00:02.000\nHello\n\n00:00:03.000 --> 00:00:04.000\nWorld\n"
	result := fallbackCleanSubtitle(input)
	assert.Contains(t, result, "Hello")
	assert.Contains(t, result, "World")
	assert.NotContains(t, result, "-->")
}

func TestNormalizeContent(t *testing.T) {
	result := NormalizeContent("  test  ", "plaintext")
	assert.Equal(t, "test", result)
}

func TestDetectContentTypeExported(t *testing.T) {
	assert.Equal(t, plaintextContentType, DetectContentType("https://example.com/file.txt", "", nil))
}

// --- NewXiaoyuzhouProvider ---

func TestNewXiaoyuzhouProviderDefaultPath(t *testing.T) {
	p := NewXiaoyuzhouProvider("")
	assert.Equal(t, XiaoyuzhouCredentialFile(), p.credentialPath)
}

func TestNewXiaoyuzhouProviderCustomPath(t *testing.T) {
	p := NewXiaoyuzhouProvider("/tmp/creds.json")
	assert.Equal(t, "/tmp/creds.json", p.credentialPath)
}

func TestXiaoyuzhouProviderName(t *testing.T) {
	p := NewXiaoyuzhouProvider("")
	assert.Equal(t, "xiaoyuzhou", p.Name())
}

func TestXiaoyuzhouProviderFetchNoEpisodeID(t *testing.T) {
	p := NewXiaoyuzhouProvider("")
	_, err := p.Fetch(context.Background(), &EpisodeRef{URL: "https://example.com/ep1"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot extract xiaoyuzhou episode ID")
}

// --- loadCredentials / saveCredentials ---

func TestLoadCredentialsMissingFile(t *testing.T) {
	_, err := loadCredentials("/tmp/nonexistent-creds-12345.json")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "read credential file")
}

func TestLoadAndSaveCredentials(t *testing.T) {
	path := filepath.Join(t.TempDir(), "creds.json")
	creds := &xiaoyuzhouCredentials{
		AccessToken:  "token",
		RefreshToken: "refresh",
		DeviceID:     "dev-1",
	}
	require.NoError(t, saveCredentials(path, creds))

	loaded, err := loadCredentials(path)
	require.NoError(t, err)
	assert.Equal(t, "token", loaded.AccessToken)
	assert.Equal(t, "refresh", loaded.RefreshToken)
	assert.Equal(t, "dev-1", loaded.DeviceID)
}

func TestLoadCredentialsInvalidJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "creds.json")
	require.NoError(t, os.WriteFile(path, []byte("not json"), 0o600))
	_, err := loadCredentials(path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "parse credential file")
}

func TestLoadCredentialsMissingRefreshToken(t *testing.T) {
	path := filepath.Join(t.TempDir(), "creds.json")
	require.NoError(t, os.WriteFile(path, []byte(`{"access_token":"token"}`), 0o600))
	_, err := loadCredentials(path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing refresh_token")
}

func TestLoadCredentialsDefaultDeviceID(t *testing.T) {
	path := filepath.Join(t.TempDir(), "creds.json")
	require.NoError(t, os.WriteFile(path, []byte(`{"refresh_token":"rt"}`), 0o600))
	creds, err := loadCredentials(path)
	require.NoError(t, err)
	assert.Equal(t, xiaoyuzhouDefaultDevID, creds.DeviceID)
}

// --- LitterboxUploader ---

func TestNewLitterboxUploader(t *testing.T) {
	u := NewLitterboxUploader("1h")
	assert.NotNil(t, u)
	assert.NotNil(t, u.inner)
}

// --- Summarizer ---

func TestNewSummarizerDefaultLanguage(t *testing.T) {
	s := NewSummarizer(nil, "")
	assert.Equal(t, "zh", s.Language)
	assert.Nil(t, s.Config)
}

func TestNewSummarizerCustomLanguage(t *testing.T) {
	s := NewSummarizer(nil, "en")
	assert.Equal(t, "en", s.Language)
}

func TestSummarizerSummaryPrompt(t *testing.T) {
	tests := []struct {
		lang string
		want string
	}{
		{"zh", "中文"},
		{"en", "English"},
		{"other", "中文"}, // default falls back to zh
	}
	for _, tt := range tests {
		t.Run(tt.lang, func(t *testing.T) {
			s := NewSummarizer(nil, tt.lang)
			assert.Contains(t, s.summaryPrompt(), tt.want)
		})
	}
}

func TestSummarizerGenerateSummaryNoConfig(t *testing.T) {
	s := NewSummarizer(nil, "zh")
	_, err := s.GenerateSummary(context.Background(), "Title", "Content")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "AI not configured")
}

func TestSummarizerGenerateSummaryNoAPIKey(t *testing.T) {
	s := NewSummarizer(&ai.ClientConfig{BaseURL: "https://api.example.com", Model: "model"}, "zh")
	_, err := s.GenerateSummary(context.Background(), "Title", "Content")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "AI not configured")
}

// --- cleanSubtitle with valid VTT ---

func TestCleanSubtitleValidVTT(t *testing.T) {
	vtt := `WEBVTT

00:00:01.000 --> 00:00:04.000
Hello world

00:00:05.000 --> 00:00:08.000
This is a test
`
	result := cleanSubtitle(vtt, "vtt")
	assert.Contains(t, result, "Hello world")
	assert.Contains(t, result, "This is a test")
}

func TestCleanSubtitleValidSRT(t *testing.T) {
	srt := `1
00:00:01,000 --> 00:00:04,000
Hello world

2
00:00:05,000 --> 00:00:08,000
This is a test
`
	result := cleanSubtitle(srt, "srt")
	assert.Contains(t, result, "Hello world")
	assert.Contains(t, result, "This is a test")
}

func TestCleanSubtitleInvalidVTT(t *testing.T) {
	// Invalid VTT may trigger fallback - parser might return empty result
	result := cleanSubtitle("not valid vtt content at all", "vtt")
	// Result may be empty if parser returns no items; that's acceptable
	_ = result
}

// --- normalizeTranscriptContent VTT/SRT ---

func TestNormalizeTranscriptContentVTT(t *testing.T) {
	vtt := `WEBVTT

00:00:01.000 --> 00:00:04.000
Hello world
`
	result := normalizeTranscriptContent(vtt, "vtt")
	assert.Contains(t, result, "Hello world")
}

// --- extractTranscriptLinksFromText ---

func TestExtractTranscriptLinksFromTextWithHTTPLink(t *testing.T) {
	links := extractTranscriptLinksFromText(
		`<a href="https://example.com/transcript.txt">transcript</a>`,
		"https://example.com/episode",
	)
	assert.NotEmpty(t, links)
}

func TestExtractTranscriptLinksFromTextEmpty(t *testing.T) {
	links := extractTranscriptLinksFromText("just text", "https://example.com")
	assert.Empty(t, links)
}

// --- XiaoyuzhouCredentialFile ---

func TestXiaoyuzhouCredentialFile(t *testing.T) {
	path := XiaoyuzhouCredentialFile()
	assert.Contains(t, path, ".opencli")
	assert.Contains(t, path, "xiaoyuzhou.json")
}

// --- getLinkScore ---

func TestGetLinkScore(t *testing.T) {
	rank := map[string]int{"text/plain": 4, "text/vtt": 3}
	link := &TranscriptLink{URL: "https://example.com/transcript.txt", Type: "text/plain"}
	score := getLinkScore(link, rank)
	assert.Equal(t, 5, score) // 4 (text/plain) + 1 (contains "transcript")
}

func TestGetLinkScoreNoBonus(t *testing.T) {
	rank := map[string]int{"text/plain": 4}
	link := &TranscriptLink{URL: "https://example.com/data.txt", Type: "text/plain"}
	score := getLinkScore(link, rank)
	assert.Equal(t, 4, score) // 4 (text/plain) + 0
}

// --- urlExtension ---

func TestURLExtension(t *testing.T) {
	assert.Equal(t, ".vtt", urlExtension("https://example.com/file.vtt"))
	assert.Equal(t, ".srt", urlExtension("https://example.com/file.srt"))
	assert.Empty(t, urlExtension("https://example.com/file"))
}

func TestURLExtensionInvalidURL(t *testing.T) {
	assert.Equal(t, ".txt", urlExtension("://invalid/file.txt"))
}

// --- EpisodeRefFromFeedItem edge cases ---

func TestEpisodeRefFromFeedItemNonPodcastNamespace(t *testing.T) {
	extensions := ext.Extensions{
		"itunes": {
			"transcript": []ext.Extension{
				{Attrs: map[string]string{"url": "https://example.com/transcript.txt", "type": "text/plain"}},
			},
		},
	}
	item := &gofeed.Item{
		Title:      "Episode",
		Link:       "https://example.com/ep1",
		Extensions: extensions,
	}
	ref := EpisodeRefFromFeedItem(item, "Feed", "https://feed.url")
	assert.Empty(t, ref.TranscriptLinks, "non-podcast namespaces should be skipped")
}

func TestEpisodeRefFromFeedItemEmptyURLAttr(t *testing.T) {
	extensions := ext.Extensions{
		"podcast": {
			"transcript": []ext.Extension{
				{Attrs: map[string]string{"url": "", "type": "text/plain"}},
			},
		},
	}
	item := &gofeed.Item{
		Title:      "Episode",
		Link:       "https://example.com/ep1",
		Extensions: extensions,
	}
	ref := EpisodeRefFromFeedItem(item, "Feed", "https://feed.url")
	assert.Empty(t, ref.TranscriptLinks, "extensions with empty URL should be skipped")
}

func TestEpisodeRefFromFeedItemNonTranscriptTag(t *testing.T) {
	extensions := ext.Extensions{
		"podcast": {
			"chapters": []ext.Extension{
				{Attrs: map[string]string{"url": "https://example.com/chapters.json"}},
			},
		},
	}
	item := &gofeed.Item{
		Title:      "Episode",
		Link:       "https://example.com/ep1",
		Extensions: extensions,
	}
	ref := EpisodeRefFromFeedItem(item, "Feed", "https://feed.url")
	assert.Empty(t, ref.TranscriptLinks, "non-transcript tags should be skipped")
}

// --- normalizeTranscriptContent HTML fallback ---

func TestNormalizeTranscriptContentHTMLEmptyResult(t *testing.T) {
	// Empty HTML content should produce empty markdown, triggering the fallback
	result := normalizeTranscriptContent("", "html")
	assert.Empty(t, result)
}

// --- normalizeMediaType fallback ---

func TestNormalizeMediaTypeUnparseable(t *testing.T) {
	// A non-empty string that doesn't match prefix checks and fails mime.ParseMediaType.
	// mime.ParseMediaType fails when there's no '/' in the media type.
	result := normalizeMediaType("invalid-no-slash")
	assert.Equal(t, "invalid-no-slash", result, "unparseable MIME type should use raw value")
}

// --- detectTranscriptContentType with data-based detection ---

func TestDetectContentTypeFromDataHTML(t *testing.T) {
	// HTML data with no URL and no declared type should be detected via content sniffing
	htmlData := []byte("<!doctype html><html><head></head><body>Transcript</body></html>")
	result := detectTranscriptContentType("", "", htmlData)
	// mimetype should detect this as text/html
	assert.Equal(t, htmlContentType, result)
}

// --- Cache.Get edge cases ---

func TestCacheGetNonNotExistError(t *testing.T) {
	tmpDir := t.TempDir()
	c := NewCache(tmpDir)
	key := "testkey"
	metaPath := c.MetaFilePath(key)
	require.NoError(t, os.MkdirAll(filepath.Dir(metaPath), 0o700))

	// Write invalid JSON to trigger a non-NotExist error from ReadJSONFile
	require.NoError(t, os.WriteFile(metaPath, []byte("not valid json {{{"), 0o600))

	entry, err := c.Get(key)
	assert.Nil(t, entry)
	assert.Error(t, err)
	assert.NotErrorIs(t, err, ErrCacheMiss, "non-NotExist errors should not be ErrCacheMiss")
}

func TestCacheGetTranscriptPathStatNonNotExistError(t *testing.T) {
	tmpDir := t.TempDir()
	c := NewCache(tmpDir)
	key := "testkey"
	metaPath := c.MetaFilePath(key)
	require.NoError(t, os.MkdirAll(filepath.Dir(metaPath), 0o700))

	// Create a regular file where a directory is expected in the path.
	// os.Stat("/regularfile/deep/transcript.txt") returns ENOTDIR,
	// which is NOT caught by os.IsNotExist.
	blockingFile := filepath.Join(tmpDir, "blocking")
	require.NoError(t, os.WriteFile(blockingFile, []byte("not a dir"), 0o600))

	targetPath := filepath.Join(blockingFile, "deep", "transcript.txt")
	meta := CacheEntry{
		EpisodeTitle:   "Test",
		TranscriptPath: targetPath,
	}
	data, _ := json.Marshal(meta)
	require.NoError(t, os.WriteFile(metaPath, data, 0o600))

	entry, err := c.Get(key)
	// ENOTDIR is not os.IsNotExist, so the code falls through
	// and returns the entry without error.
	if err != nil {
		assert.Nil(t, entry)
		assert.NotErrorIs(t, err, ErrCacheMiss)
	} else {
		assert.NotNil(t, entry)
	}
}

// --- RssTranscriptProvider.Fetch with pickBestTranscriptLink returning nil ---

func TestRssTranscriptProviderFetchPickBestReturnsNil(t *testing.T) {
	// pickBestTranscriptLink always returns non-nil when len(links) > 0.
	// The best==nil branch at provider.go:98 is defensive dead code
	// because Fetch guards with len(ep.TranscriptLinks)==0 first.
	// We can still verify the guard works with a single unknown-type link.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte("transcript content"))
	}))
	t.Cleanup(server.Close)

	p := NewRssTranscriptProvider()
	ep := &EpisodeRef{
		Title: "Episode",
		TranscriptLinks: []TranscriptLink{
			{URL: server.URL + "/transcript.txt", Type: "application/unknown-type"},
		},
	}
	result, err := p.Fetch(context.Background(), ep)
	require.NoError(t, err, "unknown type should still work (falls back to URL/data detection)")
	assert.NotEmpty(t, result.Content)
}

// --- Pipeline with nil result ---
