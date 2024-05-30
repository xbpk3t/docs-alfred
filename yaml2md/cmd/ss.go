package cmd

import (
	"fmt"
	"log/slog"
	"os"
	"slices"
	"strings"

	"github.com/hxhac/docs-alfred/workflow/pkg/ss"
	"github.com/spf13/cobra"
)

// ssCmd represents the ss command
var ssCmd = &cobra.Command{
	Use:   "ss",
	Short: "A brief description of your command",
	Run: func(cmd *cobra.Command, args []string) {
		f, err := os.ReadFile(cfgFile)
		if err != nil {
			return
		}

		df := ss.NewSS(f)
		// df := dfo.FilterReposMD()
		// tmpl := template.Must(template.New("").Parse(ghTpl))
		//
		// file, err := os.Create(targetFile)
		// if err != nil {
		// 	log.Fatal(err)
		// }
		// defer file.Close()
		//
		// err = tmpl.Execute(file, df)
		// if err != nil {
		// 	log.Fatal(err)
		// }

		var res strings.Builder

		var ss []string

		for _, d := range df {
			if !slices.Contains(ss, d.Tag) {
				ss = append(ss, d.Tag)
				res.WriteString(fmt.Sprintf("## %s \n", d.Tag))
				res.WriteString(fmt.Sprintf("### %s \n", d.Type))
				if d.Qs != nil {
					res.WriteString(addMarkdownQsFormatSS(d.Qs))
				}
			} else {
				res.WriteString(fmt.Sprintf("### %s \n", d.Type))
				if d.Qs != nil {
					res.WriteString(addMarkdownQsFormatSS(d.Qs))
				}
			}
		}

		err = os.WriteFile(targetFile, []byte(res.String()), os.ModePerm)
		if err != nil {
			return
		}

		slog.Info(fmt.Sprintf("Markdown output has been written to %s", targetFile))
	},
}

func init() {
	rootCmd.AddCommand(ssCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// ssCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// ssCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}

func addMarkdownQsFormatSS(qs ss.Qs) string {
	var builder strings.Builder
	// builder.WriteString("<dl>")
	for _, q := range qs {
		if q.X == "" {
			builder.WriteString(fmt.Sprintf("- %s\n", q.Q))
		} else {
			builder.WriteString(fmt.Sprintf("\n\n<details>\n<summary>%s</summary>\n\n%s\n%s\n\n</details>\n\n", q.Q, q.U, q.X))
		}
	}
	// builder.WriteString("</dl>")

	return builder.String()
}
