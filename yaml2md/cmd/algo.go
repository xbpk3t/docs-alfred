package cmd

import (
	"fmt"
	"log/slog"
	"os"
	"slices"
	"strings"

	"github.com/hxhac/docs-alfred/pkg/algo"
	"github.com/spf13/cobra"
)

// algoCmd represents the algo command
var algoCmd = &cobra.Command{
	Use:   "algo",
	Short: "A brief description of your command",
	Run: func(cmd *cobra.Command, args []string) {
		f, err := os.ReadFile(cfgFile)
		if err != nil {
			return
		}
		algoData := algo.NewConfigAlgo(f)
		var res strings.Builder
		var ss []string

		for _, ad := range algoData {
			if !slices.Contains(ss, ad.Tag) {
				ss = append(ss, ad.Tag)
				if strings.EqualFold(ad.Tag, ad.Type) {
					res.WriteString(fmt.Sprintf("## %s \n", ad.Tag))
				} else {
					if strings.EqualFold(ad.Tag, ad.Type) {
						res.WriteString(fmt.Sprintf("## %s \n", ad.Tag))
					} else {
						res.WriteString(fmt.Sprintf("## %s \n", ad.Tag))
						res.WriteString(fmt.Sprintf("### %s \n", ad.Type))
					}
				}
				if ad.Repos != nil {
					res.WriteString(addMarkdownQsFormatAlgo(ad.Repos))
				}
			} else {
				res.WriteString(fmt.Sprintf("### %s \n", ad.Type))
				if ad.Repos != nil {
					res.WriteString(addMarkdownQsFormatAlgo(ad.Repos))
				}
			}
		}

		err = os.WriteFile(targetFile, []byte(res.String()), os.ModePerm)
		if err != nil {
			return
		}

		slog.Info("Markdown output has been written to", slog.String("File", targetFile))
	},
}

func init() {
	rootCmd.AddCommand(algoCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// algoCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// algoCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}

func addMarkdownQsFormatAlgo(repo algo.Repo) string {
	var builder strings.Builder
	// builder.WriteString("<dl>")
	for _, r := range repo {
		if r.Sol == "" {
			if r.URL != "" && r.Qs != "" {
				builder.WriteString(fmt.Sprintf("- [%s](%s)\n", r.Qs, r.URL))
			} else if r.Qs != "" {
				builder.WriteString(fmt.Sprintf("- %s\n", r.Qs))
			} else if r.URL != "" {
				builder.WriteString(fmt.Sprintf("- %s\n", r.URL))
			}
		} else {
			if r.URL != "" && r.Doc != "" {
				builder.WriteString(fmt.Sprintf("\n\n<details>\n<summary>%s</summary>\n\nURL: %s\n\nDoc: %s\n\n%s\n\n</details>\n\n", r.Qs, r.URL, r.Doc, r.Sol))
			} else if r.URL != "" {
				builder.WriteString(fmt.Sprintf("\n\n<details>\n<summary>%s</summary>\n\nURL: %s\n\n%s\n\n</details>\n\n", r.Qs, r.URL, r.Sol))
			} else if r.Doc != "" {
				builder.WriteString(fmt.Sprintf("\n\n<details>\n<summary>%s</summary>\n\nDoc: %s\n\n%s\n\n</details>\n\n", r.Qs, r.Doc, r.Sol))
			}
		}
	}
	// builder.WriteString("</dl>")

	return builder.String()
}
