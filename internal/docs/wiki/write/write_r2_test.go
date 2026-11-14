package write

import (
	"context"
	"strings"
	"sync"
	"testing"

	"github.com/adrg/frontmatter"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xbpk3t/docs-alfred/internal/docs/wiki/types"
)

// fakeDigestStore records uploads keyed by object name and returns a public URL
// per upload, so R2-mode write tests don't touch the network.
type fakeDigestStore struct {
	mu      sync.Mutex
	objects map[string][]byte
	puts    []string
	urlBase string
}

func newFakeDigestStore() *fakeDigestStore {
	return &fakeDigestStore{objects: map[string][]byte{}, urlBase: "https://dw.test"}
}

func (f *fakeDigestStore) Upload(_ context.Context, key, content string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.puts = append(f.puts, key)
	f.objects[key] = []byte(content)

	return f.urlBase + "/" + key, nil
}

func (f *fakeDigestStore) GetObject(_ context.Context, key string) ([]byte, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	return f.objects[key], nil
}

func TestDigestObjectKey(t *testing.T) {
	tests := []struct {
		name      string
		batch     string
		topic     string
		url       string
		wantPref  string
		wantLenAt int
	}{
		{"basic", "wiki-2026-10-26", "AI/agent", "https://example.com/a", "digest/wiki-2026-10-26/AI/agent/", -1},
		{"empty batch", "", "AI/agent", "https://example.com/a", "digest/unbatched/AI/agent/", -1},
		{"empty topic", "wiki-2026-10-26", "", "https://example.com/a", "digest/wiki-2026-10-26/uncat/", -1},
		{"slashes stripped", "/wiki-2026-10-26/", "/AI/agent/", "https://example.com/a", "digest/wiki-2026-10-26/AI/agent/", -1},
		// Topic segments outside the R2 key charset (CJK, spaces) are slugified so
		// Upload's path validator accepts them.
		{"cjk topic", "wiki-2026-10-26", "algo/algorithm/算法思维", "https://example.com/a", "digest/wiki-2026-10-26/algo/algorithm/", -1},
		{"space topic", "wiki-2026-10-26", "db/RDB/mysql 建表", "https://example.com/a", "digest/wiki-2026-10-26/db/RDB/", -1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key := digestObjectKey(tt.batch, tt.topic, tt.url)
			assert.True(t, strings.HasPrefix(key, tt.wantPref), "key %q should start with %q", key, tt.wantPref)
			assert.True(t, strings.HasSuffix(key, ".md"), "key should end with .md: %q", key)
			// Every segment must pass the R2 path validator's charset contract.
			for _, seg := range strings.Split(strings.TrimSuffix(strings.TrimPrefix(key, "digest/"), ".md"), "/") {
				assert.Regexp(t, `^[A-Za-z0-9][A-Za-z0-9._-]*$`, seg, "segment %q of key %q", seg, key)
			}
			// stable: same input → same key
			assert.Equal(t, key, digestObjectKey(tt.batch, tt.topic, tt.url))
		})
	}
}

func TestWriteSummaryR2UploadsPerPostObject(t *testing.T) {
	store := newFakeDigestStore()
	opts := &WriteOptions{R2Store: store}

	url := "https://example.com/a"
	path, err := WriteSummary(&types.ClassifyItem{
		URL:       url,
		Title:     "entry",
		TopicPath: "AI/agent",
		Type:      types.TypeDeepDive,
		Summary:   &types.StructuredSummary{Overview: "summary"},
	}, opts)

	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(path, store.urlBase+"/digest/"), "returned URL should point into the bucket: %q", path)

	// One digest object per post; the success log is a separate put.
	var digestPuts []string
	for _, p := range store.puts {
		if strings.HasPrefix(p, "digest/") {
			digestPuts = append(digestPuts, p)
		}
	}
	require.Len(t, digestPuts, 1, "one digest object per post, got %v", store.puts)
	key := digestPuts[0]
	assert.True(t, strings.HasPrefix(key, "digest/"), "key should be under digest/: %q", key)
	assert.Contains(t, key, "/AI/agent/")
	assert.True(t, strings.HasSuffix(key, ".md"))

	content := string(store.objects[key])
	assert.True(t, strings.HasPrefix(content, "---\n"), "post must start with YAML frontmatter: %q", content)
	assert.Contains(t, content, "title: entry\n")
	assert.Contains(t, content, "URL: "+url+"\n")
	assert.Contains(t, content, "topic: AI/agent\n")
	assert.NotContains(t, content, "```")
	assert.NotContains(t, content, "\n### ")
}

// The frontmatter must round-trip through a YAML parser: the original metadata
// values are emitted as quoted scalars when needed.
func TestBuildEntryFrontmatterRoundTrips(t *testing.T) {
	item := &types.ClassifyItem{
		URL:           "https://example.com/a?x=1&y=2",
		Title:         "Title: with a colon",
		TopicPath:     "AI/agent",
		Type:          types.TypeDeepDive,
		MetadataBlock: "Type: text\ntags: KWIC, 系统设计\nverdict: try",
		TranscriptURL: "https://example.com/t.md",
		Summary:       &types.StructuredSummary{Overview: "summary"},
	}

	var fm struct {
		Title      string `yaml:"title"`
		URL        string `yaml:"URL"`
		Topic      string `yaml:"topic"`
		Type       string `yaml:"Type"`
		Tags       string `yaml:"tags"`
		Verdict    string `yaml:"verdict"`
		Transcript string `yaml:"Transcript"`
	}
	body, err := frontmatter.Parse(strings.NewReader(buildEntry(item)), &fm)
	require.NoError(t, err)
	assert.Equal(t, item.Title, fm.Title)
	assert.Equal(t, item.URL, fm.URL)
	assert.Equal(t, item.TopicPath, fm.Topic)
	assert.Equal(t, "text", fm.Type)
	assert.Equal(t, "KWIC, 系统设计", fm.Tags)
	assert.Equal(t, "try", fm.Verdict)
	assert.Equal(t, item.TranscriptURL, fm.Transcript)
	assert.NotContains(t, string(body), "---")
	assert.Contains(t, string(body), "#### overview")
}

func TestLogDigestEntryR2MergesJSONL(t *testing.T) {
	store := newFakeDigestStore()
	opts := &WriteOptions{R2Store: store, BatchID: "wiki-2026-10-26"}

	// Two successes append to the same logs/digest-success.jsonl object.
	for _, u := range []string{"https://example.com/a", "https://example.com/b"} {
		path, err := LogDigestEntry(&types.DigestEntry{URL: u, Status: types.DigestSuccess}, opts)
		require.NoError(t, err)
		assert.Equal(t, "logs/digest-success.jsonl", path)
	}

	key := "logs/digest-success.jsonl"
	require.Len(t, store.puts, 2)
	assert.Equal(t, key, store.puts[0])
	assert.Equal(t, key, store.puts[1])

	data := string(store.objects[key])
	assert.Equal(t, 2, strings.Count(data, "\n"))
	assert.Contains(t, data, "https://example.com/a")
	assert.Contains(t, data, "https://example.com/b")
}

func TestWriteManualReviewEntryR2WritesUncat(t *testing.T) {
	store := newFakeDigestStore()
	opts := &WriteOptions{R2Store: store}

	path, err := WriteManualReviewEntry(&types.ClassifyItem{
		URL:         "https://example.com/needs-review",
		Title:       "review me",
		TopicPath:   "none",
		Type:        types.TypeInbox,
		RouteReason: types.RouteReasonNoTopicMatch,
		Summary:     &types.StructuredSummary{Overview: "summary"},
	}, opts)

	require.NoError(t, err)
	assert.Equal(t, store.urlBase+"/uncat.md", path)

	// uncat.md put plus its success-log put.
	require.Contains(t, store.puts, "uncat.md")
	content := string(store.objects["uncat.md"])
	assert.Contains(t, content, "### review me")
	assert.Contains(t, content, "---\ntitle: review me\n")
	assert.NotContains(t, content, "```")
}
