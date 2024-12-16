package gh

import (
	"fmt"
	"github.com/samber/lo"
	"github.com/xbpk3t/docs-alfred/utils"
	"strings"
)

type GhRenderer struct {
	utils.MarkdownRenderer
	Config ConfigRepos
}

// RenderContent 保留原有的渲染逻辑
func (g *GhRenderer) RenderContent() (string, error) {
	for _, repo := range g.Config {
		g.RenderHeader(2, repo.Type)
		g.renderRepos(repo.Repos)
	}
	return g.String(), nil
}

// Render 实现 ContentRenderer 接口
func (g *GhRenderer) Render(data []byte) (string, error) {
	g.Config = NewConfigRepos(data)
	return g.RenderContent()
}

func (g *GhRenderer) renderRepos(repos Repos) {
	for _, repo := range repos {
		if repo.Qs != nil {
			g.RenderHeader(3, g.RenderLink(repo.URL, repo.URL))

			// 渲染子仓库
			if len(repo.Sub) > 0 {
				g.renderSubRepos(repo.Sub)
			}

			// 渲染命令
			if len(repo.Cmd) > 0 {
				g.RenderCodeBlock("shell", strings.Join(repo.Cmd, "\n"))
			}

			// 渲染问题
			g.renderQuestions(repo.Qs)
		}
	}
}

// renderSubRepos 渲染子仓库
func (g *GhRenderer) renderSubRepos(repos Repos) {
	if len(repos) > 0 {
		g.Write(utils.RenderMarkdownAdmonitions(utils.AdmonitionTip, "Sub Repos", RenderRepositoriesAsMarkdownTable(repos)))
	}
}

// renderQuestions 渲染问题
func (g *GhRenderer) renderQuestions(qs Qs) {
	for _, q := range qs {
		summary := formatSummary(q)
		details := formatDetails(q)
		if details == "" {
			g.RenderListItem(summary)
		} else {
			g.RenderFold(summary, details)
		}
	}
}

// RenderRepositoriesAsMarkdownTable 将仓库列表渲染为Markdown表格
func RenderRepositoriesAsMarkdownTable(repos []Repository) string {
	if len(repos) == 0 {
		return ""
	}
	var res strings.Builder
	// 准备表格数据
	data := lo.Map(repos, func(item Repository, index int) []string {
		repoName, _ := strings.CutPrefix(item.URL, GhURL)
		return []string{fmt.Sprintf("[%s](%s)", repoName, item.URL), item.Des}
	})

	// 渲染Markdown表格
	utils.RenderMarkdownTable([]string{"Repo", "Des"}, &res, data)
	return res.String()
}

func RenderCmdAsMarkdownTable(repo Repository) string {
	if len(repo.Cmd) == 0 {
		return ""
	}
	var res strings.Builder
	data := lo.Map(repo.Cmd, func(item string, index int) []string {
		return []string{fmt.Sprintf("`%s`", item)}
	})
	utils.RenderMarkdownTable([]string{"Commands"}, &res, data)
	return res.String()
}

func RenderCmdAsCodeBlock(repo Repository) string {
	if len(repo.Cmd) == 0 {
		return ""
	}
	var res strings.Builder
	res.WriteString("\n```shell\n")
	res.WriteString(strings.Join(repo.Cmd, "\n"))
	res.WriteString("\n```")
	return res.String()
}

func RenderDocIcon() string {
	return "[![GitHub](https://icongr.am/feather/github.svg)](https://www.github.com)\n"
}

func formatSummary(q Qt) string {
	if q.U != "" {
		return fmt.Sprintf("[%s](%s)", q.Q, q.U)
	}
	return q.Q
}

func formatDetails(q Qt) string {
	var parts []string

	if len(q.P) != 0 {
		var b strings.Builder
		for _, s := range q.P {
			b.WriteString(utils.RenderMarkdownImageWithFigcaption(s))
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

	if len(q.S) != 0 && q.X != "" {
		parts = append(parts, "---")
	}

	if q.X != "" {
		parts = append(parts, q.X)
	}

	return strings.Join(parts, "\n\n")
}

// RenderTypeRepos 渲染整个type
func RenderTypeRepos(d ConfigRepo) (res strings.Builder) {
	if d.Repos != nil {
		res.WriteString(fmt.Sprintf("## %s \n", d.Type))
	}

	// repo下的所有repo列表
	res.WriteString(RenderRepositoriesAsMarkdownTable(d.Repos))

	repos := RenderRepos(d.Repos)
	res.WriteString(repos.String())

	return
}

func RenderRepos(repos Repos) (res strings.Builder) {
	for _, repo := range repos {
		if repo.Qs != nil {
			repoName, f := strings.CutPrefix(repo.URL, GhURL)
			if !f {
				repoName = ""
			}
			res.WriteString(fmt.Sprintf("\n\n### [%s](%s)\n\n", repoName, repo.URL))
			flag := false

			// 渲染该repo的sub repos
			if len(repo.Sub) != 0 {
				flag = true
				res.WriteString(utils.RenderMarkdownAdmonitions(utils.AdmonitionTip, "Sub Repos", RenderRepositoriesAsMarkdownTable(repo.Sub)))
			}
			// 渲染该repo的rep repos
			if len(repo.Rep) != 0 {
				flag = true
				res.WriteString(utils.RenderMarkdownAdmonitions(utils.AdmonitionWarn, "Replaced Repos",
					RenderRepositoriesAsMarkdownTable(repo.Rep)))
			}
			// 渲染cmds
			if len(repo.Cmd) != 0 {
				flag = true
				res.WriteString(RenderCmdAsCodeBlock(repo))
			}
			if flag {
				res.WriteString("\n\n---\n\n")
			}

			if repo.Qs != nil {
				res.WriteString(addMarkdownQsFormat(repo.Qs))
			}
		}
	}

	return
}

// addMarkdownQsFormat 渲染qs
func addMarkdownQsFormat(qs Qs) string {
	var builder strings.Builder

	for _, q := range qs {
		summary := formatSummary(q)
		details := formatDetails(q)
		if details == "" {
			builder.WriteString(fmt.Sprintf("- %s\n", summary))
		} else {
			builder.WriteString(utils.RenderMarkdownFold(summary, details))
		}
	}

	return builder.String()
}
