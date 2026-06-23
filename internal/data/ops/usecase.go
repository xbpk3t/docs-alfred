package dataops

import (
	"errors"
	"fmt"
	"log/slog"

	"github.com/xbpk3t/docs-alfred/internal/data/render"
	"github.com/xbpk3t/docs-alfred/internal/gh/data"
	data "github.com/xbpk3t/docs-alfred/internal/gh/domrules"
	"github.com/xbpk3t/docs-alfred/internal/gh/goods"
	"github.com/xbpk3t/docs-alfred/pkg/checkutil"
)

// DomainCheckInput holds input for domain data check.
type DomainCheckInput struct {
	Domain     data.DataDomain
	Path       string // empty = default for domain
	RuleScope  string // empty = default for domain
	GhMaxLines int    // <= 0 = default for gh checks
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
	ghMaxLines int
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

	return domainCheckOptions{spec: spec, path: path, scope: scope, ghMaxLines: input.GhMaxLines}, nil
}

func runDomainCheckWithOptions(domain data.DataDomain, opts *domainCheckOptions) (*DomainCheckResult, error) {
	if domain == data.DomainGH {
		result, err := ghdata.RunGhCheckWithOptions(opts.path, ghdata.CheckOptions{MaxLines: opts.ghMaxLines})
		if err != nil {
			return nil, err
		}

		return &DomainCheckResult{Issues: result.Issues}, nil
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

// DomainDuplicateInput holds input for duplicate detection.
type DomainDuplicateInput struct {
	Domain data.DataDomain
	Path   string
}

// DomainDuplicateResult holds duplicate detection results.
type DomainDuplicateResult struct {
	Report *data.DuplicateReport
}

// RunDomainDuplicate detects duplicate entries in a domain data directory.
func RunDomainDuplicate(input DomainDuplicateInput) (*DomainDuplicateResult, error) {
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

	return &DomainDuplicateResult{Report: report}, nil
}

// RenderInput holds input for data rendering.
type RenderInput struct {
	Config  string
	Extract string
	Out     string
}

// RenderResult holds the data render outcome.
type RenderResult struct {
	OutputPath  string
	ConfigCount int
	Extracted   bool
}

// RunRender renders YAML data into outputs.
func RunRender(input RenderInput) (*RenderResult, error) {
	if input.Extract == "topics" {
		if input.Out == "" {
			return nil, errors.New("--out is required when --extract is set")
		}

		result, err := extractTopics(extractTopicsInput{Out: input.Out})
		if err != nil {
			return nil, err
		}

		return &RenderResult{Extracted: true, OutputPath: result.OutputPath}, nil
	}

	configCount, err := datarender.Run(input.Config)
	if err != nil {
		return nil, err
	}

	return &RenderResult{ConfigCount: configCount}, nil
}

type extractTopicsInput struct {
	Out string
}

type extractTopicsResult struct {
	OutputPath string
}

func extractTopics(input extractTopicsInput) (*extractTopicsResult, error) {
	if input.Out == "" {
		return nil, errors.New("--out is required")
	}
	if err := data.ExtractTopics(input.Out); err != nil {
		return nil, err
	}

	return &extractTopicsResult{OutputPath: input.Out}, nil
}
