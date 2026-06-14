package enrich

import (
	"context"
	"testing"
)

func TestIsChineseMedia(t *testing.T) {
	tests := []struct {
		name      string
		lang      string
		country   string
		want      bool
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
			if got := IsChineseMedia(tt.lang, tt.country); got != tt.want {
				t.Errorf("IsChineseMedia(%q, %q) = %v, want %v", tt.lang, tt.country, got, tt.want)
			}
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
			if got := extractYear(tt.date); got != tt.want {
				t.Errorf("extractYear(%q) = %q, want %q", tt.date, got, tt.want)
			}
		})
	}
}

func TestMapMovieFields(t *testing.T) {
	// Test with full detail
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

	if fields.PublishAt != "2023" {
		t.Errorf("PublishAt = %q, want 2023", fields.PublishAt)
	}
	if fields.Alias != "The Original Title" {
		t.Errorf("Alias = %q, want 'The Original Title'", fields.Alias)
	}
	if fields.Dict != "Director A、Director B" {
		t.Errorf("Dict = %q, want 'Director A、Director B'", fields.Dict)
	}
	if fields.Cast != "Actor One、Actor Two" {
		t.Errorf("Cast = %q, want 'Actor One、Actor Two'", fields.Cast)
	}
}

func TestMapMovieFieldsChineseLanguage(t *testing.T) {
	// Chinese-language film should NOT get alias
	search := &tmdbSearchItem{
		Title:            "中文片名",
		OriginalTitle:    "Chinese Original",
		OriginalLanguage: "zh",
	}
	fields := mapMovieFields(search, nil)
	if fields.Alias != "" {
		t.Errorf("Chinese film should not get alias, got %q", fields.Alias)
	}
	if fields.PublishAt != "" {
		t.Errorf("no date but got %q", fields.PublishAt)
	}
}

func TestMapMovieFieldsNoDetail(t *testing.T) {
	search := &tmdbSearchItem{
		Title:         "Local Title",
		OriginalTitle: "Original",
		ReleaseDate:   "2023-01-01",
	}
	fields := mapMovieFields(search, nil)
	if fields.PublishAt != "2023" {
		t.Errorf("PublishAt = %q, want 2023", fields.PublishAt)
	}
	if fields.Alias != "Original" {
		t.Errorf("Alias = %q, want Original", fields.Alias)
	}
	// No detail means no cast/dict
	if fields.Cast != "" {
		t.Errorf("Cast should be empty without detail, got %q", fields.Cast)
	}
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
	if fields.PublishAt != "2022" {
		t.Errorf("PublishAt = %q, want 2022", fields.PublishAt)
	}
	if fields.Alias != "Original TV Name" {
		t.Errorf("Alias = %q, want Original TV Name", fields.Alias)
	}
	if fields.Dict != "Creator One、Creator Two" {
		t.Errorf("Dict = %q, want 'Creator One、Creator Two'", fields.Dict)
	}
	if fields.Cast != "TV Actor A" {
		t.Errorf("Cast = %q, want 'TV Actor A'", fields.Cast)
	}
}

func TestMapGoogleBooksFields(t *testing.T) {
	info := &gbVolumeInfo{
		Title:         "Book Title",
		Subtitle:      "A Subtitle",
		Authors:       []string{"Author One", "Author Two"},
		PublishedDate: "2019",
	}
	fields := mapGoogleBooksFields(info)
	if fields.PublishAt != "2019" {
		t.Errorf("PublishAt = %q, want 2019", fields.PublishAt)
	}
	if fields.Author != "Author One、Author Two" {
		t.Errorf("Author = %q, want 'Author One、Author Two'", fields.Author)
	}
	if fields.Alias != "A Subtitle" {
		t.Errorf("Alias = %q, want 'A Subtitle'", fields.Alias)
	}
}

func TestMapGoogleBooksFieldsNoSubtitle(t *testing.T) {
	info := &gbVolumeInfo{
		Title:         "No Subtitle",
		Authors:       []string{"Solo Author"},
		PublishedDate: "2008-03",
	}
	fields := mapGoogleBooksFields(info)
	if fields.PublishAt != "2008" {
		t.Errorf("PublishAt = %q, want 2008", fields.PublishAt)
	}
	if fields.Alias != "" {
		t.Errorf("Alias should be empty without subtitle, got %q", fields.Alias)
	}
}

func TestMapGoogleBooksFieldsEmpty(t *testing.T) {
	info := &gbVolumeInfo{}
	fields := mapGoogleBooksFields(info)
	if fields.PublishAt != "" {
		t.Errorf("PublishAt should be empty, got %q", fields.PublishAt)
	}
	if fields.Author != "" {
		t.Errorf("Author should be empty, got %q", fields.Author)
	}
}

func TestMapOLFields(t *testing.T) {
	doc := &olDoc{
		Title:            "OL Book",
		AuthorName:       []string{"OL Author"},
		FirstPublishYear: 2005,
	}
	fields := mapOLFields(doc)
	if fields.PublishAt != "2005" {
		t.Errorf("PublishAt = %q, want 2005", fields.PublishAt)
	}
	if fields.Author != "OL Author" {
		t.Errorf("Author = %q, want 'OL Author'", fields.Author)
	}
}

func TestMapOLFieldsFromPublishYear(t *testing.T) {
	doc := &olDoc{
		Title:        "No First Year",
		AuthorName:   []string{"Author"},
		PublishYear:  []int{2010, 2012},
	}
	fields := mapOLFields(doc)
	if fields.PublishAt != "2010" {
		t.Errorf("PublishAt = %q, want 2010", fields.PublishAt)
	}
}

func TestEnricherFor(t *testing.T) {
	if e := EnricherFor(ResourceMovie, "key"); e == nil {
		t.Error("EnricherFor(movie) should not be nil")
	}
	if e := EnricherFor(ResourceTV, "key"); e == nil {
		t.Error("EnricherFor(tv) should not be nil")
	}
	if e := EnricherFor(ResourceBook, "key"); e == nil {
		t.Error("EnricherFor(book) should not be nil")
	}
	if e := EnricherFor(ResourceType("unknown"), "key"); e != nil {
		t.Error("EnricherFor(unknown) should be nil")
	}
}

func TestTMDBEnricherSetCache(t *testing.T) {
	movieEnricher := NewTMDBMovieEnricher("test-key")
	cache := NewCache("/tmp/test_enrich_cache.json")
	movieEnricher.SetCache(cache)
	if movieEnricher.cache != cache {
		t.Error("SetCache did not attach cache")
	}

	tvEnricher := NewTMDBTVEnricher("test-key")
	tvEnricher.SetCache(cache)
	if tvEnricher.cache != cache {
		t.Error("SetCache did not attach cache")
	}
}

func TestBooksEnricherSetCache(t *testing.T) {
	enricher := NewBooksEnricher("test-key")
	cache := NewCache("/tmp/test_enrich_cache_books.json")
	enricher.SetCache(cache)
	if enricher.cache != cache {
		t.Error("SetCache did not attach cache")
	}
}

func TestEnricherEnrichNoNetwork(t *testing.T) {
	// These calls will fail on network, but should return error, not panic
	movieEnricher := NewTMDBMovieEnricher("invalid-key")
	_, err := movieEnricher.Enrich(context.Background(), "Test Movie", "2023")
	if err == nil {
		t.Error("expected error with invalid API key")
	}

	bookEnricher := NewBooksEnricher("invalid-key")
	_, err = bookEnricher.Enrich(context.Background(), "Test Book", "2023")
	if err == nil {
		t.Error("expected error with invalid API key")
	}
}

func TestParseYAMLFileWithInvalidPath(t *testing.T) {
	_, _, err := ParseYAMLFile("/nonexistent/path.yml")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}
