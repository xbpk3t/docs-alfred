package cmd

import (
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/xbpk3t/docs-alfred/utils"

	"github.com/samber/lo"

	"github.com/xbpk3t/docs-alfred/pkg/gh"

	"github.com/spf13/cobra"
)

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

		var res strings.Builder

		for _, d := range df {
			repos := RenderTypeRepos(d)
			res.WriteString(repos.String())
		}

		targetFile := utils.ChangeFileExtFromYamlToMd(cfgFile)
		err = os.WriteFile(targetFile, []byte(res.String()), os.ModePerm)
		if err != nil {
			return
		}

		slog.Info("Markdown output has been written to", slog.String("File", targetFile))
	},
}

// addMarkdownQsFormat 渲染qs
// 分别渲染 summary 和 details，来替换之前 switch...case 写法
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

	if len(q.P) != 0 {
		var b strings.Builder
		for _, s := range q.P {
			b.WriteString(fmt.Sprintf("![%s](%s)\n\n", "image", s))
		}
		parts = append(parts, b.String())
	}

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

	return strings.Join(parts, "\n\n")
}

// FilterRepos 过滤掉Repo中Qs为nil的ConfigRepos
// func FilterRepos(configRepos gh.ConfigRepos) (filteredRepos gh.ConfigRepos) {
// 	for _, repoGroup := range configRepos {
// 		// 过滤掉qs为nil的Repository
// 		filteredGroup := gh.ConfigRepo{
// 			Type:  repoGroup.Type,
// 			Repos: make([]gh.Repository, 0),
// 		}
// 		filteredGroup.Type = repoGroup.Type
// 		for _, repo := range repoGroup.Repos {
// 			if repo.Qs != nil {
// 				filteredGroup.Repos = append(filteredGroup.Repos, repo)
// 			}
// 		}
// 		// 只有当过滤后的Repositories不为空时，才添加到结果中
// 		if len(filteredGroup.Repos) > 0 {
// 			filteredRepos = append(filteredRepos, filteredGroup)
// 		}
// 	}
// 	return filteredRepos
// }

// RenderTypeRepos 渲染整个type
func RenderTypeRepos(d gh.ConfigRepo) (res strings.Builder) {
	if d.Repos != nil {
		res.WriteString(fmt.Sprintf("## %s \n", d.Type))
	}

	// repo下的所有repo列表
	RenderRepositoriesAsMarkdownTable(d.Repos, &res)

	repos := RenderRepos(d.Repos)
	res.WriteString(repos.String())

	return
}

func RenderRepos(repos gh.Repos) (res strings.Builder) {
	for _, repo := range repos {
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
		}
	}

	return
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
