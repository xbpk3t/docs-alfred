package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/xbpk3t/docs-alfred/pkg/wf"
	gh "github.com/xbpk3t/docs-alfred/service/gh"
)

func newGhCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "gh",
		Short: "Remote GitHub repo search and cache sync",
	}

	cmd.AddCommand(newGhSearchCmd())
	cmd.AddCommand(newGhSyncCmd())

	return cmd
}

func newGhSearchCmd() *cobra.Command {
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

			manager := gh.NewManager(cachePath, configURL)
			if err := manager.Load(); err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			repos := manager.Filter(query)

			switch outputFormat {
			case "alfred":
				return writeAlfredOutput(repos, docsURL)
			case "plain":
				return writeOutput(formatPlainOutput(repos, docsURL))
			case "raw":
				return writeRawJSON(repos)
			case "rofi":
				return writeRofiOutput(repos)
			default:
				return writeOutput(formatPlainOutput(repos, docsURL))
			}
		},
	}

	searchCmd.Flags().StringVar(&configURL, "url", gh.DefaultConfigURL, "Remote gh.yml URL")
	searchCmd.Flags().StringVar(&cachePath, "cache", gh.DefaultConfigPath, "Local cache path")
	searchCmd.Flags().StringVar(&docsURL, "docs-url", "https://docs.lucc.dev", "Docs base URL")
	searchCmd.Flags().StringVarP(&outputFormat, "output", "o", "plain", "Output format: alfred, plain, raw, rofi")
	searchCmd.Flags().StringVar(&maxAge, "max-age", "24h", "Cache TTL")
	_ = viper.BindPFlag("output", searchCmd.Flags().Lookup("output"))

	return searchCmd
}

func newGhSyncCmd() *cobra.Command {
	var configURL, cachePath string

	syncCmd := &cobra.Command{
		Use:   "sync",
		Short: "Force refresh remote gh.yml cache",
		RunE: func(cmd *cobra.Command, args []string) error {
			manager := gh.NewManager(cachePath, configURL)
			fmt.Fprintf(os.Stderr, "Syncing from %s to %s...\n", configURL, cachePath)
			if err := manager.Sync(); err != nil {
				return fmt.Errorf("sync failed: %w", err)
			}

			return writeOutput("Sync completed successfully")
		},
	}

	syncCmd.Flags().StringVar(&configURL, "url", gh.DefaultConfigURL, "Remote gh.yml URL")
	syncCmd.Flags().StringVar(&cachePath, "cache", gh.DefaultConfigPath, "Cache write path")

	return syncCmd
}

// ---- format helpers ----

func writeAlfredOutput(repos gh.Repos, docsURL string) error {
	var items []wf.AlfredItem

	for _, repo := range repos {
		item := wf.AlfredItem{
			Title:    repo.FullName(),
			Subtitle: repo.GetDes(),
			Arg:      repo.GetURL(),
			Valid:    true,
		}

		switch {
		case repo.HasQs() && repo.Doc != "":
			item.Icon = &wf.AlfredIcon{Path: gh.IconQsDoc}
		case repo.HasQs():
			item.Icon = &wf.AlfredIcon{Path: gh.IconQs}
		case repo.Doc != "":
			item.Icon = &wf.AlfredIcon{Path: gh.IconDoc}
		default:
			item.Icon = &wf.AlfredIcon{Path: gh.IconSearch}
		}

		item.Mods = make(map[string]*wf.AlfredMod)
		if repo.Doc != "" {
			docURL := fmt.Sprintf("%s/#/%s", docsURL, repo.Doc)
			item.Mods["cmd"] = &wf.AlfredMod{
				Valid:    true,
				Arg:      docURL,
				Subtitle: "Open documentation",
			}
		}

		items = append(items, item)
	}

	output := wf.AlfredOutput{Items: items}
	formatter := wf.GetFormatter("alfred")
	result, err := formatter.Format(output)
	if err != nil {
		return err
	}

	return writeOutput(result)
}

func formatPlainOutput(repos gh.Repos, docsURL string) string {
	var sb strings.Builder

	for i, repo := range repos {
		if i > 0 {
			sb.WriteString("\n\n")
		}
		sb.WriteString(fmt.Sprintf("repo: %s\n", repo.GetURL()))
		if repo.GetDes() != "" {
			sb.WriteString(fmt.Sprintf("desc: %s\n", repo.GetDes()))
		}
		if repo.Doc != "" {
			docURL := fmt.Sprintf("%s/#/%s", docsURL, repo.Doc)
			sb.WriteString(fmt.Sprintf("doc: %s\n", repo.Doc))
			sb.WriteString(fmt.Sprintf("docs: %s\n", docURL))
		}
		if repo.Type != "" {
			typeInfo := repo.Type
			if repo.Tag != "" {
				typeInfo = fmt.Sprintf("%s#%s", repo.Tag, repo.Type)
			}
			sb.WriteString(fmt.Sprintf("type: %s\n", typeInfo))
		}
	}

	return sb.String()
}

func writeRawJSON(repos gh.Repos) error {
	formatter := wf.GetFormatter("raw")
	result, err := formatter.Format(repos)
	if err != nil {
		return err
	}

	return writeOutput(result)
}

func writeRofiOutput(repos gh.Repos) error {
	var lines []string
	for _, repo := range repos {
		line := repo.FullName()
		if repo.GetDes() != "" {
			line = fmt.Sprintf("%s - %s", line, repo.GetDes())
		}
		lines = append(lines, line)
	}

	return writeOutput(strings.Join(lines, "\n"))
}

func writeOutput(s string) error {
	_, err := os.Stdout.WriteString(s + "\n")

	return err
}
