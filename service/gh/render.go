package gh

import (
	"strings"

	"github.com/xbpk3t/docs-alfred/pkg/parser"
	"github.com/xbpk3t/docs-alfred/pkg/render"
)

// GhRenderer Markdown渲染器
type GhRenderer struct {
	render.MarkdownRenderer
	Config ConfigRepos
}

// Renderer 相关方法
func NewGhRenderer() *GhRenderer {
	return &GhRenderer{}
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
	// 渲染子仓库
	if len(repo.SubRepos) > 0 {
		g.renderSubRepos(repo.SubRepos)
	}

	if len(repo.ReplacedRepos) > 0 {
		g.renderReplacedRepos(repo.ReplacedRepos)
	}

	if len(repo.RelatedRepos) > 0 {
		g.renderRelatedRepos(repo.RelatedRepos)
	}

	// 渲染命令
	if len(repo.Cmd) > 0 {
		g.RenderCodeBlock("shell", strings.Join(repo.Cmd, "\n"))
	}
}

func (g *GhRenderer) renderSubRepos(repos Repos) {
	if len(repos) > 0 {
		content := RenderRepositoriesAsMarkdownTable(repos)
		g.RenderAdmonition(render.AdmonitionTip, "SubRepos Repos", content)
	}
}

func (g *GhRenderer) renderReplacedRepos(repos Repos) {
	if len(repos) > 0 {
		content := RenderRepositoriesAsMarkdownTable(repos)
		g.RenderAdmonition(render.AdmonitionWarning, "Replaced Repos", content)
	}
}

func (g *GhRenderer) renderRelatedRepos(repos Repos) {
	if len(repos) > 0 {
		content := RenderRepositoriesAsMarkdownTable(repos)
		g.RenderAdmonition(render.AdmonitionInfo, "Related Repos", content)
	}
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
