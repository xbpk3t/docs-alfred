package cmd

import (
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/hxhac/docs-alfred/pkg/gh"

	"github.com/spf13/cobra"
)

// const ghTpl = `
// {{- each .}}
// ## {{.Type}}
// {{- each .Qs}}
// * {{.}}
// {{- end}}
// {{- each .Repos}}
// ### {{.URL}}
// {{- each .Qs}}
// * {{.}}
// {{- end}}
// {{- end}}
// `

// const ghTpl = `
// {{- range . -}}
// ## {{.Type}}
//
// {{- if .Qs -}}
// - {{ range .Qs }}{{ . }}
// {{- end -}}
// {{- end }}
//
// {{- range .Repos -}}
// ### {{.URL}}
//
// {{- range .Qs -}}
// - {{ . }}
// {{- end -}}
// {{- end -}}
// {{- end -}}
// `

// const ghTpl = `
// {{- range . -}}
// {{"\n"}}
// ## {{.Type}}
// {{"\n"}}
//
// {{- if .Qs -}}
// {{- range .Qs}}
// - {{.}}
// {{- end}}
// {{- end }}
//
// {{- range .Repos -}}
// {{"\n"}}
// ### {{.URL}}
// {{"\n"}}
//
// {{- range .Pix}}
// {{.}}
// {{- end}}
//
// {{- range .Qs}}
// - {{.}}
// {{- end}}
//
// {{- if .Qq -}}
// {{- range .Qq -}}
// {{"\n"}}
// #### {{.Topic}}
// {{- range .Qs}}
// - {{.}}
// {{- end}}
// {{- end}}
// {{- end }}
//
// {{- end -}}
// {{- end -}}
// `

// ghCmd represents the gh command
var ghCmd = &cobra.Command{
	Use:   "gh",
	Short: "A brief description of your command",
	Run: func(cmd *cobra.Command, args []string) {
		f, err := os.ReadFile(cfgFile)
		if err != nil {
			return
		}

		dfo := gh.NewConfigRepos(f)
		df := dfo.FilterReposMD()
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

		for _, d := range df {
			if d.Md {
				res.WriteString(fmt.Sprintf("## %s \n", d.Type))
				if d.Qs != nil {
					res.WriteString(addMarkdownQsFormat(d.Qs))
				}

				for _, repo := range d.Repos {
					res.WriteString(fmt.Sprintf("\n\n### %s\n\n", repo.URL))
					if repo.Qs != nil {
						res.WriteString(addMarkdownQsFormat(repo.Qs))
					}
					if repo.Qq != nil {
						for _, s := range repo.Qq {
							if s.Qs != nil {
								res.WriteString(fmt.Sprintf("\n\n#### %s \n\n", s.Topic))
								res.WriteString(addMarkdownQsFormat(s.Qs))
							}
						}
					}
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

func addMarkdownQsFormat(qs gh.Qs) string {
	var builder strings.Builder
	// builder.WriteString("<dl>")
	for _, q := range qs {
		if q.X == "" {
			builder.WriteString(fmt.Sprintf("- %s\n", q.Q))
		} else {
			builder.WriteString(fmt.Sprintf("\n\n<details>\n<summary>%s</summary>\n\n%s\n\n</details>\n\n", q.Q, q.X))
		}
	}
	// builder.WriteString("</dl>")

	return builder.String()
}

func init() {
	rootCmd.AddCommand(ghCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// ghCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// ghCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
