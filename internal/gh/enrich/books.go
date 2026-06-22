package enrich

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/go-resty/resty/v2"
	"golang.org/x/time/rate"
)

// defaultGBRate limits Google Books API to 1 req/s (safe under 100 req/min quota).
const defaultGBRate = rate.Limit(1.0)

const (
	googleBooksBaseURL = "https://www.googleapis.com/books/v1"
	openLibraryBaseURL = "https://openlibrary.org"
)

// --- Google Books API response types ---

type gbSearchResult struct {
	Items []gbVolume `json:"items"`
}

type gbVolume struct {
	VolumeInfo gbVolumeInfo `json:"volumeInfo"`
}

type gbVolumeInfo struct {
	Title         string   `json:"title"`
	Subtitle      string   `json:"subtitle"`
	PublishedDate string   `json:"publishedDate"`
	Publisher     string   `json:"publisher"`
	Authors       []string `json:"authors"`
}

// --- Open Library API response types ---

type olSearchResult struct {
	Docs []olDoc `json:"docs"`
}

type olDoc struct {
	Title            string   `json:"title"`
	AuthorName       []string `json:"author_name"`
	PublishYear      []int    `json:"publish_year"`
	FirstPublishYear int      `json:"first_publish_year"`
}

// --- Google Books client ---

type googleBooksClient struct {
	http   *resty.Client
	rl     *rate.Limiter
	apiKey string
}

func newGoogleBooksClient(apiKey string) *googleBooksClient {
	return &googleBooksClient{
		apiKey: apiKey,
		http:   resty.New().SetBaseURL(googleBooksBaseURL),
		rl:     rate.NewLimiter(defaultGBRate, 1),
	}
}

func (c *googleBooksClient) search(ctx context.Context, query string) (*gbVolumeInfo, error) {
	if err := c.rl.Wait(ctx); err != nil {
		return nil, err
	}

	resp, err := c.http.R().
		SetContext(ctx).
		SetQueryParams(map[string]string{
			"q":            query,
			"key":          c.apiKey,
			"langRestrict": "zh-CN",
			"maxResults":   "5",
		}).
		Get("/volumes")
	if err != nil {
		return nil, fmt.Errorf("google books request: %w", err)
	}
	if resp.IsError() {
		return nil, fmt.Errorf("google books: %s", resp.String())
	}

	var result gbSearchResult
	if err := json.Unmarshal(resp.Body(), &result); err != nil {
		return nil, fmt.Errorf("google books decode: %w", err)
	}
	if len(result.Items) == 0 {
		return nil, ErrNotFound
	}

	return &result.Items[0].VolumeInfo, nil
}

// --- Open Library client (fallback) ---

type openLibraryClient struct {
	http *resty.Client
}

func newOpenLibraryClient() *openLibraryClient {
	return &openLibraryClient{
		http: resty.New().SetBaseURL(openLibraryBaseURL),
	}
}

func (c *openLibraryClient) search(ctx context.Context, query string) (*olDoc, error) {
	resp, err := c.http.R().
		SetContext(ctx).
		SetQueryParam("q", query).
		Get("/search.json")
	if err != nil {
		return nil, fmt.Errorf("open library request: %w", err)
	}
	if resp.IsError() {
		return nil, fmt.Errorf("open library: %s", resp.String())
	}

	var result olSearchResult
	if err := json.Unmarshal(resp.Body(), &result); err != nil {
		return nil, fmt.Errorf("open library decode: %w", err)
	}
	if len(result.Docs) == 0 {
		return nil, ErrNotFound
	}

	return &result.Docs[0], nil
}

// --- Books Enricher ---

// BooksEnricher enriches book entries using Google Books + Open Library fallback.
type BooksEnricher struct {
	google *googleBooksClient
	ol     *openLibraryClient
	cache  *Cache
}

// NewBooksEnricher creates a book enricher with the given Google Books API key.
func NewBooksEnricher(apiKey string) *BooksEnricher {
	return &BooksEnricher{
		google: newGoogleBooksClient(apiKey),
		ol:     newOpenLibraryClient(),
	}
}

// SetCache attaches a shared cache to this enricher.
func (e *BooksEnricher) SetCache(c *Cache) { e.cache = c }

// Enrich searches for a book and returns fields to set.
// Tries Google Books first, falls back to Open Library.
func (e *BooksEnricher) Enrich(ctx context.Context, name, publishAt string) (*EnrichFields, error) {
	// Try Google Books
	query := name
	fields, err := e.searchGoogleWithCache(ctx, query)
	if err != nil && !errors.Is(err, ErrNotFound) {
		return nil, err
	}
	if fields != nil {
		return fields, nil
	}

	// Fallback to Open Library
	fields, err = e.searchOLWithCache(ctx, query)
	if err != nil && !errors.Is(err, ErrNotFound) {
		return nil, err
	}
	if fields != nil {
		return fields, nil
	}

	return nil, ErrNotFound // needs review
}

// cachedSearch is a generic helper that wraps an API search with cache lookup/store.
func cachedSearch[T any](e *BooksEnricher, ctx context.Context, cacheKey, query string, searchFn func(context.Context, string) (*T, error), mapFn func(*T) *EnrichFields) (*EnrichFields, error) {
	if e.cache != nil {
		if cached := e.cache.Get(cacheKey); cached != nil {
			var fields EnrichFields
			if err := json.Unmarshal(cached, &fields); err == nil {
				return &fields, nil
			}
		}
	}

	raw, err := searchFn(ctx, query)
	if err != nil {
		return nil, err
	}
	if raw == nil {
		return nil, ErrNotFound
	}

	fields := mapFn(raw)

	if e.cache != nil {
		if data, err := json.Marshal(fields); err == nil {
			e.cache.Set(cacheKey, data)
		}
	}

	return fields, nil
}

func (e *BooksEnricher) searchGoogleWithCache(ctx context.Context, query string) (*EnrichFields, error) {
	return cachedSearch(e, ctx, "gb:"+query, query, e.google.search, mapGoogleBooksFields)
}

func (e *BooksEnricher) searchOLWithCache(ctx context.Context, query string) (*EnrichFields, error) {
	return cachedSearch(e, ctx, "ol:"+query, query, e.ol.search, mapOLFields)
}

func mapGoogleBooksFields(info *gbVolumeInfo) *EnrichFields {
	fields := &EnrichFields{}

	if info.PublishedDate != "" {
		fields.PublishAt = extractYear(info.PublishedDate)
	}

	if len(info.Authors) > 0 {
		fields.Author = strings.Join(info.Authors, "、")
	}

	if info.Subtitle != "" {
		// Only use subtitle as alias if it's reasonably short
		if len(info.Subtitle) < 120 {
			fields.Alias = info.Subtitle
		}
	}

	return fields
}

func mapOLFields(doc *olDoc) *EnrichFields {
	fields := &EnrichFields{}

	if doc.FirstPublishYear > 0 {
		fields.PublishAt = strconv.Itoa(doc.FirstPublishYear)
	} else if len(doc.PublishYear) > 0 && doc.PublishYear[0] > 0 {
		fields.PublishAt = strconv.Itoa(doc.PublishYear[0])
	}

	if len(doc.AuthorName) > 0 {
		fields.Author = strings.Join(doc.AuthorName, "、")
	}

	return fields
}
