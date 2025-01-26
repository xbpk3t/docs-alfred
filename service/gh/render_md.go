package gh

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/olekukonko/tablewriter"
	"github.com/samber/lo"
	"github.com/xbpk3t/docs-alfred/pkg/errcode"
	"github.com/xbpk3t/docs-alfred/pkg/parser"
	"github.com/xbpk3t/docs-alfred/pkg/render"
)

// GithubMarkdownRender Markdown渲染器
type GithubMarkdownRender struct {
	processor *render.FileProcessor
	render.MarkdownRenderer
	Config      ConfigRepos
	repoConfigs []repoRenderConfig
}

// 定义仓库类型和对应的渲染配置
type repoRenderConfig struct {
	admonitionType render.AdmonitionType
	title          string
	repos          Repos
}

// SetProcessor 设置文件处理器
func (g *GithubMarkdownRender) SetProcessor(processor *render.FileProcessor) {
	g.processor = processor
}

// NewGithubMarkdownRender 创建新的渲染器
func NewGithubMarkdownRender() *GithubMarkdownRender {
	return &GithubMarkdownRender{
		processor: &render.FileProcessor{},
		repoConfigs: []repoRenderConfig{
			{admonitionType: render.AdmonitionTip, title: "Sub Repos"},
			{admonitionType: render.AdmonitionWarning, title: "Replaced Repos"},
			{admonitionType: render.AdmonitionInfo, title: "Related Repos"},
		},
	}
}

// RenderToFile 渲染并写入文件
func (g *GithubMarkdownRender) RenderToFile() error {
	if g.processor.IsMerge {
		return g.renderMerged()
	}
	return g.renderSeparate()
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

// renderMerged 合并渲染
func (g *GithubMarkdownRender) renderMerged() error {
	// 读取并合并所有YAML文件
	files, err := os.ReadDir(g.processor.SrcDir)
	if err != nil {
		return errcode.WithError(errcode.ErrListDir, err)
	}

	var mergedData []byte
	for _, file := range files {
		if file.IsDir() || !strings.HasSuffix(file.Name(), ".yml") {
			continue
		}

		data, err := os.ReadFile(filepath.Join(g.processor.SrcDir, file.Name()))
		if err != nil {
			return errcode.WithError(errcode.ErrReadFile, err)
		}
		mergedData = append(mergedData, data...)
	}

	// 渲染内容
	content, err := g.Render(mergedData)
	if err != nil {
		return err
	}

	// 确定输出文件名
	outputFn := g.processor.TargetFile
	if outputFn == "" {
		outputFn = fmt.Sprintf("%s.md", filepath.Base(g.processor.SrcDir))
	}

	// 写入文件
	return g.writeFile(outputFn, content)
}

// renderSeparate 分别渲染
func (g *GithubMarkdownRender) renderSeparate() error {
	files, err := os.ReadDir(g.processor.SrcDir)
	if err != nil {
		return errcode.WithError(errcode.ErrListDir, err)
	}

	for _, file := range files {
		if file.IsDir() || !strings.HasSuffix(file.Name(), ".yml") {
			continue
		}

		// 读取YAML文件
		data, err := os.ReadFile(filepath.Join(g.processor.SrcDir, file.Name()))
		if err != nil {
			return errcode.WithError(errcode.ErrReadFile, err)
		}

		// 渲染内容
		content, err := g.Render(data)
		if err != nil {
			return err
		}

		// 使用YAML文件名作为MD文件名
		outputFn := strings.TrimSuffix(file.Name(), ".yml") + ".md"

		// 写入文件
		if err := g.writeFile(outputFn, content); err != nil {
			return err
		}
	}

	return nil
}

// writeFile 写入文件
func (g *GithubMarkdownRender) writeFile(filename, content string) error {
	// 创建以srcDir命名的中间目录
	tmpDir := filepath.Join(g.processor.TargetDir, filepath.Base(g.processor.SrcDir))
	if err := os.MkdirAll(tmpDir, 0o755); err != nil {
		return errcode.WithError(errcode.ErrCreateDir, err)
	}

	// 写入文件到中间目录
	tmpFile := filepath.Join(tmpDir, filename)
	if err := os.WriteFile(tmpFile, []byte(content), 0o644); err != nil {
		return errcode.WithError(errcode.ErrWriteFile, err)
	}

	// 移动文件到最终目标目录
	finalFile := filepath.Join(g.processor.TargetDir, filename)
	if err := os.Rename(tmpFile, finalFile); err != nil {
		return errcode.WithError(errcode.ErrFileProcess, err)
	}

	// 删除临时目录
	if err := os.RemoveAll(tmpDir); err != nil {
		return errcode.WithError(errcode.ErrFileProcess, err)
	}

	return nil
}

func (g *GithubMarkdownRender) Render(data []byte) (string, error) {
	config, err := parser.NewParser[ConfigRepos](data).WithFileName(g.processor.GetCurrentFileName()).ParseSingle()
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
