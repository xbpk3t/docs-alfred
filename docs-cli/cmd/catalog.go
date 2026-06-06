package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	cataloguc "github.com/xbpk3t/docs-alfred/docs-cli/internal/usecase/catalog"
	"github.com/xbpk3t/docs-alfred/pkg/wf"
	gh "github.com/xbpk3t/docs-alfred/service/gh"
)

const cmdCatalog = "catalog"

func newCatalogCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   cmdCatalog,
		Short: "Remote GitHub repo search and cache sync",
	}

	cmd.AddCommand(newCatalogSearchCmd())
	cmd.AddCommand(newCatalogSyncCmd())

	return cmd
}

func newCatalogSearchCmd() *cobra.Command {
	var configURL, cachePath, docsURL, outputFormat string
	var maxAge string

	searchCmd := &cobra.Command{
		Use:   "search [query]",
		Short: "Search GitHub repositories from remote gh.yml",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			query := ""
			if len(args) > 0 {
				query = args[0]
			}

			result, err := cataloguc.RunSearch(cataloguc.SearchInput{
				ConfigURL: configURL,
				CachePath: cachePath,
				Query:     query,
				MaxAge:    maxAge,
			})
			if err != nil {
				return err
			}

			return runCatalogSearchOutput(result.Repos, outputFormat, docsURL)
		},
	}

	searchCmd.Flags().StringVar(&configURL, "url", gh.DefaultConfigURL, "Remote gh.yml URL")
	searchCmd.Flags().StringVar(&cachePath, "cache", gh.DefaultConfigPath, "Local cache path")
	searchCmd.Flags().StringVar(&docsURL, "docs-url", "https://docs.lucc.dev", "Docs base URL")
	searchCmd.Flags().StringVarP(&outputFormat, "output", "o", "plain", "Output format: alfred, plain, raw, rofi")
	searchCmd.Flags().StringVar(&maxAge, "max-age", "24h", "Cache TTL")

	return searchCmd
}

func newCatalogSyncCmd() *cobra.Command {
	var configURL, cachePath string

	syncCmd := &cobra.Command{
		Use:   "sync",
		Short: "Force refresh remote gh.yml cache",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Fprintf(os.Stderr, "Syncing from %s to %s...\n", configURL, cachePath)
			_, err := cataloguc.RunSync(cataloguc.SyncInput{
				ConfigURL: configURL,
				CachePath: cachePath,
			})
			if err != nil {
				return err
			}

			return writeOutput("Sync completed successfully")
		},
	}

	syncCmd.Flags().StringVar(&configURL, "url", gh.DefaultConfigURL, "Remote gh.yml URL")
	syncCmd.Flags().StringVar(&cachePath, "cache", gh.DefaultConfigPath, "Cache write path")

	return syncCmd
}

// ---- output ----

func writeFormatterOutput(format string, data any) error {
	formatter := wf.GetFormatter(format)
	result, err := formatter.Format(data)
	if err != nil {
		return err
	}

	return writeOutput(result)
}

func writeOutput(s string) error {
	_, err := os.Stdout.WriteString(s + "\n")

	return err
}

func runCatalogSearchOutput(repos gh.Repos, outputFormat, docsURL string) error {
	switch outputFormat {
	case "alfred":
		items := gh.FormatAlfredItems(repos, docsURL)

		return writeFormatterOutput("alfred", items)
	case "plain":
		return writeOutput(gh.FormatPlain(repos, docsURL))
	case "raw":
		return writeFormatterOutput("raw", repos)
	case "rofi":
		return writeOutput(gh.FormatRofi(repos))
	default:
		return writeOutput(gh.FormatPlain(repos, docsURL))
	}
}
