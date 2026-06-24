package transcript

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNormalizeTranscriptContentHTML(t *testing.T) {
	got := normalizeTranscriptContent(`<html><body><h1>Transcript</h1><p>Hello <strong>world</strong>.</p></body></html>`, htmlContentType)

	assert.Contains(t, got, "Transcript")
	assert.Contains(t, got, "Hello")
	assert.Contains(t, got, "world")
	assert.NotContains(t, got, "<p>", "HTML transcripts should be converted to Markdown")
}

func TestDetectTranscriptContentType(t *testing.T) {
	assert.Equal(t, srtContentType,
		detectTranscriptContentType("https://example.com/transcript", "application/x-subrip; charset=utf-8", nil))
	assert.Equal(t, jsonContentType,
		detectTranscriptContentType("https://example.com/transcript.json", "", []byte("not json, but URL wins")))
	assert.Equal(t, htmlContentType,
		detectTranscriptContentType("https://example.com/transcript", "", []byte(`<!doctype html><html><body>Transcript</body></html>`)))
}

func TestIsTranscriptURLHTMLRequiresTranscriptHint(t *testing.T) {
	assert.True(t, isTranscriptURL("https://example.com/transcript.html"))
	assert.True(t, isTranscriptURL("https://example.com/file.vtt"))
	assert.False(t, isTranscriptURL("https://example.com/article.html"))
}

// --- RssTranscriptProvider.Fetch with httptest ---

func TestRssTranscriptProviderFetchServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(server.Close)

	p := NewRssTranscriptProvider()
	ep := &EpisodeRef{
		Title: "Episode",
		TranscriptLinks: []TranscriptLink{
			{URL: server.URL + "/transcript.txt", Type: "text/plain"},
		},
	}
	_, err := p.Fetch(context.Background(), ep)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "fetch transcript URL")
}

func TestRssTranscriptProviderFetchSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte("This is a long enough transcript content for testing purposes. It needs to be reasonably long."))
	}))
	t.Cleanup(server.Close)

	p := NewRssTranscriptProvider()
	ep := &EpisodeRef{
		Title: "Episode",
		TranscriptLinks: []TranscriptLink{
			{URL: server.URL + "/transcript.txt", Type: "text/plain"},
		},
	}
	result, err := p.Fetch(context.Background(), ep)
	require.NoError(t, err)
	assert.NotEmpty(t, result.Content)
	assert.Equal(t, "rss-transcript", result.Source)
}

// --- DescriptionLinkProvider.Fetch with httptest ---

func TestDescriptionLinkProviderFetchServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	t.Cleanup(server.Close)

	p := NewDescriptionLinkProvider()
	ep := &EpisodeRef{
		Title:       "Episode",
		URL:         "https://example.com/ep1",
		Description: "Transcript at " + server.URL + "/transcript.txt",
	}
	_, err := p.Fetch(context.Background(), ep)
	require.Error(t, err)
}

// --- normalizeTranscriptContent VTT/SRT ---

func TestNormalizeTranscriptContentSRTProvider(t *testing.T) {
	srt := "1\n00:00:01,000 --> 00:00:04,000\nHello world\n\n2\n00:00:05,000 --> 00:00:08,000\nThis is a test\n"
	result := normalizeTranscriptContent(srt, "srt")
	assert.Contains(t, result, "Hello world")
}

// --- cleanSubtitle fallback ---

func TestCleanSubtitleInvalidSRTProvider(t *testing.T) {
	// go-astisub may parse this successfully with no items, returning empty
	result := cleanSubtitle("not valid srt\n\nwith some lines", "srt")
	_ = result // just exercise the code path
}

// --- contentTypeFromMediaType ---

func TestContentTypeFromMediaTypeUnknown(t *testing.T) {
	_, ok := contentTypeFromMediaType("application/unknown")
	assert.False(t, ok)
}

func TestContentTypeFromMediaTypeEmpty(t *testing.T) {
	_, ok := contentTypeFromMediaType("")
	assert.False(t, ok)
}

// --- normalizeMediaType ---

func TestNormalizeMediaTypeEmpty(t *testing.T) {
	assert.Empty(t, normalizeMediaType(""))
}

func TestNormalizeMediaTypeWithCharset(t *testing.T) {
	assert.Equal(t, "text/vtt", normalizeMediaType("text/vtt;charset=utf-8"))
}

func TestNormalizeMediaTypeSRT(t *testing.T) {
	assert.Equal(t, "application/srt", normalizeMediaType("text/srt"))
}

func TestNormalizeMediaTypeSubrip(t *testing.T) {
	assert.Equal(t, "application/x-subrip", normalizeMediaType("application/x-subrip"))
}

// --- Pipeline with httptest ---

func TestPipelineFirstProviderSucceeds(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte("This is a long enough transcript content for testing pipeline. Needs some length."))
	}))
	t.Cleanup(server.Close)

	p := NewPipeline(NewRssTranscriptProvider())
	ep := &EpisodeRef{
		Title: "Episode",
		TranscriptLinks: []TranscriptLink{
			{URL: server.URL + "/transcript.txt", Type: "text/plain"},
		},
	}
	result, source, err := p.Fetch(context.Background(), ep)
	require.NoError(t, err)
	assert.NotEmpty(t, result.Content)
	assert.Equal(t, "rss-transcript", source)
}

func TestPipelineFallbackToSecondProvider(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte("This is a long enough transcript content from the description link provider."))
	}))
	t.Cleanup(server.Close)

	p := NewPipeline(NewRssTranscriptProvider(), NewDescriptionLinkProvider())
	ep := &EpisodeRef{
		Title:       "Episode",
		Description: "Transcript: " + server.URL + "/transcript.txt",
	}
	result, source, err := p.Fetch(context.Background(), ep)
	require.NoError(t, err)
	assert.NotEmpty(t, result.Content)
	assert.Equal(t, "description-link", source)
}

// --- XiaoyuzhouProvider ---

func TestXiaoyuzhouProviderValidateCredentialsNoFile(t *testing.T) {
	p := NewXiaoyuzhouProvider(filepath.Join(t.TempDir(), "nonexistent.json"))
	err := p.ValidateCredentials(context.Background())
	require.Error(t, err)
}
