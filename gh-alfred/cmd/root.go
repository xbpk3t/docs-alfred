package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/xbpk3t/docs-alfred/gh-alfred/internal/presenter"
	"github.com/xbpk3t/docs-alfred/gh-alfred/internal/usecase"
	"github.com/xbpk3t/docs-alfred/pkg/wf"
	"github.com/xbpk3t/docs-alfred/service/ghindex"
)

// Execute is the entry point for the gh-alfred binary.
func Execute() error {
	return newRootCmd().Execute()
}

func newRootCmd() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "gh-alfred",
		Short: "Alfred GitHub repo search and cache sync",
	}

	rootCmd.AddCommand(newSearchCmd())
	rootCmd.AddCommand(newSyncCmd())
	rootCmd.AddCommand(newExportCmd())
	rootCmd.AddCommand(newValidateCmd())
	rootCmd.SetHelpCommand(&cobra.Command{Hidden: true})

	return rootCmd
}

func newSearchCmd() *cobra.Command {
	var configURL, cachePath, docsURL string
	var maxAge string

	cmd := &cobra.Command{
		Use:   "search [query]",
		Short: "Search GitHub repositories from remote gh.yml for Alfred",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			query := ""
			if len(args) > 0 {
				query = args[0]
			}

			result, err := usecase.RunSearch(usecase.SearchInput{
				ConfigURL: configURL,
				CachePath: cachePath,
				Query:     query,
				MaxAge:    maxAge,
			})
			if err != nil {
				return writeFormatterOutput("alfred", []wf.AlfredItem{{
					Title:    "Alfred index unavailable",
					Subtitle: err.Error(),
					Valid:    false,
					Icon:     &wf.AlfredIcon{Path: presenter.IconError},
				}})
			}

			return runSearchOutput(result.Repos, docsURL)
		},
	}

	cmd.Flags().StringVar(&configURL, "url", ghindex.DefaultConfigURL, "Remote gh.yml URL")
	cmd.Flags().StringVar(&cachePath, "cache", ghindex.DefaultConfigPath, "Local cache path")
	cmd.Flags().StringVar(&docsURL, "docs-url", "https://docs.lucc.dev", "Docs base URL")
	cmd.Flags().StringVar(&maxAge, "max-age", "24h", "Cache TTL")

	return cmd
}

func newSyncCmd() *cobra.Command {
	var configURL, cachePath string

	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Force refresh remote gh.yml cache",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Fprintf(os.Stderr, "Syncing from %s to %s...\n", configURL, cachePath)
			_, err := usecase.RunSync(usecase.SyncInput{
				ConfigURL: configURL,
				CachePath: cachePath,
			})
			if err != nil {
				return err
			}

			return writeOutput("Sync completed successfully")
		},
	}

	cmd.Flags().StringVar(&configURL, "url", ghindex.DefaultConfigURL, "Remote gh.yml URL")
	cmd.Flags().StringVar(&cachePath, "cache", ghindex.DefaultConfigPath, "Cache write path")

	return cmd
}

func newExportCmd() *cobra.Command {
	var src, out string

	cmd := &cobra.Command{
		Use:   "export",
		Short: "Export split data/gh YAML into a validated gh.yml Alfred artifact",
		RunE: func(cmd *cobra.Command, args []string) error {
			result, err := usecase.RunExport(usecase.ExportInput{Src: src, Out: out})
			if err != nil {
				return err
			}

			return writeOutput(fmt.Sprintf("Exported %s (%d repos)", result.OutputPath, result.RepoCount))
		},
	}

	cmd.Flags().StringVar(&src, "src", "data/gh", "Source data/gh directory")
	cmd.Flags().StringVar(&out, "out", "gh.yml", "Output gh.yml path")

	return cmd
}

func newValidateCmd() *cobra.Command {
	var file string

	cmd := &cobra.Command{
		Use:   "validate",
		Short: "Validate a gh.yml Alfred artifact for search",
		RunE: func(cmd *cobra.Command, args []string) error {
			result, err := usecase.RunValidate(usecase.ValidateInput{File: file})
			if err != nil {
				return err
			}

			return writeOutput("Validated " + result.File)
		},
	}

	cmd.Flags().StringVar(&file, "file", "gh.yml", "gh.yml path")

	return cmd
}

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

func runSearchOutput(repos ghindex.Repos, docsURL string) error {
	items := presenter.FormatAlfredItems(repos, docsURL)

	return writeFormatterOutput("alfred", items)
}
