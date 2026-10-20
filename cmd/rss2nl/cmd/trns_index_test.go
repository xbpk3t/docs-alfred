package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xbpk3t/docs-alfred/internal/rss/feed"
	"github.com/xbpk3t/docs-alfred/pkg/litter"
)

type fakeUploader struct {
	name     string
	filename string
	content  string
	err      error
	calls    int
}

func (f *fakeUploader) Upload(_ context.Context, filename, content string) (*litter.Result, error) {
	f.calls++
	f.filename = filename
	f.content = content
	if f.err != nil {
		return nil, f.err
	}

	return &litter.Result{URL: "https://media.example/" + filename, Driver: f.name}, nil
}

func (f *fakeUploader) Name() string { return f.name }

func TestCollectTrnsIndexRecordsKeepsOnlyLinkedItems(t *testing.T) {
	categories := []NewsletterCategory{
		{Items: []NewsletterItem{
			{Title: "Ep 1", Link: "https://pod.example/ep1", FeedTitle: "Feed", TrnsURL: "https://media.example/t1.html"},
			{Title: "Ep 2", Link: "https://pod.example/ep2", FeedTitle: "Feed"}, // no transcript uploaded
		}},
		{Items: []NewsletterItem{
			{Title: "Ep 3", TrnsURL: "https://media.example/t3.html"}, // no episode URL
		}},
	}

	records := collectTrnsIndexRecords(categories)

	require.Len(t, records, 1)
	assert.Equal(t, trnsIndexRecord{
		URL:           "https://pod.example/ep1",
		TranscriptURL: "https://media.example/t1.html",
		Title:         "Ep 1",
		FeedTitle:     "Feed",
	}, records[0])
}

// The digest parses these objects with its own struct, so pin the wire shape:
// url / transcriptUrl / title / feedTitle.
func TestUploadTrnsIndexPublishesDigestWireShape(t *testing.T) {
	uploader := &fakeUploader{name: "r2"}
	records := []trnsIndexRecord{{
		URL:           "https://pod.example/ep1",
		TranscriptURL: "https://media.example/t1.html",
		Title:         "Ep 1",
		FeedTitle:     "Feed",
	}}

	require.NoError(t, uploadTrnsIndex(context.Background(), uploader, records))

	assert.Equal(t, 1, uploader.calls)
	assert.True(t, strings.HasPrefix(uploader.filename, trnsIndexPrefix), "key %q must list under the digest prefix", uploader.filename)
	assert.True(t, strings.HasSuffix(uploader.filename, ".json"))
	assert.Regexp(t, `^index-\d{4}-\d{2}-\d{2}T\d{6}\.json$`, uploader.filename)

	var decoded []map[string]string
	require.NoError(t, json.Unmarshal([]byte(uploader.content), &decoded))
	require.Len(t, decoded, 1)
	assert.Equal(t, "https://pod.example/ep1", decoded[0]["url"])
	assert.Equal(t, "https://media.example/t1.html", decoded[0]["transcriptUrl"])
	assert.Equal(t, "Ep 1", decoded[0]["title"])
	assert.Equal(t, "Feed", decoded[0]["feedTitle"])
}

func TestUploadTrnsIndexUploadErrorPropagates(t *testing.T) {
	uploader := &fakeUploader{name: "r2", err: errors.New("r2 put: 503")}

	err := uploadTrnsIndex(context.Background(), uploader, []trnsIndexRecord{{URL: "u", TranscriptURL: "t"}})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "upload trns index")
}

func TestPublishTrnsIndexNoBucketIsNoop(t *testing.T) {
	cfg := &rss.Config{}
	categories := []NewsletterCategory{{Items: []NewsletterItem{
		{Title: "Ep 1", Link: "https://pod.example/ep1", TrnsURL: "https://media.example/t1.html"},
	}}}

	assert.NoError(t, publishTrnsIndex(cfg, categories))
}

func TestPublishTrnsIndexWithoutLinkedItemsIsNoop(t *testing.T) {
	cfg := &rss.Config{}
	cfg.R2Config.Bucket = "media"
	cfg.R2Config.PublicBaseURL = "https://media.example"

	assert.NoError(t, publishTrnsIndex(cfg, nil))
}

func TestNewTrnsUploaderDefaultsToLitterbox(t *testing.T) {
	uploader, err := newTrnsUploader(&rss.Config{})
	require.NoError(t, err)
	assert.Equal(t, "fallback", uploader.Name())
}

func TestNewTrnsUploaderR2WithoutConfigErrors(t *testing.T) {
	cfg := &rss.Config{}
	cfg.TrnsConfig.TemporaryUpload.Driver = trnsDriverR2

	_, err := newTrnsUploader(cfg)

	require.Error(t, err, "the r2 driver must not silently fall back to an expiring host")
}

func TestNewTrnsUploaderR2WithConfig(t *testing.T) {
	t.Setenv("R2_AK", "ak")
	t.Setenv("R2_SK", "sk")
	t.Setenv("R2_ENDPOINT", "https://acct.r2.cloudflarestorage.com")
	cfg := &rss.Config{}
	cfg.TrnsConfig.TemporaryUpload.Driver = trnsDriverR2
	cfg.R2Config.Bucket = "media"
	cfg.R2Config.PublicBaseURL = "https://media.example"

	uploader, err := newTrnsUploader(cfg)

	require.NoError(t, err)
	assert.Equal(t, "fallback", uploader.Name())
}

func TestNewTrnsUploaderUnknownDriverErrors(t *testing.T) {
	cfg := &rss.Config{}
	cfg.TrnsConfig.TemporaryUpload.Driver = "nope"

	_, err := newTrnsUploader(cfg)

	require.Error(t, err)
}
