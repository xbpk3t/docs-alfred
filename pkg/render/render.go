package render

import (
	"fmt"
	"github.com/olekukonko/tablewriter"
	"github.com/xbpk3t/docs-alfred/pkg"
	"strings"
)

// Docusaurus admonitions 常量
const (
	AdmonitionTip    = "tip"
	AdmonitionInfo   = "info"
	AdmonitionWarn   = "warning"
	AdmonitionDanger = "danger"
)

// ContentRenderer 定义渲染器接口
type MarkdownRender interface {
	Render(data []byte) (string, error)
}

// MarkdownRenderer Markdown渲染器
type MarkdownRenderer struct {
	builder strings.Builder
}

func NewMarkdownRenderer() *MarkdownRenderer {
	return &MarkdownRenderer{
		builder: strings.Builder{},
	}
}

// Write 写入内容
func (m *MarkdownRenderer) Write(s string) {
	m.builder.WriteString(s)
}

// String 获取结果
func (m *MarkdownRenderer) String() string {
	return m.builder.String()
}

// RenderHeader 渲染标题
func (m *MarkdownRenderer) RenderHeader(level int, text string) {
	m.Write(fmt.Sprintf("%s %s\n", strings.Repeat("#", level), text))
}

// RenderLink 渲染链接
func (m *MarkdownRenderer) RenderLink(text, url string) string {
	return fmt.Sprintf("[%s](%s)", text, url)
}

// RenderList 渲染列表项
func (m *MarkdownRenderer) RenderListItem(text string) {
	m.Write(fmt.Sprintf("- %s\n", text))
}

// RenderFold 渲染折叠块
func (m *MarkdownRenderer) RenderFold(summary, details string) {
	m.Write(fmt.Sprintf("\n\n<details>\n<summary>%s</summary>\n\n%s\n\n</details>\n\n",
		summary, details))
}

// RenderCodeBlock 渲染代码块
func (m *MarkdownRenderer) RenderCodeBlock(language, code string) {
	m.Write(fmt.Sprintf("```%s\n%s\n```\n", language, code))
}

// RenderImageWithFigcaption 渲染带有图片说明的图片
func (m *MarkdownRenderer) RenderImageWithFigcaption(url string) {
	title := extractTitleFromURL(url)
	m.Write(fmt.Sprintf("![image](%s)\n<center>*%s*</center>\n\n", url, title))
}

// extractTitleFromURL 从 URL 中提取标题 (私有方法)
func extractTitleFromURL(url string) string {
	parts := strings.Split(url, "/")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return url
}

// RenderAdmonitions 渲染提示块
func (m *MarkdownRenderer) RenderAdmonitions(admonitionType, title, rex string) {
	if title == "" {
		title = strings.ToUpper(admonitionType)
	}

	m.Write("\n---\n")
	m.Write(fmt.Sprintf(":::%s[%s]\n\n", admonitionType, title))
	m.Write(rex)
	m.Write("\n\n:::\n\n")
}

// RenderURLTable 渲染URL表格
func (r *MarkdownRenderer) RenderURLTable(items []pkg.URLInfo, headers []string) string {
	if len(items) == 0 {
		return ""
	}

	var res strings.Builder
	data := make([][]string, len(items))
	for i, item := range items {
		data[i] = []string{
			fmt.Sprintf("[%s](%s)", item.GetDisplayName(), item.GetLink()),
			item.Des,
		}
	}

	r.RenderMarkdownTable(headers, &res, data)
	return res.String()
}

// RenderMarkdownTable 封装了创建和渲染Markdown表格的逻辑
func (m *MarkdownRenderer) RenderMarkdownTable(header []string, res *strings.Builder, data [][]string) {
	table := tablewriter.NewWriter(res)
	table.SetAutoWrapText(false)
	table.SetHeader(header)
	table.SetBorders(tablewriter.Border{Left: true, Top: false, Right: true, Bottom: false})
	table.SetCenterSeparator("|")
	table.AppendBulk(data)
	table.Render()
}
