package cmd

import (
	"fmt"
	"log/slog"
	"os"
	"slices"
	"strings"

	"github.com/hxhac/docs-alfred/pkg/work"

	"github.com/spf13/cobra"
)

// workCmd represents the work command
var workCmd = &cobra.Command{
	Use:   "works",
	Short: "A brief description of your command",
	Run: func(cmd *cobra.Command, args []string) {
		f, err := os.ReadFile(cfgFile)
		if err != nil {
			return
		}

		dfo := work.NewConfigQs(f)

		var res strings.Builder
		var ss []string

		for _, d := range dfo {
			if !slices.Contains(ss, d.Tag) {
				ss = append(ss, d.Tag)

				if strings.EqualFold(d.Tag, d.Type) {
					res.WriteString(fmt.Sprintf("## %s \n", d.Tag))
				} else {
					res.WriteString(fmt.Sprintf("## %s \n", d.Tag))
					res.WriteString(fmt.Sprintf("### %s \n", d.Type))
				}

				if d.Qs != nil {
					res.WriteString(addMarkdownQsFormatWorks(d.Qs))
				}
			} else {
				res.WriteString(fmt.Sprintf("### %s \n", d.Type))
				if d.Qs != nil {
					res.WriteString(addMarkdownQsFormatWorks(d.Qs))
				}
			}
		}

		tf, _ := strings.CutSuffix(cfgFile, ".yml")
		targetFile := fmt.Sprintf("%s.md", tf)
		err = os.WriteFile(fmt.Sprintf("%s.md", targetFile), []byte(res.String()), os.ModePerm)
		if err != nil {
			return
		}

		slog.Info("Markdown output has been written to", slog.String("File", targetFile))
	},
}

// func addMarkdownQsFormat(qs gh.Qs) string {
// 	var builder strings.Builder
// 	// builder.WriteString("<dl>")
// 	for _, q := range qs {
// 		if q.X == "" {
// 			builder.WriteString(fmt.Sprintf("- %s\n", q.Q))
// 		} else {
// 			builder.WriteString(fmt.Sprintf("\n\n<details>\n<summary>%s</summary>\n\n%s\n\n</details>\n\n", q.Q, q.X))
// 		}
// 	}
// 	// builder.WriteString("</dl>")
//
// 	return builder.String()
// }

func init() {
	rootCmd.AddCommand(workCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// workCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// workCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}

// 用来渲染qs
func addMarkdownQsFormatWorks(qs work.Qs) string {
	var builder strings.Builder
	// builder.WriteString("<dl>")
	for _, q := range qs {
		if q.X == "" {
			if q.U != "" {
				builder.WriteString(fmt.Sprintf("- [%s](%s)\n", q.Q, q.U))
			} else {
				builder.WriteString(fmt.Sprintf("- %s\n", q.Q))
			}
		} else {
			if q.U != "" {
				builder.WriteString(fmt.Sprintf("\n\n<details>\n<summary>[%s](%s)</summary>\n\n%s\n\n</details>\n\n", q.Q, q.U, q.X))
			} else {
				builder.WriteString(fmt.Sprintf("\n\n<details>\n<summary>%s</summary>\n\n%s\n\n</details>\n\n", q.Q, q.X))
			}
		}
	}
	// builder.WriteString("</dl>")

	return builder.String()
}
