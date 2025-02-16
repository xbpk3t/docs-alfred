package gh

import (
	"fmt"
	"strings"

	"github.com/olekukonko/tablewriter"
	"github.com/samber/lo"
	"github.com/xbpk3t/docs-alfred/pkg/parser"
	"github.com/xbpk3t/docs-alfred/pkg/render"
)

// GithubMarkdownRender Markdown渲染器
type GithubMarkdownRender struct {
	currentFile string
	renderer    render.MarkdownRenderer
	Config      ConfigRepos
	repoConfigs []repoRenderConfig
}

// 定义仓库类型和对应的渲染配置
type repoRenderConfig struct {
	admonitionType render.AdmonitionType
	title          string
	repos          Repos
}

// NewGithubMarkdownRender 创建新的渲染器
func NewGithubMarkdownRender() *GithubMarkdownRender {
	return &GithubMarkdownRender{
		renderer: render.NewMarkdownRenderer(),
		repoConfigs: []repoRenderConfig{
			{admonitionType: render.AdmonitionTip, title: "Sub Repos"},
			{admonitionType: render.AdmonitionWarning, title: "Replaced Repos"},
			{admonitionType: render.AdmonitionInfo, title: "Related Repos"},
		},
	}
}

// GetCurrentFileName 获取当前处理的文件名
func (g *GithubMarkdownRender) GetCurrentFileName() string {
	return g.currentFile
}

// SetCurrentFile 设置当前处理的文件名
func (g *GithubMarkdownRender) SetCurrentFile(filename string) {
	g.currentFile = filename
}

// RenderMarkdownTable 渲染Markdown表格
func (g *GithubMarkdownRender) RenderMarkdownTable(header []string, res *strings.Builder, data [][]string) {
	table := tablewriter.NewWriter(res)
	table.SetAutoWrapText(false)
	table.SetHeader(header)
	table.SetBorders(tablewriter.Border{Left: true, Top: false, Right: true, Bottom: false})
	table.SetCenterSeparator("|")
	table.AppendBulk(data)
	table.Render()
}

func (g *GithubMarkdownRender) Render(data []byte) (string, error) {
	// config, err := parser.NewParser[ConfigRepos](data).WithFileName(g.GetCurrentFileName()).ParseSingle()
	config, err := parser.NewParser[ConfigRepo](data).WithFileName(g.GetCurrentFileName()).ParseFlatten()
	if err != nil {
		return "", err
	}
	g.Config = config
	return g.renderContent()
}

func (g *GithubMarkdownRender) renderContent() (string, error) {
	for _, repo := range g.Config {
		g.RenderHeader(render.HeadingLevel2, repo.Type)
		g.RenderRepositoriesAsMarkdownTable(repo.Repos)
		g.renderRepos(repo.Repos)
	}
	return g.String(), nil
}

func (g *GithubMarkdownRender) renderRepos(repos Repos) {
	for _, repo := range repos {
		if repo.Qs != nil {
			g.RenderHeader(render.HeadingLevel3, g.RenderLink(repo.FullName(), repo.URL))
			g.renderSubComponents(repo)
			g.renderQuestions(repo.Qs)
		}
	}
}

func (g *GithubMarkdownRender) renderSubComponents(repo Repository) {
	reposSlices := []Repos{repo.SubRepos, repo.ReplacedRepos, repo.RelatedRepos}

	for i, repos := range reposSlices {
		if len(repos) > 0 {
			config := g.repoConfigs[i]
			config.repos = repos
			g.renderSubRepoComponent(config)
		}
	}

	if len(repo.Cmd) > 0 {
		g.RenderCodeBlock("shell", strings.Join(repo.Cmd, "\n"))
	}
}

func (g *GithubMarkdownRender) renderSubRepoComponent(config repoRenderConfig) {
	content := g.RepositoriesAsMarkdownTable(config.repos)
	g.RenderAdmonition(config.admonitionType, config.title, content)
}

func (g *GithubMarkdownRender) renderQuestions(qs Questions) {
	for _, q := range qs {
		summary := formatQuestionSummary(q)
		details := formatQuestionDetails(q)
		if details == "" {
			g.RenderListItem(summary)
		} else {
			g.RenderFold(summary, details)
		}
	}
}

// RenderRepositoriesAsMarkdownTable 将仓库列表渲染为Markdown表格
func (g *GithubMarkdownRender) RenderRepositoriesAsMarkdownTable(repos Repos) {
	g.Write(g.RepositoriesAsMarkdownTable(repos))
}

// RepositoriesAsMarkdownTable 将仓库列表渲染为Markdown表格
func (g *GithubMarkdownRender) RepositoriesAsMarkdownTable(repos Repos) string {
	if len(repos) == 0 {
		return ""
	}
	var res strings.Builder
	data := lo.Map(repos, func(item Repository, _ int) []string {
		repoName := item.FullName()
		return []string{fmt.Sprintf("[%s](%s)", repoName, item.URL), item.Des}
	})

	g.RenderMarkdownTable([]string{"Repo", "Des"}, &res, data)
	return res.String()
}

// formatQuestionSummary 格式化问题摘要
func formatQuestionSummary(q Question) string {
	if q.U != "" {
		return fmt.Sprintf("[%s](%s)", q.Q, q.U)
	}
	return q.Q
}

// formatQuestionDetails 格式化问题详情
func formatQuestionDetails(q Question) string {
	var parts []string
	renderer := render.NewMarkdownRenderer()

	// 处理图片
	if len(q.P) > 0 {
		var images strings.Builder
		for _, img := range q.P {
			renderer.RenderImageWithFigcaption(img)
			images.WriteString(renderer.String())
		}
		parts = append(parts, images.String())
	}

	// 处理子问题
	if len(q.S) > 0 {
		var subQuestions strings.Builder
		for _, sq := range q.S {
			subQuestions.WriteString(fmt.Sprintf("- %s\n", sq))
		}
		parts = append(parts, subQuestions.String())
	}

	// 处理答案
	if q.X != "" {
		if len(parts) > 0 {
			parts = append(parts, "---")
		}
		parts = append(parts, q.X)
	}

	return strings.Join(parts, "\n\n")
}

// Write implements writing content
func (g *GithubMarkdownRender) Write(s string) {
	g.renderer.Write(s)
}

// RenderFold implements rendering fold content
func (g *GithubMarkdownRender) RenderFold(summary, details string) {
	g.renderer.RenderFold(summary, details)
}

// String implements getting result
func (g *GithubMarkdownRender) String() string {
	return g.renderer.String()
}

// RenderHeader implements rendering header
func (g *GithubMarkdownRender) RenderHeader(level int, text string) {
	g.renderer.RenderHeader(level, text)
}

// RenderLink implements rendering link
func (g *GithubMarkdownRender) RenderLink(text, url string) string {
	return g.renderer.RenderLink(text, url)
}

// RenderCodeBlock implements rendering code block
func (g *GithubMarkdownRender) RenderCodeBlock(language, code string) {
	g.renderer.RenderCodeBlock(language, code)
}

// RenderAdmonition implements rendering admonition
func (g *GithubMarkdownRender) RenderAdmonition(admonitionType render.AdmonitionType, title, content string) {
	g.renderer.RenderAdmonition(admonitionType, title, content)
}

// RenderListItem implements rendering list item
func (g *GithubMarkdownRender) RenderListItem(text string) {
	g.renderer.RenderListItem(text)
}
