package cmd

import (
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/hxhac/docs-alfred/utils"

	"github.com/samber/lo"

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
			slog.Error(err.Error())
			return
		}

		df := gh.NewConfigRepos(f)
		// dfo := gh.NewConfigRepos(f)
		// df := dfo.FilterReposMD()

		// dfo.IsTypeQsEmpty()
		// 清理掉 Qs == nil 的 Type
		// dfr := FilterRepos(df)

		var res strings.Builder

		for _, d := range df {
			if d.Qs != nil || d.Repos != nil {
				res.WriteString(fmt.Sprintf("## %s \n", d.Type))
			}

			// repo下的所有repo列表
			RenderRepositoriesAsMarkdownTable(d.Repos, &res)

			// type对应的qs
			if d.Qs != nil {
				res.WriteString(addMarkdownQsFormat(d.Qs))
			}
			for _, repo := range d.Repos {
				if repo.Qs != nil {
					repoName, f := strings.CutPrefix(repo.URL, gh.GhURL)
					if !f {
						repoName = ""
					}
					res.WriteString(fmt.Sprintf("\n\n### [%s](%s)\n\n", repoName, repo.URL))

					// 渲染该repo的sub repo
					RenderRepositoriesAsMarkdownTable(repo.Sub, &res)

					if repo.Qs != nil {
						res.WriteString(addMarkdownQsFormat(repo.Qs))
					}
					if repo.Qq != nil {
						for _, s := range repo.Qq {
							if s.Qs != nil {
								res.WriteString(fmt.Sprintf("\n\n#### %s \n\n", s.Topic))

								RenderRepositoriesAsMarkdownTable(s.Sub, &res)

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

// func addMarkdownQsFormat(qs gh.Qs) string {
// 	var builder strings.Builder
// 	// builder.WriteString("<dl>")
// 	for _, q := range qs {
// 		// if q.X == "" {
// 		// 	if q.U != "" {
// 		// 		builder.WriteString(fmt.Sprintf("- [%s](%s)\n", q.Q, q.U))
// 		// 	} else {
// 		// 		builder.WriteString(fmt.Sprintf("- %s\n", q.Q))
// 		// 	}
// 		// } else {
// 		// 	if q.U != "" {
// 		// 		builder.WriteString(fmt.Sprintf("\n\n<details>\n<summary>[%s](%s)</summary>\n\n%s\n\n</details>\n\n", q.Q, q.U, q.X))
// 		// 	} else {
// 		// 		builder.WriteString(fmt.Sprintf("\n\n<details>\n<summary>%s</summary>\n\n%s\n\n</details>\n\n", q.Q, q.X))
// 		// 	}
// 		// }
//
// 		switch {
// 		case q.X == "" && q.U == "" && q.P == "":
// 			builder.WriteString(fmt.Sprintf("- %s\n", q.Q))
// 		case q.X == "" && q.U != "" && q.P == "":
// 			builder.WriteString(fmt.Sprintf("- [%s](%s)\n", q.Q, q.U))
// 		case q.X == "" && q.U == "" && q.P != "":
// 			builder.WriteString(fmt.Sprintf("\n\n<details>\n<summary>%s</summary>\n\n![%s](%s)\n\n</details>\n\n", q.Q, "image", q.P))
//
// 		case q.X != "" && q.U == "" && q.P == "":
// 			builder.WriteString(fmt.Sprintf("\n\n<details>\n<summary>%s</summary>\n\n%s\n\n</details>\n\n", q.Q, q.X))
// 		case q.X != "" && q.U != "" && q.P == "":
// 			builder.WriteString(fmt.Sprintf("\n\n<details>\n<summary>[%s](%s)</summary>\n\n%s\n\n</details>\n\n", q.Q, q.U, q.X))
// 		case q.X != "" && q.U == "" && q.P != "":
// 			builder.WriteString(fmt.Sprintf("\n\n<details>\n<summary>%s</summary>\n\n![%s](%s)\n\n%s\n\n</details>\n\n", q.Q, "image", q.P, q.X))
//
//
// 		case q.X == "" && q.U != "" && q.P != "":
// 			builder.WriteString(fmt.Sprintf("\n\n<details>\n<summary>[%s](%s)</summary>\n\n![%s](%s)\n\n</details>\n\n", q.Q, q.U, "image", q.P))
// 		case q.X != "" && q.U != "" && q.P != "":
// 			builder.WriteString(fmt.Sprintf("\n\n<details>\n<summary>[%s](%s)</summary>\n\n![%s](%s)\n\n%s\n\n</details>\n\n", q.Q, q.U, "image", q.P, q.X))
//
// 		default:
// 			builder.WriteString(fmt.Sprintf("\n\n<details>\n<summary>[%s](%s)</summary>\n\n%s\n\n</details>\n\n", q.Q, q.U, q.X))
// 		}
// 	}
//
// 	return builder.String()
// }

func addMarkdownQsFormat(qs gh.Qs) string {
	var builder strings.Builder

	for _, q := range qs {
		summary := formatSummary(q)
		details := formatDetails(q)
		if details == "" {
			builder.WriteString(fmt.Sprintf("- %s\n", summary))
		} else {
			builder.WriteString(fmt.Sprintf("\n\n<details>\n<summary>%s</summary>\n\n%s\n\n</details>\n\n", summary, details))
		}
	}

	return builder.String()
}

// RenderRepositoriesAsMarkdownTable 将仓库列表渲染为Markdown表格
func RenderRepositoriesAsMarkdownTable(repos []gh.Repository, res *strings.Builder) {
	if len(repos) == 0 {
		return
	}
	// 准备表格数据
	data := lo.Map(repos, func(item gh.Repository, index int) []string {
		repoName, _ := strings.CutPrefix(item.URL, gh.GhURL)
		return []string{fmt.Sprintf("[%s](%s)", repoName, item.URL), item.Des}
	})

	// 渲染Markdown表格
	utils.RenderMarkdownTable(res, data)
}

func formatSummary(q gh.Qt) string {
	if q.U != "" {
		return fmt.Sprintf("[%s](%s)", q.Q, q.U)
	}
	return q.Q
}

func formatDetails(q gh.Qt) string {
	var parts []string
	if len(q.S) != 0 {
		var b strings.Builder
		for _, t := range q.S {
			b.WriteString(fmt.Sprintf("- %s\n", t))
		}
		parts = append(parts, b.String())
	}
	// 在s和x之间插入分隔符
	if len(q.S) != 0 && q.X != "" {
		parts = append(parts, "---")
	}

	if q.X != "" {
		parts = append(parts, q.X)
	}
	if q.P != "" {
		parts = append(parts, fmt.Sprintf("![%s](%s)", "image", q.P))
	}
	return strings.Join(parts, "\n\n")
}

// FilterRepos 过滤掉Repo中Qs为nil的ConfigRepos
func FilterRepos(configRepos gh.ConfigRepos) (filteredRepos gh.ConfigRepos) {
	for _, repoGroup := range configRepos {
		// 过滤掉qs为nil的Repository
		filteredGroup := gh.ConfigRepo{
			Type:  repoGroup.Type,
			Repos: make([]gh.Repository, 0),
			Qs:    make(gh.Qs, 0),
		}
		filteredGroup.Type = repoGroup.Type
		filteredGroup.Qs = repoGroup.Qs
		for _, repo := range repoGroup.Repos {
			if repo.Qs != nil {
				filteredGroup.Repos = append(filteredGroup.Repos, repo)
			}
		}
		// 只有当过滤后的Repositories不为空时，才添加到结果中
		if len(filteredGroup.Repos) > 0 {
			filteredRepos = append(filteredRepos, filteredGroup)
		}
	}
	return filteredRepos
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
