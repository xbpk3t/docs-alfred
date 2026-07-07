package dataops

import (
	"fmt"
	"log/slog"

	"github.com/xbpk3t/docs-alfred/internal/data/render"
	data "github.com/xbpk3t/docs-alfred/internal/gh/domrules"
	"github.com/xbpk3t/docs-alfred/internal/gh/goods"
	"github.com/xbpk3t/docs-alfred/pkg/checkutil"
)

// DomainCheckInput holds input for domain data check.
type DomainCheckInput struct {
	Domain    data.DataDomain
	Path      string // empty = default for domain
	RuleScope string // empty = default for domain
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

	slog.Info("Checking domain", "domain", input.Domain, "path", opts.path, "scope", opts.scope)

	return runDomainCheckWithOptions(input.Domain, &opts)
}

type domainCheckOptions struct {
	path       string
	scope      string
	spec       data.DomainSpec
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
	scope := input.RuleScope
	if scope == "" {
		scope = string(spec.RuleScope)
		if scope == "" {
			scope = "auto"
		}
	}

	return domainCheckOptions{spec: spec, path: path, scope: scope}, nil
}

func runDomainCheckWithOptions(domain data.DataDomain, opts *domainCheckOptions) (*DomainCheckResult, error) {
	if domain == data.DomainGH {
		return &DomainCheckResult{}, nil
	}

	if domain == data.DomainGoods {
		result, err := goods.RunCheck(opts.path)
		if err != nil {
			return nil, err
		}

		return &DomainCheckResult{Issues: result.Issues}, nil
	}

	if opts.spec.StructuredCheck {
		result, err := data.RunStructuredDataCheck(opts.path, opts.scope)
		if err != nil {
			return nil, err
		}

		return &DomainCheckResult{Issues: result.Issues}, nil
	}

	if opts.spec.YAMLParseOnly {
		return runYAMLParseOnlyDomainCheck(domain, opts.path)
	}

	slog.Info("Data check passed", "domain", domain)

	return &DomainCheckResult{}, nil
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
	if !spec.DuplicateCheck {
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
	if input.Domain == data.DomainGH {
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
	switch domain {
	case data.DomainGH:
		return "json,yaml"
	case data.DomainGoods:
		return "json"
	default:
		return "yaml"
	}
}
