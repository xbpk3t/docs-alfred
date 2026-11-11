package domrules

import (
	"fmt"

	"github.com/xbpk3t/docs-alfred/pkg/render"
)

// DataDomain defines a data domain for validation.
type DataDomain string

const (
	DomainBooks DataDomain = "books"
	DomainDiary DataDomain = "diary"
	DomainGH    DataDomain = "gh"
	DomainGoods DataDomain = "goods"
	DomainTask  DataDomain = "task"
	DomainNtl   DataDomain = "ntl"
)

// CheckStrategy selects how a domain's data directory is validated.
type CheckStrategy string

const (
	// CheckGoodsSchema validates against the embedded goods.schema.json.
	CheckGoodsSchema CheckStrategy = "goods_schema"
	// CheckBooksSchema validates against the embedded books.schema.json (books/ntl).
	CheckBooksSchema CheckStrategy = "books_schema"
	// CheckDiarySchema validates against the embedded diary.schema.json.
	CheckDiarySchema CheckStrategy = "diary_schema"
	// CheckWalker runs the gh walker check.
	CheckWalker CheckStrategy = "walker"
	// CheckYAMLParseOnly only parses YAML files (task).
	CheckYAMLParseOnly CheckStrategy = "yaml_parse_only"
)

// DuplicateStrategy selects how duplicates are detected.
type DuplicateStrategy string

const (
	// DupNone disables duplicate detection.
	DupNone DuplicateStrategy = ""
	// DupGeneric runs the generic name/url duplicate check (books).
	DupGeneric DuplicateStrategy = "generic"
	// DupGH runs the gh-specific owner/repo duplicate check.
	DupGH DuplicateStrategy = "gh"
)

// RenderStrategy selects how a domain's files are rendered.
type RenderStrategy string

const (
	// RenderNone disables rendering for a domain.
	RenderNone RenderStrategy = "none"
	// RenderGeneric uses the single-stream YAML renderer (task); books/ntl also
	// use it as the base when their catalog path isn't taken.
	RenderGeneric RenderStrategy = "generic"
	// RenderGH walks the gh directory tree and emits a merged repo list.
	RenderGH RenderStrategy = "gh"
	// RenderGoods uses the goods renderer (single-file goods input).
	RenderGoods RenderStrategy = "goods"
)

// DomainSpec defines the default behavior for a data domain.
type DomainSpec struct {
	Domain        DataDomain
	DefaultPath   string
	Check         CheckStrategy
	Duplicate     DuplicateStrategy
	Render        RenderStrategy
	DefaultFormat string
	CatalogPrefix string
	ParseMode     render.ParseMode
	Catalog       bool
}

const defaultPathBooks = "data/books"

var domainSpecs = []DomainSpec{
	// books is validated by its embedded JSON Schema (books.schema.json) via
	// the books domain check.
	{Domain: DomainBooks, DefaultPath: defaultPathBooks,
		Check: CheckBooksSchema, Duplicate: DupGeneric,
		Render: RenderGeneric, DefaultFormat: "json", ParseMode: render.ParseFlatten,
		Catalog: true, CatalogPrefix: "books."},
	// diary is validated by its embedded JSON Schema (diary.schema.json); it is
	// not rendered.
	{Domain: DomainDiary, DefaultPath: "data/diary",
		Check: CheckDiarySchema, Render: RenderNone},
	{Domain: DomainGH, DefaultPath: "data/gh",
		Check: CheckWalker, Duplicate: DupGH,
		Render: RenderGH, DefaultFormat: "json,yaml", ParseMode: render.ParseFlatten},
	// goods is validated by its embedded JSON Schema (goods.schema.json) via
	// the goods domain check.
	{Domain: DomainGoods, DefaultPath: "data/goods",
		Check:  CheckGoodsSchema,
		Render: RenderGoods, DefaultFormat: "json", ParseMode: render.ParseFlatten,
		Catalog: true, CatalogPrefix: "goods."},
	{Domain: DomainTask, DefaultPath: "data",
		Check:  CheckYAMLParseOnly,
		Render: RenderGeneric, DefaultFormat: "yaml", ParseMode: render.ParseMulti},
	// ntl (movie/TV/music) is validated against the shared books.schema.json
	// (see internal/gh/schema); .jav.yml/.asmr.yml are dot-prefixed and excluded
	// as hidden files by the default check.
	{Domain: DomainNtl, DefaultPath: "data/ntl",
		Check:  CheckBooksSchema,
		Render: RenderGeneric, DefaultFormat: "json", ParseMode: render.ParseFlatten,
		Catalog: true},
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
