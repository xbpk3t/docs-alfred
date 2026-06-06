package usecase

import (
	"errors"
	"fmt"
	"log/slog"

	"github.com/xbpk3t/docs-alfred/internal/datarender"
	"github.com/xbpk3t/docs-alfred/pkg/checkutil"
	pkgdata "github.com/xbpk3t/docs-alfred/pkg/data"
	ghcheck "github.com/xbpk3t/docs-alfred/pkg/gh"
)

// DomainCheckInput holds input for domain data check.
type DomainCheckInput struct {
	Domain    pkgdata.DataDomain
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
	path  string
	scope string
	spec  pkgdata.DomainSpec
}

func resolveDomainCheckOptions(input DomainCheckInput) (domainCheckOptions, error) {
	spec, ok := pkgdata.SpecForDomain(input.Domain)
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

func runDomainCheckWithOptions(domain pkgdata.DataDomain, opts *domainCheckOptions) (*DomainCheckResult, error) {
	if domain == pkgdata.DomainGH {
		result, err := ghcheck.RunGhCheck(opts.path)
		if err != nil {
			return nil, err
		}

		return &DomainCheckResult{Issues: result.Issues}, nil
	}

	if opts.spec.StructuredCheck {
		result, err := pkgdata.RunStructuredDataCheck(opts.path, opts.scope)
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

func runYAMLParseOnlyDomainCheck(domain pkgdata.DataDomain, path string) (*DomainCheckResult, error) {
	count, errs := pkgdata.ParseYAMLDir(path)
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
	Domain pkgdata.DataDomain
	Path   string
}

// DomainDuplicateResult holds duplicate detection results.
type DomainDuplicateResult struct {
	Report *pkgdata.DuplicateReport
}

// RunDomainDuplicate detects duplicate entries in a domain data directory.
func RunDomainDuplicate(input DomainDuplicateInput) (*DomainDuplicateResult, error) {
	spec, ok := pkgdata.SpecForDomain(input.Domain)
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
		report *pkgdata.DuplicateReport
		err    error
	)
	if input.Domain == pkgdata.DomainGH {
		report, err = pkgdata.RunGHDuplicateCheck(path)
	} else {
		report, err = pkgdata.RunDuplicateCheck(path)
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
	if err := pkgdata.ExtractTopics(input.Out); err != nil {
		return nil, err
	}

	return &extractTopicsResult{OutputPath: input.Out}, nil
}

// GhFindInput holds input for local gh entry search.
type GhFindInput struct {
	Root  string
	Query string
	URL   string
	Limit int
}

// GhFindResult holds search results.
type GhFindResult struct {
	Entries []ghcheck.FindEntry
}

// RunGhFind searches local data/gh entries.
func RunGhFind(input GhFindInput) (*GhFindResult, error) {
	root := input.Root
	if root == "" {
		root = pkgdata.DefaultPathForDomain(pkgdata.DomainGH)
	}

	slog.Info("Searching data/gh", "query", input.Query, "url", input.URL, "limit", input.Limit)

	entries, err := ghcheck.FindEntries(root, input.Query, input.URL)
	if err != nil {
		return nil, err
	}

	ghcheck.SortEntries(entries)

	if input.Limit > 0 && input.Limit < len(entries) {
		entries = entries[:input.Limit]
	}

	return &GhFindResult{Entries: entries}, nil
}

// GhAppendInput holds input for appending a record.
type GhAppendInput struct {
	File  string
	URL   string
	Date  string
	Des   string
	Topic string
}

// GhAppendResult holds the result of appending a record.
type GhAppendResult struct {
	File string
	Diff string
}

// RunGhAppend appends a record to a data/gh entry.
func RunGhAppend(input *GhAppendInput) (*GhAppendResult, error) {
	if input == nil {
		return nil, errors.New("append input is required")
	}
	if input.URL == "" && input.File == "" {
		return nil, errors.New("either --url or --file is required")
	}
	if input.Date == "" || input.Des == "" {
		return nil, errors.New("--date and --des are required")
	}

	slog.Info("Appending record", "url", input.URL, "date", input.Date, "des", input.Des)

	result, err := ghcheck.AppendRecord(&ghcheck.AppendRecordOptions{
		File:  input.File,
		URL:   input.URL,
		Date:  input.Date,
		Des:   input.Des,
		Topic: input.Topic,
	})
	if err != nil {
		return nil, fmt.Errorf("append-record failed: %w", err)
	}

	return &GhAppendResult{File: result.File, Diff: result.Diff}, nil
}
