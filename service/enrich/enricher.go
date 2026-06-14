// Package enrich enriches YAML metadata files using external structured APIs.
//
// Supported resources:
//   - movie: TMDB (title, year, director(s), cast, original title)
//   - tv:    TMDB (name, year, creator(s), cast, original name)
//   - book:  Google Books + Open Library fallback (title, author(s), year, subtitle)
package enrich

import (
	"context"
	"errors"
	"strings"
)

// ResourceType identifies the type of content to enrich.
type ResourceType string

const (
	ResourceMovie ResourceType = "movie"
	ResourceTV    ResourceType = "tv"
	ResourceBook  ResourceType = "book"
)

// YAML field names used during enrichment.
const (
	FieldName      = "name"
	FieldPublishAt = "publishAt"
	FieldAlias     = "alias"
	FieldDict      = "dict"
	FieldCast      = "cast"
	FieldAuthor    = "author"
)

// Chinese-language detection keywords for country/origin.
var (
	chineseKeywords  = []string{"中国", "台湾", "香港", "Chinese", "Taiwan", "Hong Kong"}
	chineseLanguages = []string{"zh", "cn", "tw", "hk"}
)

// ErrNotFound is returned when an API search returns no results.
// Callers should treat this as "no data found" rather than a hard error.
var ErrNotFound = errors.New("enrich: not found")

// IsChineseMedia returns true if the media likely originates from a Chinese-speaking region.
func IsChineseMedia(originalLanguage, originCountry string) bool {
	for _, lang := range chineseLanguages {
		if strings.EqualFold(originalLanguage, lang) {
			return true
		}
	}
	for _, kw := range chineseKeywords {
		if strings.Contains(originCountry, kw) {
			return true
		}
	}

	return false
}

// EnrichFields holds the fields discovered by the API that should be set in YAML.
type EnrichFields struct {
	PublishAt string `json:"publishAt,omitempty"` // year extracted from date string
	Alias     string `json:"alias,omitempty"`     // original title/name (if different from local name)
	Dict      string `json:"dict,omitempty"`      // director(s) or creator(s), joined with "、"
	Cast      string `json:"cast,omitempty"`      // cast members, joined with "、"
	Author    string `json:"author,omitempty"`    // book author(s), joined with "、"
}

// EnrichAction records what happened to a single YAML field.
type EnrichAction struct {
	Field   string // YAML field name
	Value   string // value set (empty if skipped)
	Skipped bool   // true if field already existed and was preserved
}

// EnrichResult holds the enrichment outcome for a single YAML item.
type EnrichResult struct {
	Err         error
	Name        string
	Actions     []EnrichAction
	Index       int
	NeedsReview bool
}

// EnrichReport summarizes the enrichment of one YAML file.
type EnrichReport struct {
	Resource ResourceType
	File     string
	Results  []EnrichResult
	DryRun   bool
}

// HasChanges returns true if at least one field was set (non-skipped).
func (r *EnrichResult) HasChanges() bool {
	for _, a := range r.Actions {
		if !a.Skipped {
			return true
		}
	}

	return false
}

// Enricher is the interface implemented by each data-source backend.
type Enricher interface {
	// Enrich searches for content by name and optional year, returning
	// fields extracted from the external API.
	Enrich(ctx context.Context, name, publishAt string) (*EnrichFields, error)
}

// EnricherFor returns the appropriate enricher for the given resource type.
func EnricherFor(rt ResourceType, apiKey string) Enricher {
	switch rt {
	case ResourceMovie:
		return NewTMDBMovieEnricher(apiKey)
	case ResourceTV:
		return NewTMDBTVEnricher(apiKey)
	case ResourceBook:
		return NewBooksEnricher(apiKey)
	default:
		return nil
	}
}
