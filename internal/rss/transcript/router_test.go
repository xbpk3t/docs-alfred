package transcript_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xbpk3t/docs-alfred/internal/rss/transcript"
	"github.com/xbpk3t/docs-alfred/internal/rss/transcript/mocks"
	"go.uber.org/mock/gomock"
)

func TestRouterXiaoyuzhouEpisode(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockXY := mocks.NewMockProvider(ctrl)
	mockRSS := mocks.NewMockProvider(ctrl)

	mockXY.EXPECT().Fetch(gomock.Any(), gomock.Any()).Return(&transcript.TranscriptResult{
		Content: "transcript", ContentType: "plaintext", Source: "xiaoyuzhou",
	}, nil)

	r := &transcript.Router{Xiaoyuzhou: mockXY, RssTranscript: mockRSS}
	result, err := r.Fetch(context.Background(), &transcript.EpisodeRef{
		URL: "https://www.xiaoyuzhoufm.com/episode/abc123",
	})
	require.NoError(t, err)
	assert.Equal(t, "transcript", result.Content)
}

func TestRouterRSSTranscriptEpisode(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockXY := mocks.NewMockProvider(ctrl)
	mockRSS := mocks.NewMockProvider(ctrl)

	mockRSS.EXPECT().Fetch(gomock.Any(), gomock.Any()).Return(&transcript.TranscriptResult{
		Content: "rss transcript", ContentType: "plaintext", Source: "rss-transcript",
	}, nil)

	r := &transcript.Router{Xiaoyuzhou: mockXY, RssTranscript: mockRSS}
	result, err := r.Fetch(context.Background(), &transcript.EpisodeRef{
		URL:             "https://example.com/ep1",
		TranscriptLinks: []transcript.TranscriptLink{{URL: "https://example.com/t.txt"}},
	})
	require.NoError(t, err)
	assert.Equal(t, "rss transcript", result.Content)
}

func TestRouterNoSource(t *testing.T) {
	ctrl := gomock.NewController(t)
	r := &transcript.Router{
		Xiaoyuzhou:    mocks.NewMockProvider(ctrl),
		RssTranscript: mocks.NewMockProvider(ctrl),
	}
	_, err := r.Fetch(context.Background(), &transcript.EpisodeRef{URL: "https://example.com/ep1"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no transcript source")
}

func TestPipelineFirstProviderSucceeds(t *testing.T) {
	ctrl := gomock.NewController(t)
	p1 := mocks.NewMockProvider(ctrl)
	p2 := mocks.NewMockProvider(ctrl)

	p1.EXPECT().Fetch(gomock.Any(), gomock.Any()).Return(&transcript.TranscriptResult{
		Content: "result", ContentType: "plaintext",
	}, nil)
	p1.EXPECT().Name().Return("p1")

	pipeline := transcript.NewPipeline(p1, p2)
	result, source, err := pipeline.Fetch(context.Background(), &transcript.EpisodeRef{})
	require.NoError(t, err)
	assert.Equal(t, "result", result.Content)
	assert.Equal(t, "p1", source)
}

func TestPipelineFallsToSecondProvider(t *testing.T) {
	ctrl := gomock.NewController(t)
	p1 := mocks.NewMockProvider(ctrl)
	p2 := mocks.NewMockProvider(ctrl)

	p1.EXPECT().Fetch(gomock.Any(), gomock.Any()).Return(nil, errors.New("no transcript"))
	p2.EXPECT().Fetch(gomock.Any(), gomock.Any()).Return(&transcript.TranscriptResult{
		Content: "fallback", ContentType: "plaintext",
	}, nil)
	p2.EXPECT().Name().Return("p2")

	pipeline := transcript.NewPipeline(p1, p2)
	result, source, err := pipeline.Fetch(context.Background(), &transcript.EpisodeRef{})
	require.NoError(t, err)
	assert.Equal(t, "fallback", result.Content)
	assert.Equal(t, "p2", source)
}

func TestPipelineAllFail(t *testing.T) {
	ctrl := gomock.NewController(t)
	p1 := mocks.NewMockProvider(ctrl)
	p2 := mocks.NewMockProvider(ctrl)

	p1.EXPECT().Fetch(gomock.Any(), gomock.Any()).Return(nil, errors.New("fail1"))
	p2.EXPECT().Fetch(gomock.Any(), gomock.Any()).Return(nil, errors.New("fail2"))

	pipeline := transcript.NewPipeline(p1, p2)
	_, _, err := pipeline.Fetch(context.Background(), &transcript.EpisodeRef{})
	require.Error(t, err)
}

func TestPipelineAllReturnEmptyContent(t *testing.T) {
	ctrl := gomock.NewController(t)
	p1 := mocks.NewMockProvider(ctrl)

	p1.EXPECT().Fetch(gomock.Any(), gomock.Any()).Return(&transcript.TranscriptResult{
		Content: "", ContentType: "plaintext",
	}, nil)

	pipeline := transcript.NewPipeline(p1)
	_, _, err := pipeline.Fetch(context.Background(), &transcript.EpisodeRef{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "all providers failed")
}
