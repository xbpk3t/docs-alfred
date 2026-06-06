package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	alfreduc "github.com/xbpk3t/docs-alfred/docs-cli/internal/usecase/alfred"
	"github.com/xbpk3t/docs-alfred/pkg/wf"
	gh "github.com/xbpk3t/docs-alfred/service/gh"
)

const cmdAlfred = "alfred"

func newAlfredCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   cmdAlfred,
		Short: "Alfred GitHub repo search and cache sync",
	}

	cmd.AddCommand(newAlfredSearchCmd())
	cmd.AddCommand(newAlfredSyncCmd())
	cmd.AddCommand(newAlfredExportCmd())
	cmd.AddCommand(newAlfredValidateCmd())

	return cmd
}

func newAlfredSearchCmd() *cobra.Command {
	var configURL, cachePath, docsURL string
	var maxAge string

	searchCmd := &cobra.Command{
		Use:   "search [query]",
		Short: "Search GitHub repositories from remote gh.yml for Alfred",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			query := ""
			if len(args) > 0 {
				query = args[0]
			}

			result, err := alfreduc.RunSearch(alfreduc.SearchInput{
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
					Icon:     &wf.AlfredIcon{Path: gh.IconError},
				}})
			}

			return runAlfredSearchOutput(result.Repos, docsURL)
		},
	}

	searchCmd.Flags().StringVar(&configURL, "url", gh.DefaultConfigURL, "Remote gh.yml URL")
	searchCmd.Flags().StringVar(&cachePath, "cache", gh.DefaultConfigPath, "Local cache path")
	searchCmd.Flags().StringVar(&docsURL, "docs-url", "https://docs.lucc.dev", "Docs base URL")
	searchCmd.Flags().StringVar(&maxAge, "max-age", "24h", "Cache TTL")

	return searchCmd
}

func newAlfredSyncCmd() *cobra.Command {
	var configURL, cachePath string

	syncCmd := &cobra.Command{
		Use:   "sync",
		Short: "Force refresh remote gh.yml cache",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Fprintf(os.Stderr, "Syncing from %s to %s...\n", configURL, cachePath)
			_, err := alfreduc.RunSync(alfreduc.SyncInput{
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

func newAlfredExportCmd() *cobra.Command {
	var src, out string

	exportCmd := &cobra.Command{
		Use:   "export",
		Short: "Export split data/gh YAML into a validated gh.yml Alfred artifact",
		RunE: func(cmd *cobra.Command, args []string) error {
			result, err := alfreduc.RunExport(alfreduc.ExportInput{Src: src, Out: out})
			if err != nil {
				return err
			}

			return writeOutput(fmt.Sprintf("Exported %s (%d repos)", result.OutputPath, result.RepoCount))
		},
	}

	exportCmd.Flags().StringVar(&src, "src", "data/gh", "Source data/gh directory")
	exportCmd.Flags().StringVar(&out, "out", "gh.yml", "Output gh.yml path")

	return exportCmd
}

func newAlfredValidateCmd() *cobra.Command {
	var file string

	validateCmd := &cobra.Command{
		Use:   "validate",
		Short: "Validate a gh.yml Alfred artifact for search",
		RunE: func(cmd *cobra.Command, args []string) error {
			result, err := alfreduc.RunValidate(alfreduc.ValidateInput{File: file})
			if err != nil {
				return err
			}

			return writeOutput("Validated " + result.File)
		},
	}

	validateCmd.Flags().StringVar(&file, "file", "gh.yml", "gh.yml path")

	return validateCmd
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

func runAlfredSearchOutput(repos gh.Repos, docsURL string) error {
	items := gh.FormatAlfredItems(repos, docsURL)

	return writeFormatterOutput("alfred", items)
}
