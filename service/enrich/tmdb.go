package enrich

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/go-resty/resty/v2"
)

const tmdbBaseURL = "https://api.themoviedb.org/3"

// --- TMDB API response types ---

type tmdbSearchResult struct {
	Results []tmdbSearchItem `json:"results"`
}

type tmdbSearchItem struct {
	Title            string  `json:"title,omitempty"`
	Name             string  `json:"name,omitempty"`
	OriginalTitle    string  `json:"original_title,omitempty"`
	OriginalName     string  `json:"original_name,omitempty"`
	ReleaseDate      string  `json:"release_date,omitempty"`
	FirstAirDate     string  `json:"first_air_date,omitempty"`
	OriginalLanguage string  `json:"original_language"`
	ID               int     `json:"id"`
	Popularity       float64 `json:"popularity"`
}

type tmdbMovieDetail struct {
	OriginalTitle    string      `json:"original_title"`
	ReleaseDate      string      `json:"release_date"`
	OriginalLanguage string      `json:"original_language"`
	Credits          tmdbCredits `json:"credits"`
}

type tmdbTvDetail struct {
	OriginalName     string        `json:"original_name"`
	FirstAirDate     string        `json:"first_air_date"`
	OriginalLanguage string        `json:"original_language"`
	CreatedBy        []tmdbCreator `json:"created_by"`
	Credits          tmdbCredits   `json:"credits"`
}

type tmdbCredits struct {
	Cast []tmdbCast `json:"cast"`
	Crew []tmdbCrew `json:"crew"`
}

type tmdbCast struct {
	Name      string `json:"name"`
	Character string `json:"character"`
	Order     int    `json:"order"`
}

type tmdbCrew struct {
	Name string `json:"name"`
	Job  string `json:"job"`
}

type tmdbCreator struct {
	Name string `json:"name"`
}

// --- TMDB Client ---

type tmdbClient struct {
	http   *resty.Client
	apiKey string
}

func newTMDBClient(apiKey string) *tmdbClient {
	return &tmdbClient{
		apiKey: apiKey,
		http:   resty.New().SetBaseURL(tmdbBaseURL),
	}
}

func (c *tmdbClient) searchMovie(ctx context.Context, query, year string) (*tmdbSearchItem, error) {
	params := map[string]string{
		"query":         query,
		"api_key":       c.apiKey,
		"language":      "zh-CN",
		"include_adult": "false",
	}
	if year != "" {
		params["year"] = year
	}

	return c.search(ctx, "/search/movie", params)
}

func (c *tmdbClient) searchTV(ctx context.Context, query, year string) (*tmdbSearchItem, error) {
	params := map[string]string{
		"query":         query,
		"api_key":       c.apiKey,
		"language":      "zh-CN",
		"include_adult": "false",
	}
	if year != "" {
		params["first_air_date_year"] = year
	}

	return c.search(ctx, "/search/tv", params)
}

func (c *tmdbClient) search(ctx context.Context, endpoint string, params map[string]string) (*tmdbSearchItem, error) {
	resp, err := c.http.R().
		SetContext(ctx).
		SetQueryParams(params).
		Get(endpoint)
	if err != nil {
		return nil, fmt.Errorf("tmdb request: %w", err)
	}
	if resp.IsError() {
		return nil, fmt.Errorf("tmdb %s: %s", endpoint, resp.String())
	}

	var result tmdbSearchResult
	if err := json.Unmarshal(resp.Body(), &result); err != nil {
		return nil, fmt.Errorf("tmdb decode: %w", err)
	}
	if len(result.Results) == 0 {
		return nil, ErrNotFound
	}
	// Return the most popular result
	best := &result.Results[0]
	for i := 1; i < len(result.Results); i++ {
		if result.Results[i].Popularity > best.Popularity {
			best = &result.Results[i]
		}
	}

	return best, nil
}

// getDetail fetches movie or TV detail by ID, using generics to handle both types.
func getDetail[T tmdbMovieDetail | tmdbTvDetail](c *tmdbClient, ctx context.Context, id int, mediaType string) (*T, error) {
	resp, err := c.http.R().
		SetContext(ctx).
		SetQueryParam("api_key", c.apiKey).
		SetQueryParam("append_to_response", "credits").
		SetQueryParam("language", "zh-CN").
		Get(fmt.Sprintf("/%s/%d", mediaType, id))
	if err != nil {
		return nil, fmt.Errorf("tmdb %s detail: %w", mediaType, err)
	}
	if resp.IsError() {
		return nil, fmt.Errorf("tmdb %s detail %d: %s", mediaType, id, resp.String())
	}
	var detail T
	if err := json.Unmarshal(resp.Body(), &detail); err != nil {
		return nil, fmt.Errorf("tmdb decode detail: %w", err)
	}

	return &detail, nil
}

//nolint:cyclop
// searchWithCache wraps a TMDB search with cache lookup/store and year retry.
func (c *tmdbClient) searchWithCache(
	ctx context.Context,
	cachePrefix, name, year string,
	cache *Cache,
	searchFn func(ctx context.Context, name, year string) (*tmdbSearchItem, error),
) (*tmdbSearchItem, error) {
	cacheKey := fmt.Sprintf("%s:%s|%s", cachePrefix, name, year)
	if cache != nil {
		if cached := cache.Get(cacheKey); cached != nil {
			var item tmdbSearchItem
			if err := json.Unmarshal(cached, &item); err == nil {
				return &item, nil
			}
		}
	}

	item, err := searchFn(ctx, name, year)
	if err != nil && errors.Is(err, ErrNotFound) && year != "" {
		// Retry without year
		item, err = searchFn(ctx, name, "")
		if err != nil {
			return nil, err
		}
	} else if err != nil {
		return nil, err
	}

	if cache != nil {
		if data, err := json.Marshal(item); err == nil {
			cache.Set(cacheKey, data)
		}
	}

	return item, nil
}

// --- TMDB Movie Enricher ---

// TMDBMovieEnricher enriches movie entries using the TMDB API.
type TMDBMovieEnricher struct {
	client *tmdbClient
	cache  *Cache
}

// NewTMDBMovieEnricher creates a movie enricher with the given TMDB API key.
func NewTMDBMovieEnricher(apiKey string) *TMDBMovieEnricher {
	return &TMDBMovieEnricher{
		client: newTMDBClient(apiKey),
	}
}

// SetCache attaches a shared cache to this enricher.
func (e *TMDBMovieEnricher) SetCache(c *Cache) { e.cache = c }

// Enrich searches TMDB for a movie and returns fields to set.
func (e *TMDBMovieEnricher) Enrich(ctx context.Context, name, publishAt string) (*EnrichFields, error) {
	searchResult, err := e.searchWithCache(ctx, name, publishAt)
	if err != nil {
		return nil, err
	}

	detail, err := getDetail[tmdbMovieDetail](e.client, ctx, searchResult.ID, "movie")
	if err != nil {
		slog.Warn("failed to get movie detail, using search result", "id", searchResult.ID, "error", err)

		return mapMovieFields(searchResult, nil), nil
	}

	return mapMovieFields(searchResult, detail), nil
}

func (e *TMDBMovieEnricher) searchWithCache(ctx context.Context, name, year string) (*tmdbSearchItem, error) {
	return e.client.searchWithCache(ctx, "movie", name, year, e.cache, e.client.searchMovie)
}

//nolint:gocyclo,cyclop
func mapMovieFields(search *tmdbSearchItem, detail *tmdbMovieDetail) *EnrichFields {
	fields := &EnrichFields{}

	// publishAt from release_date
	if detail != nil && detail.ReleaseDate != "" {
		fields.PublishAt = extractYear(detail.ReleaseDate)
	} else if search.ReleaseDate != "" {
		fields.PublishAt = extractYear(search.ReleaseDate)
	}

	// Alias: original_title if it's different from the local title
	originalTitle := search.OriginalTitle
	if detail != nil && detail.OriginalTitle != "" {
		originalTitle = detail.OriginalTitle
	}
	originalLang := search.OriginalLanguage
	if detail != nil && detail.OriginalLanguage != "" {
		originalLang = detail.OriginalLanguage
	}
	if originalTitle != "" && originalTitle != search.Title && !IsChineseMedia(originalLang, "") {
		fields.Alias = originalTitle
	}

	// Dict: directors (crew with job = "Director")
	if detail != nil {
		var directors []string
		for _, crew := range detail.Credits.Crew {
			if crew.Job == "Director" {
				directors = append(directors, crew.Name)
			}
		}
		if len(directors) > 0 {
			fields.Dict = strings.Join(directors, "、")
		}

		// Cast: top 2
		if len(detail.Credits.Cast) > 0 {
			var cast []string
			maxCast := min(len(detail.Credits.Cast), 2)
			for _, c := range detail.Credits.Cast[:maxCast] {
				cast = append(cast, c.Name)
			}
			fields.Cast = strings.Join(cast, "、")
		}
	}

	return fields
}

// --- TMDB TV Enricher ---

// TMDBTVEnricher enriches TV entries using the TMDB API.
type TMDBTVEnricher struct {
	client *tmdbClient
	cache  *Cache
}

// NewTMDBTVEnricher creates a TV enricher with the given TMDB API key.
func NewTMDBTVEnricher(apiKey string) *TMDBTVEnricher {
	return &TMDBTVEnricher{
		client: newTMDBClient(apiKey),
	}
}

// SetCache attaches a shared cache to this enricher.
func (e *TMDBTVEnricher) SetCache(c *Cache) { e.cache = c }

// Enrich searches TMDB for a TV show and returns fields to set.
func (e *TMDBTVEnricher) Enrich(ctx context.Context, name, publishAt string) (*EnrichFields, error) {
	searchResult, err := e.searchWithCache(ctx, name, publishAt)
	if err != nil {
		return nil, err
	}

	detail, err := getDetail[tmdbTvDetail](e.client, ctx, searchResult.ID, "tv")
	if err != nil {
		slog.Warn("failed to get tv detail, using search result", "id", searchResult.ID, "error", err)

		return mapTvFields(searchResult, nil), nil
	}

	return mapTvFields(searchResult, detail), nil
}

func (e *TMDBTVEnricher) searchWithCache(ctx context.Context, name, year string) (*tmdbSearchItem, error) {
	return e.client.searchWithCache(ctx, "tv", name, year, e.cache, e.client.searchTV)
}

//nolint:gocyclo,cyclop
func mapTvFields(search *tmdbSearchItem, detail *tmdbTvDetail) *EnrichFields {
	fields := &EnrichFields{}

	// publishAt from first_air_date
	if detail != nil && detail.FirstAirDate != "" {
		fields.PublishAt = extractYear(detail.FirstAirDate)
	} else if search.FirstAirDate != "" {
		fields.PublishAt = extractYear(search.FirstAirDate)
	}

	// Alias: original_name if different
	originalName := search.OriginalName
	if detail != nil && detail.OriginalName != "" {
		originalName = detail.OriginalName
	}
	originalLang := search.OriginalLanguage
	if detail != nil && detail.OriginalLanguage != "" {
		originalLang = detail.OriginalLanguage
	}
	if originalName != "" && originalName != search.Name && !IsChineseMedia(originalLang, "") {
		fields.Alias = originalName
	}

	// Dict: created_by
	if detail != nil && len(detail.CreatedBy) > 0 {
		var creators []string
		for _, c := range detail.CreatedBy {
			creators = append(creators, c.Name)
		}
		fields.Dict = strings.Join(creators, "、")
	}

	// Cast: top 2
	if detail != nil && len(detail.Credits.Cast) > 0 {
		var cast []string
		maxCast := min(len(detail.Credits.Cast), 2)
		for _, c := range detail.Credits.Cast[:maxCast] {
			cast = append(cast, c.Name)
		}
		fields.Cast = strings.Join(cast, "、")
	}

	return fields
}

// extractYear extracts the 4-digit year from a date string (YYYY-MM-DD or YYYY).
func extractYear(date string) string {
	if len(date) >= 4 {
		year := date[:4]
		// Verify it looks like a year
		if year >= "1900" && year <= "2100" {
			return year
		}
	}

	return ""
}
