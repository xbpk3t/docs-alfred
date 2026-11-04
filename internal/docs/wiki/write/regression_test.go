package write

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xbpk3t/docs-alfred/internal/docs/wiki/types"
)

// reviewR2 is a minimal DigestStore whose uncat.md reads rendezvous, so two
// writers both observe the same pre-write content before either uploads.
type reviewR2 struct {
	mu       sync.Mutex
	data     map[string]string
	reads    int32
	blockKey string
	release  chan struct{}
}

func newReviewR2(blockKey string) *reviewR2 {
	return &reviewR2{data: map[string]string{}, blockKey: blockKey, release: make(chan struct{})}
}

func (s *reviewR2) GetObject(_ context.Context, key string) ([]byte, error) {
	s.mu.Lock()
	snapshot := s.data[key]
	s.mu.Unlock()
	if key == s.blockKey {
		if atomic.AddInt32(&s.reads, 1) == 2 {
			close(s.release)
		} else {
			// If a process-local lock serializes writers, the second reader never
			// arrives; do not deadlock the locked (correct) implementation.
			select {
			case <-s.release:
			case <-time.After(150 * time.Millisecond):
			}
		}
	}

	return []byte(snapshot), nil
}

func (s *reviewR2) Upload(_ context.Context, key, content string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[key] = content

	return "https://r2.example/" + key, nil
}

func (s *reviewR2) content(key string) string {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.data[key]
}

// WriteManualReviewEntry's R2 path is an unlocked read-merge-write. Two
// concurrent workers both read the old uncat.md and overwrite each other.
func TestReviewR2ManualReviewConcurrentWritersDoNotLoseEntries(t *testing.T) {
	store := newReviewR2("uncat.md")
	opts := &WriteOptions{WikiRoot: t.TempDir(), R2Store: store}

	var wg sync.WaitGroup
	for _, u := range []string{"https://example.com/one", "https://example.com/two"} {
		wg.Add(1)
		go func(url string) {
			defer wg.Done()
			_, err := WriteManualReviewEntry(&types.ClassifyItem{
				URL:   url,
				Title: url,
			}, opts)
			assert.NoError(t, err)
		}(u)
	}
	wg.Wait()

	merged := store.content("uncat.md")
	require.NotEmpty(t, merged)
	assert.Contains(t, merged, "https://example.com/one")
	assert.Contains(t, merged, "https://example.com/two")
}

// The same unlocked read-merge-write pattern is used for R2 digest logs.
// Losing success lines makes history dedup re-digest URLs and loses failure
// diagnostics.
func TestReviewR2DigestLogConcurrentWritersDoNotLoseLines(t *testing.T) {
	store := newReviewR2("logs/digest-success.jsonl")
	opts := &WriteOptions{WikiRoot: t.TempDir(), R2Store: store}

	var wg sync.WaitGroup
	for _, u := range []string{"https://example.com/one", "https://example.com/two"} {
		wg.Add(1)
		go func(url string) {
			defer wg.Done()
			_, err := LogDigestEntry(&types.DigestEntry{
				URL:    url,
				Stage:  types.StageWrite,
				Status: types.DigestSuccess,
			}, opts)
			assert.NoError(t, err)
		}(u)
	}
	wg.Wait()

	merged := store.content("logs/digest-success.jsonl")
	require.NotEmpty(t, merged)
	assert.Contains(t, merged, "https://example.com/one")
	assert.Contains(t, merged, "https://example.com/two")
}

// Local uncat dedup must be idempotent for the same URL.
func TestReviewLocalManualReviewDedup(t *testing.T) {
	opts := &WriteOptions{WikiRoot: t.TempDir()}
	item := &types.ClassifyItem{URL: "https://example.com/one", Title: "one"}

	_, err := WriteManualReviewEntry(item, opts)
	require.NoError(t, err)
	_, err = WriteManualReviewEntry(item, opts)
	require.NoError(t, err)

	data, err := os.ReadFile(filepath.Join(opts.WikiRoot, "uncat.md"))
	require.NoError(t, err)
	assert.Equal(t, 1, strings.Count(string(data), "URL: https://example.com/one"))
}

// The R2 uncat queue must also be idempotent for a repeated URL.
func TestReviewR2ManualReviewDedup(t *testing.T) {
	store := newReviewR2("")
	opts := &WriteOptions{WikiRoot: t.TempDir(), R2Store: store}
	item := &types.ClassifyItem{URL: "https://example.com/one", Title: "one"}

	_, err := WriteManualReviewEntry(item, opts)
	require.NoError(t, err)
	_, err = WriteManualReviewEntry(item, opts)
	require.NoError(t, err)

	assert.Equal(t, 1, strings.Count(store.content("uncat.md"), "URL: https://example.com/one"))
}

// When the topic catalog is unavailable, ValidTopicPaths is nil and
// routeInvalidTopicPath is a no-op. An AI-supplied topic path must still not
// escape the wiki root.
func TestReviewSummaryTopicPathCannotEscapeWikiRoot(t *testing.T) {
	parent := t.TempDir()
	root := filepath.Join(parent, "wiki")
	require.NoError(t, os.MkdirAll(root, 0o700))
	escaped := filepath.Join(parent, "escape", "summary.md")
	t.Cleanup(func() { _ = os.RemoveAll(filepath.Join(parent, "escape")) })

	item := &types.ClassifyItem{
		URL:       "https://example.com/a",
		Title:     "escape",
		TopicPath: "../escape",
		Summary:   &types.StructuredSummary{Overview: "real content"},
	}
	_, err := WriteSummary(item, &WriteOptions{WikiRoot: root})
	if err == nil {
		_, statErr := os.Stat(escaped)
		assert.True(t, os.IsNotExist(statErr), "summary write escaped the wiki root")
	}
}
