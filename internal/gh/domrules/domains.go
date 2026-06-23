package domrules

import (
	"path/filepath"
	"strings"
)

// DomainSpec defines the default behavior for a data domain.
type DomainSpec struct {
	Domain          DataDomain
	DefaultPath     string
	RuleScope       RuleScope
	StructuredCheck bool
	DuplicateCheck  bool
	YAMLParseOnly   bool
}

const defaultPathBooks = "data/books"

var domainSpecs = []DomainSpec{
	{Domain: DomainBooks, DefaultPath: defaultPathBooks, RuleScope: ScopeBooks, StructuredCheck: true, DuplicateCheck: true},
	{Domain: DomainMovie, DefaultPath: defaultPathBooks, RuleScope: ScopeMovie, StructuredCheck: true},
	{Domain: DomainTV, DefaultPath: defaultPathBooks, RuleScope: ScopeMovie, StructuredCheck: true},
	{Domain: DomainMusic, DefaultPath: "data/music", RuleScope: ScopeMusic, StructuredCheck: true, DuplicateCheck: true},
	{Domain: DomainDiary, DefaultPath: "data/diary", RuleScope: ScopeDiary, StructuredCheck: true},
	{Domain: DomainGH, DefaultPath: "data/gh", DuplicateCheck: true},
	{Domain: DomainGoods, DefaultPath: "data/goods", YAMLParseOnly: true},
	{Domain: DomainTask, DefaultPath: "data", YAMLParseOnly: true},
	{Domain: DomainNtl, DefaultPath: "data/.archive/ntl", RuleScope: RuleScope(DomainNtl), StructuredCheck: true},
}

// SpecForDomain returns the configured behavior for a data domain.
func SpecForDomain(domain DataDomain) (DomainSpec, bool) {
	for _, spec := range domainSpecs {
		if spec.Domain == domain {
			return spec, true
		}
	}

	return DomainSpec{}, false
}

// DefaultPathForDomain returns the default data path for a domain.
func DefaultPathForDomain(domain DataDomain) string {
	spec, ok := SpecForDomain(domain)
	if !ok {
		return ""
	}

	return spec.DefaultPath
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
