package dataops

import (
	"fmt"
	"log/slog"

	datarender "github.com/xbpk3t/docs-alfred/internal/data/render"
	"github.com/xbpk3t/docs-alfred/internal/gh/books"
	data "github.com/xbpk3t/docs-alfred/internal/gh/domrules"
	"github.com/xbpk3t/docs-alfred/internal/gh/ghcheck"
	"github.com/xbpk3t/docs-alfred/internal/gh/goods"
	"github.com/xbpk3t/docs-alfred/internal/gh/schema"
	"github.com/xbpk3t/docs-alfred/pkg/checkutil"
	"github.com/xbpk3t/docs-alfred/pkg/schemacheck"
)

// DomainCheckInput holds input for domain data check.
type DomainCheckInput struct {
	Domain data.DataDomain
	Path   string // empty = default for domain

	// IncludeHidden reports whether hidden (dot-prefixed) YAML files are checked.
	IncludeHidden bool
}

// DomainCheckResult holds the result of a domain data check.
type DomainCheckResult struct {
	Issues []checkutil.Issue
}

// RunDomainCheck validates YAML files in a domain data directory.
func RunDomainCheck(input DomainCheckInput) (*DomainCheckResult, error) {
	opts, err := resolveDomainCheckOptions(input)
	if err != nil {
		return nil, err
	}

	slog.Info("Checking domain", "domain", input.Domain, "path", opts.path)

	return runDomainCheckWithOptions(input.Domain, &opts)
}

type domainCheckOptions struct {
	path          string
	spec          data.DomainSpec
	includeHidden bool
}

func resolveDomainCheckOptions(input DomainCheckInput) (domainCheckOptions, error) {
	spec, ok := data.SpecForDomain(input.Domain)
	if !ok {
		return domainCheckOptions{}, fmt.Errorf("unknown data domain %q", input.Domain)
	}

	path := input.Path
	if path == "" {
		path = spec.DefaultPath
	}

	return domainCheckOptions{spec: spec, path: path, includeHidden: input.IncludeHidden}, nil
}

func runDomainCheckWithOptions(domain data.DataDomain, opts *domainCheckOptions) (*DomainCheckResult, error) {
	switch opts.spec.Check {
	case data.CheckGoodsSchema:
		result, err := goods.RunCheckWithOptions(opts.path, goods.CheckOptions{IncludeHidden: opts.includeHidden})
		if err != nil {
			return nil, err
		}

		return &DomainCheckResult{Issues: result.Issues}, nil
	case data.CheckBooksSchema:
		// books and ntl share the books.schema.json (ntl borrows the books
		// schema — same table shape).
		result, err := books.RunCheckWithOptions(opts.path, books.CheckOptions{IncludeHidden: opts.includeHidden})
		if err != nil {
			return nil, err
		}

		return &DomainCheckResult{Issues: result.Issues}, nil
	case data.CheckWalker:
		result, err := ghcheck.RunCheckWithOptions(opts.path, ghcheck.CheckOptions{IncludeHidden: opts.includeHidden})
		if err != nil {
			return nil, err
		}

		return &DomainCheckResult{Issues: result.Issues}, nil
	case data.CheckDiarySchema:
		issues, err := schemacheck.CheckDirectory(opts.path, "diary", schema.Diary, opts.includeHidden)
		if err != nil {
			return nil, err
		}

		return &DomainCheckResult{Issues: issues}, nil
	case data.CheckYAMLParseOnly:
		return runYAMLParseOnlyDomainCheck(domain, opts.path)
	default:
		slog.Info("Data check passed", "domain", domain)

		return &DomainCheckResult{}, nil
	}
}

func runYAMLParseOnlyDomainCheck(domain data.DataDomain, path string) (*DomainCheckResult, error) {
	count, errs := data.ParseYAMLDir(path)
	for _, e := range errs {
		slog.Error("YAML parse error", "error", e)
	}
	if len(errs) > 0 {
		return nil, fmt.Errorf("data check %s: %d file(s) failed YAML parsing", domain, len(errs))
	}
	slog.Info("Data check passed", "domain", domain, "files", count)

	return &DomainCheckResult{}, nil
}

// DomainDedupInput holds input for duplicate detection.
type DomainDedupInput struct {
	Domain data.DataDomain
	Path   string
}

// DomainDedupResult holds duplicate detection results.
type DomainDedupResult struct {
	Report *data.DuplicateReport
}

// RunDomainDedup detects duplicate entries in a domain data directory.
func RunDomainDedup(input DomainDedupInput) (*DomainDedupResult, error) {
	spec, ok := data.SpecForDomain(input.Domain)
	if !ok {
		return nil, fmt.Errorf("unknown data domain %q", input.Domain)
	}
	if spec.Duplicate == data.DupNone {
		return nil, fmt.Errorf("data duplicate %s is not supported", input.Domain)
	}

	path := input.Path
	if path == "" {
		path = spec.DefaultPath
	}

	slog.Info("Checking duplicates", "domain", input.Domain, "path", path)

	var (
		report *data.DuplicateReport
		err    error
	)
	if spec.Duplicate == data.DupGH {
		report, err = data.RunGHDuplicateCheck(path)
	} else {
		report, err = data.RunDuplicateCheck(path)
	}
	if err != nil {
		return nil, err
	}

	return &DomainDedupResult{Report: report}, nil
}

// DomainRenderInput holds input for domain rendering.
type DomainRenderInput struct {
	Domain data.DataDomain
	Path   string // empty = default for domain
	OutDir string // empty = "docs/public"
	Format string // empty = domain default
}

// DomainRenderResult holds the result of a domain render.
type DomainRenderResult = datarender.DomainRenderResult

// RunDomainRender renders a single domain's data into output files.
func RunDomainRender(input DomainRenderInput) (*DomainRenderResult, error) {
	spec, ok := data.SpecForDomain(input.Domain)
	if !ok {
		return nil, fmt.Errorf("unknown data domain %q", input.Domain)
	}

	src := input.Path
	if src == "" {
		src = spec.DefaultPath
	}

	outDir := input.OutDir
	if outDir == "" {
		outDir = "docs/public"
	}

	format := input.Format
	if format == "" {
		format = defaultRenderFormat(input.Domain)
	}

	slog.Info("Rendering domain", "domain", input.Domain, "src", src, "outDir", outDir, "format", format)

	return datarender.RunDomainRender(datarender.DomainRenderConfig{
		Domain: string(input.Domain),
		Src:    src,
		OutDir: outDir,
		Format: format,
	})
}

func defaultRenderFormat(domain data.DataDomain) string {
	spec, ok := data.SpecForDomain(domain)
	if !ok || spec.DefaultFormat == "" {
		return "yaml"
	}

	return spec.DefaultFormat
}
