package enrich

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsChineseMedia(t *testing.T) {
	tests := []struct {
		name    string
		lang    string
		country string
		want    bool
	}{
		{"Chinese language zh", "zh", "", true},
		{"Chinese language cn", "cn", "", true},
		{"Chinese language tw", "tw", "", true},
		{"English language", "en", "", false},
		{"Japanese language", "ja", "", false},
		{"China in country", "en", "中国", true},
		{"Taiwan in country", "en", "Taiwan", true},
		{"Non-chinese country", "en", "US", false},
		{"Empty", "", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, IsChineseMedia(tt.lang, tt.country))
		})
	}
}

func TestExtractYear(t *testing.T) {
	tests := []struct {
		date string
		want string
	}{
		{"2023-01-15", "2023"},
		{"2000", "2000"},
		{"1999-12-31", "1999"},
		{"invalid", ""},
		{"", ""},
		{"18", ""},   // too short
		{"1000", ""}, // out of range
	}
	for _, tt := range tests {
		t.Run(tt.date, func(t *testing.T) {
			assert.Equal(t, tt.want, extractYear(tt.date))
		})
	}
}

func TestMapMovieFields(t *testing.T) {
	detail := &tmdbMovieDetail{
		OriginalTitle:    "The Original Title",
		ReleaseDate:      "2023-06-15",
		OriginalLanguage: "en",
		Credits: tmdbCredits{
			Cast: []tmdbCast{
				{Name: "Actor One", Order: 0},
				{Name: "Actor Two", Order: 1},
				{Name: "Actor Three", Order: 2},
			},
			Crew: []tmdbCrew{
				{Name: "Director A", Job: "Director"},
				{Name: "Director B", Job: "Director"},
				{Name: "Producer X", Job: "Producer"},
			},
		},
	}

	search := &tmdbSearchItem{
		Title:            "Local Title",
		OriginalTitle:    "The Original Title",
		ReleaseDate:      "2023-06-15",
		OriginalLanguage: "en",
	}

	fields := mapMovieFields(search, detail)

	assert.Equal(t, "2023", fields.PublishAt)
	assert.Equal(t, "The Original Title", fields.Alias)
	assert.Equal(t, "Director A、Director B", fields.Dict)
	assert.Equal(t, "Actor One、Actor Two", fields.Cast)
}

func TestMapMovieFieldsChineseLanguage(t *testing.T) {
	search := &tmdbSearchItem{
		Title:            "中文片名",
		OriginalTitle:    "Chinese Original",
		OriginalLanguage: "zh",
	}
	fields := mapMovieFields(search, nil)
	assert.Empty(t, fields.Alias, "Chinese film should not get alias")
	assert.Empty(t, fields.PublishAt)
}

func TestMapMovieFieldsNoDetail(t *testing.T) {
	search := &tmdbSearchItem{
		Title:         "Local Title",
		OriginalTitle: "Original",
		ReleaseDate:   "2023-01-01",
	}
	fields := mapMovieFields(search, nil)
	assert.Equal(t, "2023", fields.PublishAt)
	assert.Equal(t, "Original", fields.Alias)
	assert.Empty(t, fields.Cast, "Cast should be empty without detail")
}

func TestMapTvFields(t *testing.T) {
	detail := &tmdbTvDetail{
		OriginalName:     "Original TV Name",
		FirstAirDate:     "2022-09-01",
		OriginalLanguage: "en",
		CreatedBy: []tmdbCreator{
			{Name: "Creator One"},
			{Name: "Creator Two"},
		},
		Credits: tmdbCredits{
			Cast: []tmdbCast{
				{Name: "TV Actor A", Order: 0},
			},
		},
	}

	search := &tmdbSearchItem{
		Name:             "TV Show",
		OriginalName:     "Original TV Name",
		FirstAirDate:     "2022-09-01",
		OriginalLanguage: "en",
	}

	fields := mapTvFields(search, detail)
	assert.Equal(t, "2022", fields.PublishAt)
	assert.Equal(t, "Original TV Name", fields.Alias)
	assert.Equal(t, "Creator One、Creator Two", fields.Dict)
	assert.Equal(t, "TV Actor A", fields.Cast)
}

func TestMapGoogleBooksFields(t *testing.T) {
	info := &gbVolumeInfo{
		Title:         "Book Title",
		Subtitle:      "A Subtitle",
		Authors:       []string{"Author One", "Author Two"},
		PublishedDate: "2019",
	}
	fields := mapGoogleBooksFields(info)
	assert.Equal(t, "2019", fields.PublishAt)
	assert.Equal(t, "Author One、Author Two", fields.Author)
	assert.Equal(t, "A Subtitle", fields.Alias)
}

func TestMapGoogleBooksFieldsNoSubtitle(t *testing.T) {
	info := &gbVolumeInfo{
		Title:         "No Subtitle",
		Authors:       []string{"Solo Author"},
		PublishedDate: "2008-03",
	}
	fields := mapGoogleBooksFields(info)
	assert.Equal(t, "2008", fields.PublishAt)
	assert.Empty(t, fields.Alias, "Alias should be empty without subtitle")
}

func TestMapGoogleBooksFieldsEmpty(t *testing.T) {
	info := &gbVolumeInfo{}
	fields := mapGoogleBooksFields(info)
	assert.Empty(t, fields.PublishAt)
	assert.Empty(t, fields.Author)
}

func TestMapOLFields(t *testing.T) {
	doc := &olDoc{
		Title:            "OL Book",
		AuthorName:       []string{"OL Author"},
		FirstPublishYear: 2005,
	}
	fields := mapOLFields(doc)
	assert.Equal(t, "2005", fields.PublishAt)
	assert.Equal(t, "OL Author", fields.Author)
}

func TestMapOLFieldsFromPublishYear(t *testing.T) {
	doc := &olDoc{
		Title:       "No First Year",
		AuthorName:  []string{"Author"},
		PublishYear: []int{2010, 2012},
	}
	fields := mapOLFields(doc)
	assert.Equal(t, "2010", fields.PublishAt)
}

func TestEnricherFor(t *testing.T) {
	require.NotNil(t, EnricherFor(ResourceMovie, "key"), "EnricherFor(movie) should not be nil")
	require.NotNil(t, EnricherFor(ResourceTV, "key"), "EnricherFor(tv) should not be nil")
	require.NotNil(t, EnricherFor(ResourceBook, "key"), "EnricherFor(book) should not be nil")
	require.Nil(t, EnricherFor(ResourceType("unknown"), "key"), "EnricherFor(unknown) should be nil")
}

func TestTMDBEnricherSetCache(t *testing.T) {
	movieEnricher := NewTMDBMovieEnricher("test-key")
	cache := NewCache("/tmp/test_enrich_cache.json")
	movieEnricher.SetCache(cache)
	require.Equal(t, cache, movieEnricher.cache, "SetCache did not attach cache")

	tvEnricher := NewTMDBTVEnricher("test-key")
	tvEnricher.SetCache(cache)
	require.Equal(t, cache, tvEnricher.cache, "SetCache did not attach cache")
}

func TestBooksEnricherSetCache(t *testing.T) {
	enricher := NewBooksEnricher("test-key")
	cache := NewCache("/tmp/test_enrich_cache_books.json")
	enricher.SetCache(cache)
	require.Equal(t, cache, enricher.cache, "SetCache did not attach cache")
}

func TestEnricherEnrichNoNetwork(t *testing.T) {
	movieEnricher := NewTMDBMovieEnricher("invalid-key")
	_, err := movieEnricher.Enrich(context.Background(), "Test Movie", "2023")
	require.Error(t, err, "expected error with invalid API key")

	bookEnricher := NewBooksEnricher("invalid-key")
	_, err = bookEnricher.Enrich(context.Background(), "Test Book", "2023")
	require.Error(t, err, "expected error with invalid API key")
}

func TestParseYAMLFileWithInvalidPath(t *testing.T) {
	_, _, err := ParseYAMLFile("/nonexistent/path.yml")
	require.Error(t, err, "expected error for nonexistent file")
}
