package domrules

import "fmt"

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
	// books is validated by its embedded JSON Schema (books.schema.json) via
	// the books domain check; it has no structured-check rule scope.
	{Domain: DomainBooks, DefaultPath: defaultPathBooks, DuplicateCheck: true},
	{Domain: DomainDiary, DefaultPath: "data/diary", RuleScope: ScopeDiary, StructuredCheck: true},
	{Domain: DomainGH, DefaultPath: "data/gh", DuplicateCheck: true},
	// goods is validated by its embedded JSON Schema (goods.schema.json) via
	// the goods domain check; it has no structured-check rule scope.
	{Domain: DomainGoods, DefaultPath: "data/goods"},
	{Domain: DomainTask, DefaultPath: "data", YAMLParseOnly: true},
	// ntl (movie/TV/music) is validated against the shared books.schema.json
	// (see internal/gh/schema); .jav.yml/.asmr.yml are dot-prefixed and excluded
	// as hidden files by the default check.
	{Domain: DomainNtl, DefaultPath: "data/ntl"},
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

// DomainDefaultPath resolves a domain's data path to override when given,
// otherwise falling back to the domain's DefaultPath.
func DomainDefaultPath(domain DataDomain, override string) (string, error) {
	spec, ok := SpecForDomain(domain)
	if !ok {
		return "", fmt.Errorf("unknown data domain %q", domain)
	}
	if override != "" {
		return override, nil
	}

	return spec.DefaultPath, nil
}

// ResolveScope determines the actual RuleScope. Only diary uses the structured
// check now (books/ntl/goods are schema-checked, gh uses the walker, task is
// YAML-parse-only), so this always resolves to the diary scope.
func ResolveScope(file, scope string) RuleScope {
	return ScopeDiary
}

// AllowedFieldsForScope returns the allowed field set for a rule scope.
// Diary is the only structured-checked domain.
func AllowedFieldsForScope(scope RuleScope) map[string]bool {
	return DiaryFields
}
