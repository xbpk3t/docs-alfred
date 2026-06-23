package enrich

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTMDBClient_SearchMovie(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/search/movie", r.URL.Path)
		assert.Equal(t, "Test Movie", r.URL.Query().Get("query"))
		assert.Equal(t, "2023", r.URL.Query().Get("year"))

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"results": [
				{
					"id": 1,
					"title": "Test Movie",
					"original_title": "Original Movie",
					"release_date": "2023-06-15",
					"original_language": "en",
					"popularity": 100.0
				},
				{
					"id": 2,
					"title": "Less Popular",
					"popularity": 50.0
				}
			]
		}`))
	}))
	t.Cleanup(server.Close)

	client := &tmdbClient{
		http:   newTestHTTPClient(server.URL),
		apiKey: "test-key",
	}

	item, err := client.searchMovie(context.Background(), "Test Movie", "2023")
	require.NoError(t, err)
	require.NotNil(t, item)
	assert.Equal(t, 1, item.ID)
	assert.Equal(t, "Test Movie", item.Title)
}

func TestTMDBClient_SearchMovie_MostPopularNotFirst(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"results": [
				{
					"id": 1,
					"title": "Less Popular",
					"popularity": 50.0
				},
				{
					"id": 2,
					"title": "Most Popular",
					"original_title": "Original Most",
					"release_date": "2024-01-01",
					"original_language": "en",
					"popularity": 200.0
				}
			]
		}`))
	}))
	t.Cleanup(server.Close)

	client := &tmdbClient{
		http:   newTestHTTPClient(server.URL),
		apiKey: "test-key",
	}

	item, err := client.searchMovie(context.Background(), "test", "")
	require.NoError(t, err)
	require.NotNil(t, item)
	assert.Equal(t, 2, item.ID, "should return the most popular result")
	assert.Equal(t, "Most Popular", item.Title)
}

func TestTMDBClient_SearchTV(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/search/tv", r.URL.Path)
		assert.Equal(t, "2022", r.URL.Query().Get("first_air_date_year"))

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"results": [{
				"id": 10,
				"name": "TV Show",
				"original_name": "Original TV",
				"first_air_date": "2022-09-01",
				"original_language": "en",
				"popularity": 80.0
			}]
		}`))
	}))
	t.Cleanup(server.Close)

	client := &tmdbClient{
		http:   newTestHTTPClient(server.URL),
		apiKey: "test-key",
	}

	item, err := client.searchTV(context.Background(), "TV Show", "2022")
	require.NoError(t, err)
	require.NotNil(t, item)
	assert.Equal(t, 10, item.ID)
	assert.Equal(t, "TV Show", item.Name)
}

func TestTMDBClient_SearchEmpty(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"results": []}`))
	}))
	t.Cleanup(server.Close)

	client := &tmdbClient{
		http:   newTestHTTPClient(server.URL),
		apiKey: "test-key",
	}

	_, err := client.searchMovie(context.Background(), "nonexistent", "")
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestTMDBClient_SearchHTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
	}))
	t.Cleanup(server.Close)

	client := &tmdbClient{
		http:   newTestHTTPClient(server.URL),
		apiKey: "bad-key",
	}

	_, err := client.searchMovie(context.Background(), "test", "")
	require.Error(t, err)
}

func TestTMDBClient_SearchDecodeError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`invalid json`))
	}))
	t.Cleanup(server.Close)

	client := &tmdbClient{
		http:   newTestHTTPClient(server.URL),
		apiKey: "test-key",
	}

	_, err := client.searchMovie(context.Background(), "test", "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "decode")
}

func TestTMDBMovieEnricher_Enrich(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/search/movie", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"results": [{
				"id": 1,
				"title": "Movie",
				"original_title": "Original Movie",
				"release_date": "2023-01-01",
				"original_language": "en",
				"popularity": 100.0
			}]
		}`))
	})
	mux.HandleFunc("/movie/1", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"original_title": "Detail Original",
			"release_date": "2023-06-15",
			"original_language": "en",
			"credits": {
				"cast": [{"name": "Actor 1", "order": 0}, {"name": "Actor 2", "order": 1}],
				"crew": [{"name": "Director X", "job": "Director"}]
			}
		}`))
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	enricher := &TMDBMovieEnricher{
		client: &tmdbClient{
			http:   newTestHTTPClient(server.URL),
			apiKey: "test-key",
		},
	}

	fields, err := enricher.Enrich(context.Background(), "Movie", "2023")
	require.NoError(t, err)
	require.NotNil(t, fields)
	assert.Equal(t, "2023", fields.PublishAt)
	assert.Equal(t, "Detail Original", fields.Alias)
	assert.Equal(t, "Director X", fields.Dict)
	assert.Equal(t, "Actor 1、Actor 2", fields.Cast)
}

func TestTMDBMovieEnricher_Enrich_SearchError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "server error", http.StatusInternalServerError)
	}))
	t.Cleanup(server.Close)

	enricher := &TMDBMovieEnricher{
		client: &tmdbClient{
			http:   newTestHTTPClient(server.URL),
			apiKey: "test-key",
		},
	}

	_, err := enricher.Enrich(context.Background(), "Movie", "")
	require.Error(t, err)
}

func TestTMDBMovieEnricher_Enrich_DetailError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/search/movie", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"results": [{
				"id": 1,
				"title": "Movie",
				"original_title": "Original",
				"release_date": "2023-01-01",
				"original_language": "en",
				"popularity": 100.0
			}]
		}`))
	})
	mux.HandleFunc("/movie/1", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "not found", http.StatusNotFound)
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	enricher := &TMDBMovieEnricher{
		client: &tmdbClient{
			http:   newTestHTTPClient(server.URL),
			apiKey: "test-key",
		},
	}

	fields, err := enricher.Enrich(context.Background(), "Movie", "2023")
	// Should fallback to search result fields
	require.NoError(t, err)
	require.NotNil(t, fields)
	assert.Equal(t, "2023", fields.PublishAt)
}

func TestTMDBTVEnricher_Enrich(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/search/tv", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"results": [{
				"id": 10,
				"name": "TV Show",
				"original_name": "Original TV",
				"first_air_date": "2022-09-01",
				"original_language": "en",
				"popularity": 80.0
			}]
		}`))
	})
	mux.HandleFunc("/tv/10", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"original_name": "Detail TV",
			"first_air_date": "2022-10-15",
			"original_language": "en",
			"created_by": [{"name": "Creator A"}],
			"credits": {
				"cast": [{"name": "TV Actor", "order": 0}]
			}
		}`))
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	enricher := &TMDBTVEnricher{
		client: &tmdbClient{
			http:   newTestHTTPClient(server.URL),
			apiKey: "test-key",
		},
	}

	fields, err := enricher.Enrich(context.Background(), "TV Show", "2022")
	require.NoError(t, err)
	require.NotNil(t, fields)
	assert.Equal(t, "2022", fields.PublishAt)
	assert.Equal(t, "Detail TV", fields.Alias)
	assert.Equal(t, "Creator A", fields.Dict)
	assert.Equal(t, "TV Actor", fields.Cast)
}

func TestTMDBTVEnricher_Enrich_SearchError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "server error", http.StatusInternalServerError)
	}))
	t.Cleanup(server.Close)

	enricher := &TMDBTVEnricher{
		client: &tmdbClient{
			http:   newTestHTTPClient(server.URL),
			apiKey: "test-key",
		},
	}

	_, err := enricher.Enrich(context.Background(), "TV Show", "")
	require.Error(t, err)
}

func TestTMDBTVEnricher_Enrich_DetailError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/search/tv", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"results": [{
				"id": 10,
				"name": "TV Show",
				"original_name": "Original TV",
				"first_air_date": "2022-09-01",
				"original_language": "en",
				"popularity": 80.0
			}]
		}`))
	})
	mux.HandleFunc("/tv/10", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "not found", http.StatusNotFound)
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	enricher := &TMDBTVEnricher{
		client: &tmdbClient{
			http:   newTestHTTPClient(server.URL),
			apiKey: "test-key",
		},
	}

	fields, err := enricher.Enrich(context.Background(), "TV Show", "2022")
	require.NoError(t, err)
	require.NotNil(t, fields)
	assert.Equal(t, "2022", fields.PublishAt)
}

func TestTMDBClient_SearchWithCache(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"results": [{
				"id": 1,
				"title": "Cached Movie",
				"popularity": 100.0
			}]
		}`))
	}))
	t.Cleanup(server.Close)

	cachePath := t.TempDir() + "/tmdb_cache.json"
	cache := NewCache(cachePath)

	client := &tmdbClient{
		http:   newTestHTTPClient(server.URL),
		apiKey: "test-key",
	}

	// First call
	item, err := client.searchWithCache(context.Background(), "movie", "Cached Movie", "2023", cache, client.searchMovie)
	require.NoError(t, err)
	assert.Equal(t, 1, callCount)

	// Second call should use cache
	item2, err := client.searchWithCache(context.Background(), "movie", "Cached Movie", "2023", cache, client.searchMovie)
	require.NoError(t, err)
	assert.Equal(t, 1, callCount) // still 1
	assert.Equal(t, item.ID, item2.ID)
}

func TestTMDBClient_SearchWithCache_YearRetry(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/json")
		if callCount == 1 {
			// First call with year - no results
			_, _ = w.Write([]byte(`{"results": []}`))
			return
		}
		// Second call without year - has results
		_, _ = w.Write([]byte(`{
			"results": [{
				"id": 2,
				"title": "Movie",
				"popularity": 50.0
			}]
		}`))
	}))
	t.Cleanup(server.Close)

	client := &tmdbClient{
		http:   newTestHTTPClient(server.URL),
		apiKey: "test-key",
	}

	item, err := client.searchWithCache(context.Background(), "movie", "Movie", "2023", nil, client.searchMovie)
	require.NoError(t, err)
	require.NotNil(t, item)
	assert.Equal(t, 2, item.ID)
	assert.Equal(t, 2, callCount)
}

func TestTMDBClient_SearchWithCache_NoCache(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"results": [{
				"id": 1,
				"title": "Movie",
				"popularity": 50.0
			}]
		}`))
	}))
	t.Cleanup(server.Close)

	client := &tmdbClient{
		http:   newTestHTTPClient(server.URL),
		apiKey: "test-key",
	}

	item, err := client.searchWithCache(context.Background(), "movie", "Movie", "", nil, client.searchMovie)
	require.NoError(t, err)
	require.NotNil(t, item)
	assert.Equal(t, 1, item.ID)
}

func TestGetDetail_Movie(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/movie/42", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"original_title": "Detail Title",
			"release_date": "2023-06-15",
			"original_language": "en",
			"credits": {
				"cast": [{"name": "Actor"}],
				"crew": [{"name": "Director", "job": "Director"}]
			}
		}`))
	}))
	t.Cleanup(server.Close)

	client := &tmdbClient{
		http:   newTestHTTPClient(server.URL),
		apiKey: "test-key",
	}

	detail, err := getDetail[tmdbMovieDetail](client, context.Background(), 42, "movie")
	require.NoError(t, err)
	require.NotNil(t, detail)
	assert.Equal(t, "Detail Title", detail.OriginalTitle)
	assert.Equal(t, "2023-06-15", detail.ReleaseDate)
}

func TestGetDetail_TV(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/tv/99", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"original_name": "TV Detail",
			"first_air_date": "2022-01-01",
			"original_language": "en",
			"created_by": [{"name": "Creator"}],
			"credits": {"cast": []}
		}`))
	}))
	t.Cleanup(server.Close)

	client := &tmdbClient{
		http:   newTestHTTPClient(server.URL),
		apiKey: "test-key",
	}

	detail, err := getDetail[tmdbTvDetail](client, context.Background(), 99, "tv")
	require.NoError(t, err)
	require.NotNil(t, detail)
	assert.Equal(t, "TV Detail", detail.OriginalName)
}

func TestGetDetail_HTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "not found", http.StatusNotFound)
	}))
	t.Cleanup(server.Close)

	client := &tmdbClient{
		http:   newTestHTTPClient(server.URL),
		apiKey: "test-key",
	}

	_, err := getDetail[tmdbMovieDetail](client, context.Background(), 999, "movie")
	require.Error(t, err)
}

func TestGetDetail_DecodeError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`invalid json`))
	}))
	t.Cleanup(server.Close)

	client := &tmdbClient{
		http:   newTestHTTPClient(server.URL),
		apiKey: "test-key",
	}

	_, err := getDetail[tmdbMovieDetail](client, context.Background(), 1, "movie")
	require.Error(t, err)
}

func TestTMDBMovieEnricher_Enrich_WithCache(t *testing.T) {
	callCount := 0
	mux := http.NewServeMux()
	mux.HandleFunc("/search/movie", func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"results": [{
				"id": 1,
				"title": "Movie",
				"popularity": 100.0
			}]
		}`))
	})
	mux.HandleFunc("/movie/1", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"original_title": "Original",
			"release_date": "2023-01-01",
			"original_language": "en",
			"credits": {}
		}`))
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	cache := NewCache(t.TempDir() + "/cache.json")
	enricher := &TMDBMovieEnricher{
		client: &tmdbClient{
			http:   newTestHTTPClient(server.URL),
			apiKey: "test-key",
		},
		cache: cache,
	}

	// First call
	_, err := enricher.Enrich(context.Background(), "Movie", "2023")
	require.NoError(t, err)
	assert.Equal(t, 1, callCount)

	// Second call uses cache for search
	_, err = enricher.Enrich(context.Background(), "Movie", "2023")
	require.NoError(t, err)
	assert.Equal(t, 1, callCount)
}

func TestTMDBTVEnricher_Enrich_WithCache(t *testing.T) {
	callCount := 0
	mux := http.NewServeMux()
	mux.HandleFunc("/search/tv", func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"results": [{
				"id": 10,
				"name": "Show",
				"popularity": 80.0
			}]
		}`))
	})
	mux.HandleFunc("/tv/10", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"original_name": "Original",
			"first_air_date": "2022-01-01",
			"original_language": "en",
			"created_by": [],
			"credits": {}
		}`))
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	cache := NewCache(t.TempDir() + "/cache.json")
	enricher := &TMDBTVEnricher{
		client: &tmdbClient{
			http:   newTestHTTPClient(server.URL),
			apiKey: "test-key",
		},
		cache: cache,
	}

	// First call
	_, err := enricher.Enrich(context.Background(), "Show", "2022")
	require.NoError(t, err)
	assert.Equal(t, 1, callCount)

	// Second call uses cache
	_, err = enricher.Enrich(context.Background(), "Show", "2022")
	require.NoError(t, err)
	assert.Equal(t, 1, callCount)
}
