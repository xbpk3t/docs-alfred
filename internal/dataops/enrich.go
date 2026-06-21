package dataops

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"

	"github.com/samber/mo"
	"golang.org/x/sync/errgroup"

	"github.com/xbpk3t/docs-alfred/service/data"
	"github.com/xbpk3t/docs-alfred/service/enrich"
)

// EnrichInput holds the input for an enrichment operation.
type EnrichInput struct {
	Resource enrich.ResourceType
	Path     string
	Cache    string
	APIKey   string
	DryRun   bool
}

// EnrichResult holds the result of a file enrichment.
type EnrichResult struct {
	Report *enrich.EnrichReport
}

//nolint:cyclop
// RunEnrich performs enrichment on a YAML file.
func RunEnrich(ctx context.Context, input *EnrichInput) (*EnrichResult, error) {
	if err := validateInput(input); err != nil {
		return nil, err
	}

	// Parse the YAML file
	items, _, err := enrich.ParseYAMLFile(input.Path)
	if err != nil {
		return nil, fmt.Errorf("parse yaml: %w", err)
	}
	if len(items) == 0 {
		return nil, fmt.Errorf("no items found in %s", input.Path)
	}

	slog.Info("Enriching", "resource", input.Resource, "file", input.Path, "items", len(items), "dry_run", input.DryRun)

	// Create enricher
	enricher := enrich.EnricherFor(input.Resource, input.APIKey)

	// Set up cache
	var cache *enrich.Cache
	if input.Cache != "" {
		cache = enrich.NewCache(input.Cache)
		if s, ok := enricher.(interface{ SetCache(*enrich.Cache) }); ok {
			s.SetCache(cache)
		}
	}

	// Run enrichment concurrently with errgroup (max 3 concurrent)
	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(3)

	results := make([]*enrich.EnrichResult, len(items))
	for idx, item := range items {
		g.Go(func() error {
			res, err := processItem(gctx, item, enricher, input.DryRun, idx)
			if err != nil {
				return err
			}
			results[idx] = res

			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return nil, err
	}

	// Flush cache
	if cache != nil {
		cache.Flush()
	}

	// Save YAML file (unless dry-run)
	if !input.DryRun {
		if err := enrich.SaveYAMLFile(input.Path, items); err != nil {
			return nil, fmt.Errorf("save yaml: %w", err)
		}
	}

	return buildReport(results, input), nil
}

// validateInput checks that the input is valid and resolves defaults.
func validateInput(input *EnrichInput) error {
	if input == nil {
		return errors.New("enrich input is required")
	}
	if input.Path == "" {
		path, err := enrichPathForResource(input.Resource)
		if err != nil {
			return err
		}
		input.Path = path
	}
	if input.APIKey == "" {
		return fmt.Errorf("API key is required for %s enrichment (set via environment variable)", input.Resource)
	}

	return nil
}

//nolint:gocyclo,cyclop
// processItem enriches a single YAML item and returns its result.
func processItem(ctx context.Context, item *enrich.ItemNode, enricher enrich.Enricher, dryRun bool, idx int) (*enrich.EnrichResult, error) {
	name := item.GetName()
	if name == "" {
		return &enrich.EnrichResult{
			Index: idx,
			Name:  "(unnamed)",
			Err:   fmt.Errorf("item %d has no name field", idx),
		}, nil
	}

	// Determine which fields we need to fill
	needsPublishAt := !item.FieldExists(enrich.FieldPublishAt)
	needsAlias := !item.FieldExists(enrich.FieldAlias)
	needsDict := !item.FieldExists(enrich.FieldDict)
	needsAuthor := !item.FieldExists(enrich.FieldAuthor)
	needsCast := !item.FieldExists(enrich.FieldCast)

	// Skip if all target fields already exist
	if !needsPublishAt && !needsAlias && !needsDict && !needsAuthor && !needsCast {
		return &enrich.EnrichResult{Index: idx, Name: name}, nil
	}

	// Call the enricher
	publishAt := item.GetPublishAt()
	fields, err := enricher.Enrich(ctx, name, publishAt)
	if err != nil {
		if errors.Is(err, enrich.ErrNotFound) {
			return &enrich.EnrichResult{
				Index: idx, Name: name, NeedsReview: true,
			}, nil
		}

		return &enrich.EnrichResult{
			Index: idx, Name: name,
			Err: fmt.Errorf("enrich %q: %w", name, err),
		}, nil
	}

	// Map fields to actions
	res := &enrich.EnrichResult{Index: idx, Name: name}
	if needsPublishAt && fields.PublishAt != "" {
		res.Actions = append(res.Actions, enrich.EnrichAction{
			Field: enrich.FieldPublishAt, Value: fields.PublishAt,
		})
	}
	if needsAlias && fields.Alias != "" {
		res.Actions = append(res.Actions, enrich.EnrichAction{
			Field: enrich.FieldAlias, Value: fields.Alias,
		})
	}
	if needsDict && fields.Dict != "" {
		res.Actions = append(res.Actions, enrich.EnrichAction{
			Field: enrich.FieldDict, Value: fields.Dict,
		})
	}
	if needsAuthor && fields.Author != "" {
		res.Actions = append(res.Actions, enrich.EnrichAction{
			Field: enrich.FieldAuthor, Value: fields.Author,
		})
	}
	if needsCast && fields.Cast != "" {
		res.Actions = append(res.Actions, enrich.EnrichAction{
			Field: enrich.FieldCast, Value: fields.Cast,
		})
	}

	// Apply changes unless dry-run
	if !dryRun {
		for _, action := range res.Actions {
			if err := item.SetField(action.Field, action.Value); err != nil {
				return res, fmt.Errorf("set field %s for %q: %w", action.Field, name, err)
			}
		}
	}

	return res, nil
}

// buildReport converts results into an EnrichResult with a formatted report.
func buildReport(results []*enrich.EnrichResult, input *EnrichInput) *EnrichResult {
	flat := make([]enrich.EnrichResult, len(results))
	for i, r := range results {
		flat[i] = mo.PointerToOption(r).OrElse(enrich.EnrichResult{Index: i, Name: "(unknown)"})
	}

	report := &enrich.EnrichReport{
		Resource: input.Resource,
		File:     input.Path,
		Results:  flat,
		DryRun:   input.DryRun,
	}

	return &EnrichResult{Report: report}
}

// FormatEnrichReport formats the enrichment report for display.
//
//nolint:cyclop
func FormatEnrichReport(r *enrich.EnrichReport) string {
	var b strings.Builder

	dryRunLabel := ""
	if r.DryRun {
		dryRunLabel = " (dry-run, no changes written)"
	}

	fmt.Fprintf(&b, "📝 Enrich Report for %s%s\n", r.File, dryRunLabel)

	var totalSet, totalSkipped, totalNeedsReview, totalErrors int

	for _, res := range r.Results {
		if res.Err != nil {
			totalErrors++
			fmt.Fprintf(&b, "  ❌ [%d] %s: error — %v\n", res.Index, res.Name, res.Err)

			continue
		}
		if res.NeedsReview {
			totalNeedsReview++
			fmt.Fprintf(&b, "  ⚠️  [%d] %s: needs_review — no results found\n", res.Index, res.Name)

			continue
		}
		if len(res.Actions) == 0 {
			// All fields already exist
			fmt.Fprintf(&b, "  ✅ [%d] %s: already complete (no fields to fill)\n", res.Index, res.Name)

			continue
		}

		var setFields, skippedFields int
		for _, action := range res.Actions {
			if action.Value != "" {
				setFields++
			} else {
				skippedFields++
			}
		}
		totalSet += setFields
		totalSkipped += skippedFields

		fmt.Fprintf(&b, "  📄 [%d] %s: ", res.Index, res.Name)
		var parts []string
		for _, action := range res.Actions {
			if action.Skipped {
				parts = append(parts, action.Field+" ⏭️")
			} else {
				parts = append(parts, fmt.Sprintf("%s=%q", action.Field, action.Value))
			}
		}
		b.WriteString(strings.Join(parts, ", "))
		b.WriteString("\n")
	}

	// Summary
	b.WriteString("\n")
	fmt.Fprintf(&b, "Summary: %d items processed", len(r.Results))
	if totalSet > 0 {
		fmt.Fprintf(&b, ", %d fields set", totalSet)
	}
	if totalNeedsReview > 0 {
		fmt.Fprintf(&b, ", %d needs_review", totalNeedsReview)
	}
	if totalErrors > 0 {
		fmt.Fprintf(&b, ", %d errors", totalErrors)
	}
	b.WriteString("\n")

	return b.String()
}

// enrichPathForResource returns the default YAML file path for the given resource type.
// Returns an error if the path must be provided explicitly via --path.
func enrichPathForResource(rt enrich.ResourceType) (string, error) {
	switch rt {
	case enrich.ResourceMovie:
		return filepath.Join(data.DefaultPathForDomain(data.DomainNtl), "movie.yml"), nil
	case enrich.ResourceTV:
		return filepath.Join(data.DefaultPathForDomain(data.DomainNtl), "TV.yml"), nil
	case enrich.ResourceBook:
		return "", fmt.Errorf("books have multiple files in %s; use --path to specify one", data.DefaultPathForDomain(data.DomainBooks))
	default:
		return "", fmt.Errorf("no default path for %q; use --path to specify a file", rt)
	}
}
