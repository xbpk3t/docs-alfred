package litter

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type putCall struct {
	bucket      string
	key         string
	content     string
	contentType string
}

type fakeStore struct {
	puts    []putCall
	objects map[string][]byte
	listErr error
	getErr  error
}

func newFakeStore() *fakeStore {
	return &fakeStore{objects: map[string][]byte{}}
}

func (f *fakeStore) Put(_ context.Context, bucket, key string, reader io.Reader, _ int64, contentType string) error {
	data, err := io.ReadAll(reader)
	if err != nil {
		return err
	}
	f.puts = append(f.puts, putCall{bucket: bucket, key: key, content: string(data), contentType: contentType})
	f.objects[key] = data

	return nil
}

func (f *fakeStore) List(_ context.Context, _, prefix string) ([]string, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	var keys []string
	for k := range f.objects {
		if strings.HasPrefix(k, prefix) {
			keys = append(keys, k)
		}
	}

	return keys, nil
}

func (f *fakeStore) Get(_ context.Context, _, key string) ([]byte, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}
	data, ok := f.objects[key]
	if !ok {
		return nil, errors.New("not found")
	}

	return data, nil
}

// publicServer serves HEAD with a fixed content type for any path.
func publicServer(t *testing.T, status int, contentType string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodHead {
			http.Error(w, "want HEAD", http.StatusMethodNotAllowed)

			return
		}
		if contentType != "" {
			w.Header().Set("Content-Type", contentType)
		}
		w.WriteHeader(status)
	}))
	t.Cleanup(srv.Close)

	return srv
}

func TestR2_Upload_Success(t *testing.T) {
	store := newFakeStore()
	srv := publicServer(t, http.StatusOK, "text/html; charset=utf-8")
	r := newR2WithStore(store, &R2Config{Bucket: "docs-wiki", PublicBaseURL: srv.URL})

	result, err := r.Upload(context.Background(), "trns-abc123.html", "<h1>hi</h1>")

	require.NoError(t, err)
	assert.Equal(t, srv.URL+"/trns-abc123.html", result.URL)
	assert.Equal(t, "r2", result.Driver)
	require.Len(t, store.puts, 1)
	assert.Equal(t, "docs-wiki", store.puts[0].bucket)
	assert.Equal(t, "trns-abc123.html", store.puts[0].key)
	assert.Equal(t, "<h1>hi</h1>", store.puts[0].content)
	assert.Equal(t, "text/html; charset=utf-8", store.puts[0].contentType)
}

func TestR2_Upload_KeyPrefix(t *testing.T) {
	store := newFakeStore()
	srv := publicServer(t, http.StatusOK, "text/markdown; charset=utf-8")
	r := newR2WithStore(store, &R2Config{Bucket: "b", PublicBaseURL: srv.URL + "/", KeyPrefix: "wiki"})

	result, err := r.Upload(context.Background(), "transcript-BV1xx.md", "# t")

	require.NoError(t, err)
	assert.Equal(t, srv.URL+"/wiki/transcript-BV1xx.md", result.URL)
	assert.Equal(t, "wiki/transcript-BV1xx.md", store.puts[0].key)
}

func TestR2_Upload_SelfCheckHTTPError(t *testing.T) {
	store := newFakeStore()
	srv := publicServer(t, http.StatusForbidden, "text/html")
	r := newR2WithStore(store, &R2Config{Bucket: "b", PublicBaseURL: srv.URL})

	_, err := r.Upload(context.Background(), "x.html", "<p>x</p>")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "self-check")
	assert.Contains(t, err.Error(), "403")
}

func TestR2_Upload_SelfCheckContentTypeMismatch(t *testing.T) {
	store := newFakeStore()
	srv := publicServer(t, http.StatusOK, "application/octet-stream")
	r := newR2WithStore(store, &R2Config{Bucket: "b", PublicBaseURL: srv.URL})

	_, err := r.Upload(context.Background(), "x.html", "<p>x</p>")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "content-type mismatch")
}

func TestR2_Upload_InvalidNames(t *testing.T) {
	store := newFakeStore()
	r := newR2WithStore(store, &R2Config{Bucket: "b", PublicBaseURL: "https://example.com"})

	for _, name := range []string{"a/b.html", "with space.html", "中文.html", ".hidden", "..", ""} {
		_, err := r.Upload(context.Background(), name, "x")
		assert.Error(t, err, "name %q should be rejected", name)
	}
	assert.Empty(t, store.puts)
}

func TestR2_ListKeysAndGetObject(t *testing.T) {
	store := newFakeStore()
	store.objects["index-2026-10-19.json"] = []byte(`[{"url":"u"}]`)
	store.objects["trns-x.html"] = []byte("x")
	r := newR2WithStore(store, &R2Config{Bucket: "b", PublicBaseURL: "https://example.com"})

	keys, err := r.ListKeys(context.Background(), "index-")
	require.NoError(t, err)
	assert.Equal(t, []string{"index-2026-10-19.json"}, keys)

	data, err := r.GetObject(context.Background(), "index-2026-10-19.json")
	require.NoError(t, err)
	assert.JSONEq(t, `[{"url":"u"}]`, string(data))
}

func TestNewR2_RequiresConfig(t *testing.T) {
	_, err := NewR2(&R2Config{})
	require.Error(t, err)
}

func TestContentTypeForName(t *testing.T) {
	tests := map[string]string{
		"a.html":  "text/html; charset=utf-8",
		"a.md":    "text/markdown; charset=utf-8",
		"a.json":  "application/json",
		"a.bin":   "application/octet-stream",
		"a.HTML":  "text/html; charset=utf-8",
		"noext":   "application/octet-stream",
	}
	for name, want := range tests {
		assert.Equal(t, want, contentTypeForName(name), "name %q", name)
	}
}

func TestNewFromConfig_R2(t *testing.T) {
	cfg := R2Config{
		Endpoint:        "https://acc.r2.cloudflarestorage.com",
		Bucket:          "docs-wiki",
		AccessKeyID:     "ak",
		SecretAccessKey: "sk",
		PublicBaseURL:   "https://dw.lucc.dev",
	}

	fallback, err := NewFromConfig([]string{"r2"}, "24h", &cfg)
	require.NoError(t, err)
	assert.Equal(t, "fallback", fallback.Name())

	_, err = NewFromConfig([]string{"r2"}, "24h", nil)
	require.Error(t, err)

	_, err = NewFromConfig(nil, "24h", &cfg)
	require.Error(t, err)

	fallback, err = NewFromConfig([]string{"zerox0", "fileio"}, "24h", nil)
	require.NoError(t, err)
	require.Len(t, fallback.uploaders, 2)
}
