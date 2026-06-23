package enrich

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGoogleBooksClient_Search(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/volumes", r.URL.Path)
		assert.Equal(t, "test-query", r.URL.Query().Get("q"))

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"items": [{
				"volumeInfo": {
					"title": "Test Book",
					"subtitle": "A Subtitle",
					"publishedDate": "2023-01-15",
					"publisher": "Test Publisher",
					"authors": ["Author A", "Author B"]
				}
			}]
		}`))
	}))
	t.Cleanup(server.Close)

	client := &googleBooksClient{
		http:   newTestHTTPClient(server.URL),
		apiKey: "test-key",
		rl:     newTestRateLimiter(),
	}

	info, err := client.search(context.Background(), "test-query")
	require.NoError(t, err)
	require.NotNil(t, info)
	assert.Equal(t, "Test Book", info.Title)
	assert.Equal(t, "A Subtitle", info.Subtitle)
	assert.Equal(t, []string{"Author A", "Author B"}, info.Authors)
}

func TestGoogleBooksClient_SearchEmpty(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"items": []}`))
	}))
	t.Cleanup(server.Close)

	client := &googleBooksClient{
		http:   newTestHTTPClient(server.URL),
		apiKey: "test-key",
		rl:     newTestRateLimiter(),
	}

	_, err := client.search(context.Background(), "no-results")
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestGoogleBooksClient_SearchHTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "server error", http.StatusInternalServerError)
	}))
	t.Cleanup(server.Close)

	client := &googleBooksClient{
		http:   newTestHTTPClient(server.URL),
		apiKey: "test-key",
		rl:     newTestRateLimiter(),
	}

	_, err := client.search(context.Background(), "test")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "google books")
}

func TestGoogleBooksClient_SearchDecodeError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`invalid json`))
	}))
	t.Cleanup(server.Close)

	client := &googleBooksClient{
		http:   newTestHTTPClient(server.URL),
		apiKey: "test-key",
		rl:     newTestRateLimiter(),
	}

	_, err := client.search(context.Background(), "test")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "decode")
}

func TestOpenLibraryClient_Search(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/search.json", r.URL.Path)

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"docs": [{
				"title": "OL Book",
				"author_name": ["OL Author"],
				"publish_year": [2020, 2021],
				"first_publish_year": 2019
			}]
		}`))
	}))
	t.Cleanup(server.Close)

	client := &openLibraryClient{
		http: newTestHTTPClient(server.URL),
	}

	doc, err := client.search(context.Background(), "test-query")
	require.NoError(t, err)
	require.NotNil(t, doc)
	assert.Equal(t, "OL Book", doc.Title)
	assert.Equal(t, []string{"OL Author"}, doc.AuthorName)
	assert.Equal(t, 2019, doc.FirstPublishYear)
}

func TestOpenLibraryClient_SearchEmpty(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"docs": []}`))
	}))
	t.Cleanup(server.Close)

	client := &openLibraryClient{
		http: newTestHTTPClient(server.URL),
	}

	_, err := client.search(context.Background(), "no-results")
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestOpenLibraryClient_SearchHTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "server error", http.StatusInternalServerError)
	}))
	t.Cleanup(server.Close)

	client := &openLibraryClient{
		http: newTestHTTPClient(server.URL),
	}

	_, err := client.search(context.Background(), "test")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "open library")
}

func TestOpenLibraryClient_SearchDecodeError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`invalid json`))
	}))
	t.Cleanup(server.Close)

	client := &openLibraryClient{
		http: newTestHTTPClient(server.URL),
	}

	_, err := client.search(context.Background(), "test")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "decode")
}

func TestBooksEnricher_Enrich_GoogleBooksSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"items": [{
				"volumeInfo": {
					"title": "Book",
					"authors": ["Author"],
					"publishedDate": "2020"
				}
			}]
		}`))
	}))
	t.Cleanup(server.Close)

	enricher := &BooksEnricher{
		google: &googleBooksClient{
			http:   newTestHTTPClient(server.URL),
			apiKey: "test",
			rl:     newTestRateLimiter(),
		},
		ol: &openLibraryClient{
			http: newTestHTTPClient("http://invalid"),
		},
	}

	fields, err := enricher.Enrich(context.Background(), "Book", "")
	require.NoError(t, err)
	require.NotNil(t, fields)
	assert.Equal(t, "Author", fields.Author)
	assert.Equal(t, "2020", fields.PublishAt)
}

func TestBooksEnricher_Enrich_GoogleNotFoundOLFallback(t *testing.T) {
	googleServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"items": []}`))
	}))
	t.Cleanup(googleServer.Close)

	olServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"docs": [{
				"title": "OL Book",
				"author_name": ["OL Author"],
				"first_publish_year": 2015
			}]
		}`))
	}))
	t.Cleanup(olServer.Close)

	enricher := &BooksEnricher{
		google: &googleBooksClient{
			http:   newTestHTTPClient(googleServer.URL),
			apiKey: "test",
			rl:     newTestRateLimiter(),
		},
		ol: &openLibraryClient{
			http: newTestHTTPClient(olServer.URL),
		},
	}

	fields, err := enricher.Enrich(context.Background(), "Book", "")
	require.NoError(t, err)
	require.NotNil(t, fields)
	assert.Equal(t, "OL Author", fields.Author)
	assert.Equal(t, "2015", fields.PublishAt)
}

func TestBooksEnricher_Enrich_BothNotFound(t *testing.T) {
	googleServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"items": []}`))
	}))
	t.Cleanup(googleServer.Close)

	olServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"docs": []}`))
	}))
	t.Cleanup(olServer.Close)

	enricher := &BooksEnricher{
		google: &googleBooksClient{
			http:   newTestHTTPClient(googleServer.URL),
			apiKey: "test",
			rl:     newTestRateLimiter(),
		},
		ol: &openLibraryClient{
			http: newTestHTTPClient(olServer.URL),
		},
	}

	_, err := enricher.Enrich(context.Background(), "Unknown Book", "")
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestBooksEnricher_Enrich_WithCache(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"items": [{
				"volumeInfo": {
					"title": "Cached Book",
					"authors": ["Cached Author"],
					"publishedDate": "2021"
				}
			}]
		}`))
	}))
	t.Cleanup(server.Close)

	cachePath := t.TempDir() + "/cache.json"
	cache := NewCache(cachePath)

	enricher := &BooksEnricher{
		google: &googleBooksClient{
			http:   newTestHTTPClient(server.URL),
			apiKey: "test",
			rl:     newTestRateLimiter(),
		},
		ol: &openLibraryClient{
			http: newTestHTTPClient("http://invalid"),
		},
		cache: cache,
	}

	// First call - hits API
	fields, err := enricher.Enrich(context.Background(), "Cached Book", "")
	require.NoError(t, err)
	assert.Equal(t, "Cached Author", fields.Author)
	assert.Equal(t, 1, callCount)

	// Second call - should use cache
	fields2, err := enricher.Enrich(context.Background(), "Cached Book", "")
	require.NoError(t, err)
	assert.Equal(t, "Cached Author", fields2.Author)
	assert.Equal(t, 1, callCount) // still 1
}

func TestBooksEnricher_Enrich_GoogleErrorReturnsError(t *testing.T) {
	googleServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "rate limited", http.StatusTooManyRequests)
	}))
	t.Cleanup(googleServer.Close)

	enricher := &BooksEnricher{
		google: &googleBooksClient{
			http:   newTestHTTPClient(googleServer.URL),
			apiKey: "test",
			rl:     newTestRateLimiter(),
		},
		ol: &openLibraryClient{
			http: newTestHTTPClient("http://invalid"),
		},
	}

	// When Google returns a non-ErrNotFound error, Enrich returns it directly
	_, err := enricher.Enrich(context.Background(), "Book", "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "rate limited")
}
