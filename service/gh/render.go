package gh

import (
	"strings"

	"github.com/xbpk3t/docs-alfred/pkg/parser"
	"github.com/xbpk3t/docs-alfred/pkg/render"
)

// GhRenderer Markdown渲染器
type GhRenderer struct {
	render.MarkdownRenderer
	Config      ConfigRepos
	repoConfigs []repoRenderConfig
}

// 定义仓库类型和对应的渲染配置
type repoRenderConfig struct {
	repos          Repos
	admonitionType render.AdmonitionType
	title          string
}

// Renderer 相关方法
func NewGhRenderer() *GhRenderer {
	return &GhRenderer{
		repoConfigs: []repoRenderConfig{
			{admonitionType: render.AdmonitionTip, title: "Sub Repos"},
			{admonitionType: render.AdmonitionWarning, title: "Replaced Repos"},
			{admonitionType: render.AdmonitionInfo, title: "Related Repos"},
		},
	}
}

func (g *GhRenderer) Render(data []byte) (string, error) {
	config, err := parser.NewParser[ConfigRepos](data).ParseSingle()
	if err != nil {
		return "", err
	}
	g.Config = config
	return g.renderContent()
}

func (g *GhRenderer) renderContent() (string, error) {
	for _, repo := range g.Config {
		g.RenderHeader(2, repo.Type)
		g.RenderRepositoriesAsMarkdownTable(repo.Repos)
		g.renderRepos(repo.Repos)
	}
	return g.String(), nil
}

func (g *GhRenderer) renderRepos(repos Repos) {
	for _, repo := range repos {
		if repo.Qs != nil {
			g.RenderHeader(3, g.RenderLink(repo.FullName(), repo.URL))
			g.renderSubComponents(repo)
			g.renderQuestions(repo.Qs)
		}
	}
}

func (g *GhRenderer) renderSubComponents(repo Repository) {

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

func (g *GhRenderer) renderSubRepoComponent(config repoRenderConfig) {
	content := g.RepositoriesAsMarkdownTable(config.repos)
	g.RenderAdmonition(config.admonitionType, config.title, content)
}

func (g *GhRenderer) renderQuestions(qs Questions) {
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
