package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
	"github.com/xbpk3t/docs-alfred/internal/dataops"
	"github.com/xbpk3t/docs-alfred/service/enrich"
)

type enrichFlags struct {
	path   string
	cache  string
	dryRun bool
}

func newEnrichCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "enrich <resource>",
		Short: "Enrich YAML metadata using external APIs",
		Long: `Enrich YAML metadata files (movie, tv, book) with data from external structured APIs.

Supported resources:
  movie  — TMDB (title, year, director, cast)
  tv     — TMDB (title, year, creators, cast)
  book   — Google Books + Open Library (title, author, year, subtitle)

API keys are read from environment variables:
  TMDB_API_KEY         — required for movie and tv
  GOOGLE_CLOUD_API_KEY — required for book`,
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			resource, err := parseEnrichResourceArg(args[0])
			if err != nil {
				return err
			}

			flags := parseEnrichFlags(cmd)

			return runEnrich(resource, flags)
		},
	}

	addEnrichFlags(cmd)

	return cmd
}

func addEnrichFlags(cmd *cobra.Command) {
	cmd.Flags().String("path", "", "YAML file path to enrich (default: auto-resolved for movie/tv)")
	cmd.Flags().String("cache", "", "Cache file path (default: /tmp/enrich_cache_<resource>.json)")
	cmd.Flags().Bool("dry-run", false, "Report changes without modifying files")
}

func parseEnrichFlags(cmd *cobra.Command) enrichFlags {
	path, _ := cmd.Flags().GetString("path")
	cache, _ := cmd.Flags().GetString("cache")
	dryRun, _ := cmd.Flags().GetBool("dry-run")

	return enrichFlags{
		path:   path,
		cache:  cache,
		dryRun: dryRun,
	}
}

func parseEnrichResourceArg(value string) (enrich.ResourceType, error) {
	rt := enrich.ResourceType(value)
	switch rt {
	case enrich.ResourceMovie, enrich.ResourceTV, enrich.ResourceBook:
		return rt, nil
	default:
		return "", fmt.Errorf("unsupported enrichment resource %q (supported: movie, tv, book)", value)
	}
}

func runEnrich(resource enrich.ResourceType, flags enrichFlags) error {
	cachePath := flags.cache
	if cachePath == "" {
		cachePath = fmt.Sprintf("/tmp/enrich_cache_%s.json", resource)
	}

	apiKey, err := resolveAPIKey(resource)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle OS signals for graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		select {
		case <-sigCh:
			cancel()
		case <-ctx.Done():
		}
	}()

	result, err := dataops.RunEnrich(ctx, &dataops.EnrichInput{
		Resource: resource,
		Path:     flags.path,
		Cache:    cachePath,
		DryRun:   flags.dryRun,
		APIKey:   apiKey,
	})
	if err != nil {
		return fmt.Errorf("enrich failed: %w", err)
	}

	report := dataops.FormatEnrichReport(result.Report)
	fmt.Fprint(os.Stderr, report)

	return nil
}

// resolveAPIKey reads the API key for the given resource from environment.
func resolveAPIKey(rt enrich.ResourceType) (string, error) {
	envVar, label := envVarForResource(rt)
	key := os.Getenv(envVar)
	if key == "" {
		return "", fmt.Errorf("%s API key not set: export %s", label, envVar)
	}

	return key, nil
}

func envVarForResource(rt enrich.ResourceType) (string, string) {
	switch rt {
	case enrich.ResourceMovie, enrich.ResourceTV:
		return "TMDB_API_KEY", "TMDB"
	case enrich.ResourceBook:
		return "GOOGLE_CLOUD_API_KEY", "Google Books"
	default:
		return "", ""
	}
}
