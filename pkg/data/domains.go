package data

import (
	"path/filepath"
	"strings"
)

// IsStructuredCheckDomain returns true for domains that use structured field validation.
func IsStructuredCheckDomain(domain DataDomain) bool {
	switch domain {
	case DomainBooks, DomainMovie, DomainTV, DomainMusic, DomainDiary, DomainNtl:
		return true
	}

	return false
}

// IsDuplicateDomain returns true for domains supporting duplicate detection.
func IsDuplicateDomain(domain DataDomain) bool {
	switch domain {
	case DomainBooks, DomainMusic, DomainGH:
		return true
	}

	return false
}

// DefaultPathForDomain returns the default data path for a domain.
func DefaultPathForDomain(domain DataDomain) string {
	switch domain {
	case DomainBooks, DomainMovie, DomainTV:
		return "data/books"
	case DomainMusic:
		return "data/music"
	case DomainDiary:
		return "data/diary"
	case DomainGH:
		return "data/gh"
	case DomainGoods:
		return "data/goods"
	case DomainTask:
		return "data"
	case DomainNtl:
		return "data/.archive/z/ntl"
	}

	return ""
}

// DefaultScopeForDomain returns the default validation scope for a domain.
func DefaultScopeForDomain(domain DataDomain) string {
	switch domain {
	case DomainBooks:
		return string(ScopeBooks)
	case DomainMovie:
		return string(ScopeMovie)
	case DomainTV:
		return string(DomainTV)
	case DomainMusic:
		return string(ScopeMusic)
	case DomainDiary:
		return string(ScopeDiary)
	case DomainNtl:
		return string(DomainNtl)
	}

	return "auto"
}

// ResolveScope determines the actual RuleScope based on scope and filename.
func ResolveScope(file, scope string) RuleScope {
	switch scope {
	case "books":
		return ScopeBooks
	case "movie", "tv":
		return ScopeMovie
	case "music":
		return ScopeMusic
	case "diary":
		return ScopeDiary
	case "ntl":
		filename := strings.ToLower(filepath.Base(file))
		if filename == "jav.yml" {
			return ScopeJav
		}
		if filename == "vg.yml" {
			return ScopeVG
		}

		return ScopeMovie
	}

	return detectScopeFromFilename(file)
}

func detectScopeFromFilename(file string) RuleScope {
	filename := strings.ToLower(filepath.Base(file))
	if filename == "movie.yml" || filename == "tv.yml" {
		return ScopeMovie
	}
	if strings.HasPrefix(filename, "music-") && strings.HasSuffix(filename, ".yml") {
		return ScopeMusic
	}

	return ScopeBooks
}

// AllowedFieldsForScope returns the allowed field set for a rule scope.
func AllowedFieldsForScope(scope RuleScope) map[string]bool {
	switch scope {
	case ScopeDiary:
		return DiaryFields
	case ScopeJav:
		return JavFields
	case ScopeVG:
		return VGFields
	case ScopeBooks, ScopeMovie:
		return ContentFields
	case ScopeMusic:
		return MusicFields
	}

	return ContentFields
}
