package render

import (
	"fmt"
	"strings"

	"github.com/olekukonko/tablewriter"
)

const (
	HeadingLevel1 = 1
	HeadingLevel2 = 2
	HeadingLevel3 = 3
)

// AdmonitionType 提示框类型
type AdmonitionType string

const (
	AdmonitionTip     AdmonitionType = "tip"
	AdmonitionInfo    AdmonitionType = "info"
	AdmonitionWarning AdmonitionType = "warning"
	AdmonitionDanger  AdmonitionType = "danger"
)

// MarkdownRender 定义渲染器接口
type MarkdownRender interface {
	Render(data []byte) (string, error)
}

// MarkdownRenderer Markdown渲染器
type MarkdownRenderer struct {
	builder strings.Builder
}

func NewMarkdownRenderer() MarkdownRenderer {
	return MarkdownRenderer{
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

// RenderMetadata 渲染元数据
func (m *MarkdownRenderer) RenderMetadata(metadata map[string]string) {
	m.Write("---\n")
	for k, v := range metadata {
		m.Write(fmt.Sprintf("%s: %s\n", k, v))
	}
	m.Write("---\n\n")
}

// RenderImport 渲染导入语句
func (m *MarkdownRenderer) RenderImport(name, path string) {
	m.Write(fmt.Sprintf("import %s from '!!raw-loader!%s';\n\n", name, path))
}

// RenderContainer 渲染容器
func (m *MarkdownRenderer) RenderContainer(content string, style string) {
	m.Write(fmt.Sprintf("\n<CodeBlock language=\"%s\">%s</CodeBlock>\n\n", style, content))
}

// RenderDocusaurusRawLoader 渲染Docusaurus原始加载器
func (m *MarkdownRenderer) RenderDocusaurusRawLoader(name, path string) {
	m.RenderImport(name, path)
	m.RenderContainer("{"+name+"}", "yaml")
}

// RenderListItem 渲染列表项
func (m *MarkdownRenderer) RenderListItem(text string) {
	m.Write(fmt.Sprintf("- %s\n", text))
}

// RenderFold 渲染折叠块
func (m *MarkdownRenderer) RenderFold(summary, details string) {
	m.Write(fmt.Sprintf("<details>\n<summary>%s</summary>\n\n%s\n\n</details>\n\n", summary, details))
}

// RenderAdmonition 渲染提示块
func (m *MarkdownRenderer) RenderAdmonition(admonitionType AdmonitionType, title, content string) {
	m.Write(fmt.Sprintf(":::%s[%s]\n\n%s\n\n:::\n\n", admonitionType, title, content))
}

// RenderMarkdownTable 渲染Markdown表格
func (m *MarkdownRenderer) RenderMarkdownTable(header []string, data [][]string) {
	table := tablewriter.NewWriter(&m.builder)
	table.SetAutoWrapText(false)
	table.SetHeader(header)
	table.SetBorders(tablewriter.Border{Left: true, Top: false, Right: true, Bottom: false})
	table.SetCenterSeparator("|")
	table.AppendBulk(data)
	table.Render()
	m.Write("\n")
}

// RenderCodeBlock 渲染代码块
func (m *MarkdownRenderer) RenderCodeBlock(language, code string) {
	m.Write(fmt.Sprintf("```%s\n%s\n```\n\n", language, code))
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
