package render

import (
	"fmt"
	"strings"

	"github.com/olekukonko/tablewriter"
	"github.com/xbpk3t/docs-alfred/pkg"
)

const (
	HeadingLevel1 = 1
	HeadingLevel2 = 2
	HeadingLevel3 = 3
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

// RenderListItems 渲染多个列表项
func (r *MarkdownRenderer) RenderListItems(items []string) {
	for _, item := range items {
		r.RenderListItem(item)
	}
}

// RenderHorizontalRule 渲染水平分隔线
func (r *MarkdownRenderer) RenderHorizontalRule() {
	r.Write("\n---\n\n")
}

// RenderParagraph 渲染段落
func (r *MarkdownRenderer) RenderParagraph(text string) {
	r.Write(text + "\n\n")
}

// RenderBadge 渲染徽章
func (r *MarkdownRenderer) RenderBadge(text, color string) string {
	return fmt.Sprintf("![%s](https://img.shields.io/badge/-%s-%s)", text, text, color)
}

// RenderSection 渲染带标题的区块
func (r *MarkdownRenderer) RenderSection(title string, level int, content string) {
	r.RenderHeader(level, title)
	r.Write(content)
}

// RenderDefinitionList 渲染定义列表
func (r *MarkdownRenderer) RenderDefinitionList(terms map[string]string) {
	for term, definition := range terms {
		r.Write(fmt.Sprintf("%s\n: %s\n", term, definition))
	}
}

// AdmonitionType 提示框类型
type AdmonitionType string

const (
	AdmonitionTip     AdmonitionType = "tip"
	AdmonitionInfo    AdmonitionType = "info"
	AdmonitionWarning AdmonitionType = "warning"
	AdmonitionDanger  AdmonitionType = "danger"
)

// RenderAdmonition 渲染提示框
func (r *MarkdownRenderer) RenderAdmonition(typ AdmonitionType, title, content string) {
	r.Write(fmt.Sprintf("\n:::%s[%s]\n\n%s\n\n:::\n\n", typ, title, content))
}

// RenderEmphasis 渲染强调文本
func (r *MarkdownRenderer) RenderEmphasis(text string, style string) string {
	return fmt.Sprintf("%s%s%s", style, text, style)
}

// RenderList 渲染列表
func (r *MarkdownRenderer) RenderList(items []string, ordered bool) {
	for i, item := range items {
		if ordered {
			r.Write(fmt.Sprintf("%d. %s\n", i+1, item))
		} else {
			r.RenderListItem(item)
		}
	}
}

// RenderTable 渲染表格
func (r *MarkdownRenderer) RenderTable(headers []string, rows [][]string) {
	// 渲染表头
	r.Write("|")
	for _, header := range headers {
		r.Write(fmt.Sprintf(" %s |", header))
	}
	r.Write("\n|")

	// 渲染分隔线
	for range headers {
		r.Write(" --- |")
	}
	r.Write("\n")

	// 渲染数据行
	for _, row := range rows {
		r.Write("|")
		for _, cell := range row {
			r.Write(fmt.Sprintf(" %s |", cell))
		}
		r.Write("\n")
	}
	r.Write("\n")
}

// RenderQuote 渲染引用
func (r *MarkdownRenderer) RenderQuote(text string) {
	r.Write(fmt.Sprintf("> %s\n", text))
}

// RenderTask 渲染任务列表
func (r *MarkdownRenderer) RenderTask(text string, checked bool) {
	checkMark := " "
	if checked {
		checkMark = "x"
	}
	r.Write(fmt.Sprintf("- [%s] %s\n", checkMark, text))
}

// RenderKeyValue 渲染键值对
func (r *MarkdownRenderer) RenderKeyValue(key, value string) {
	r.Write(fmt.Sprintf("**%s**: %s\n", key, value))
}

// RenderDetails 渲染详情块
func (r *MarkdownRenderer) RenderDetails(summary, details string) {
	r.RenderFold(summary, details)
}

// RenderWarning 渲染警告信息
func (r *MarkdownRenderer) RenderWarning(text string) {
	r.RenderAdmonition(AdmonitionWarning, "Warning", text)
}

// RenderInfo 渲染信息提示
func (r *MarkdownRenderer) RenderInfo(text string) {
	r.RenderAdmonition(AdmonitionInfo, "Info", text)
}

// RenderTip 渲染提示信息
func (r *MarkdownRenderer) RenderTip(text string) {
	r.RenderAdmonition(AdmonitionTip, "Tip", text)
}

// RenderDanger 渲染危险信息
func (r *MarkdownRenderer) RenderDanger(text string) {
	r.RenderAdmonition(AdmonitionDanger, "Danger", text)
}

// RenderCodeBlockComponent 渲染带边框的容器
func (r *MarkdownRenderer) RenderCodeBlockComponent(content string, style string) {
	r.Write(fmt.Sprintf("\n<CodeBlock language=\"%s\">%s</CodeBlock>\n\n", style, content))
}

// RenderFootnote 渲染脚注
func (r *MarkdownRenderer) RenderFootnote(text, note string) {
	r.Write(fmt.Sprintf("%s[^%s]\n\n[^%s]: %s\n", text, note, note, note))
}

// RenderTabs 渲染标签页
func (r *MarkdownRenderer) RenderTabs(tabs map[string]string) {
	r.Write("=== tabs\n")
	for title, content := range tabs {
		r.Write(fmt.Sprintf("== %s\n%s\n", title, content))
	}
	r.Write("===\n\n")
}

// RenderExpandable 渲染可展开块
func (r *MarkdownRenderer) RenderExpandable(title, content string, expanded bool) {
	symbol := "?"
	if expanded {
		symbol = "+"
	}
	r.Write(fmt.Sprintf("\n.%s %s\n%s\n\n", symbol, title, content))
}

// RenderKeyboard 渲染键盘按键
func (r *MarkdownRenderer) RenderKeyboard(key string) string {
	return fmt.Sprintf("<kbd>%s</kbd>", key)
}

// RenderMermaid 渲染 Mermaid 图表
func (r *MarkdownRenderer) RenderMermaid(diagram string) {
	r.Write("```mermaid\n" + diagram + "\n```\n\n")
}

// RenderMath 渲染数学公式
func (r *MarkdownRenderer) RenderMath(formula string, inline bool) {
	if inline {
		r.Write(fmt.Sprintf("$%s$", formula))
	} else {
		r.Write(fmt.Sprintf("\n$$\n%s\n$$\n\n", formula))
	}
}

// RenderTimeline 渲染时间线
func (r *MarkdownRenderer) RenderTimeline(events []struct{ Time, Event string }) {
	for _, event := range events {
		r.Write(fmt.Sprintf("- %s :: %s\n", event.Time, event.Event))
	}
	r.Write("\n")
}

// RenderCallout 渲染醒目提示
func (r *MarkdownRenderer) RenderCallout(text string, style string) {
	r.Write(fmt.Sprintf("\n> [!%s]\n> %s\n\n", style, text))
}

// RenderMetadata 渲染元数据
func (r *MarkdownRenderer) RenderMetadata(metadata map[string]string) {
	r.Write("---\n")
	for key, value := range metadata {
		r.Write(fmt.Sprintf("%s: %s\n", key, value))
	}
	r.Write("---\n\n")
}

// RenderCheckboxList 渲染复选框列表
func (r *MarkdownRenderer) RenderCheckboxList(items []struct {
	Text    string
	Checked bool
},
) {
	for _, item := range items {
		mark := " "
		if item.Checked {
			mark = "x"
		}
		r.Write(fmt.Sprintf("- [%s] %s\n", mark, item.Text))
	}
	r.Write("\n")
}

// ReplaceUnorderedListWithTask 将无序列表替换为任务列表
func (r *MarkdownRenderer) ReplaceUnorderedListWithTask(str string) string {
	// return "- [ ] " + strings.Replace(str, "- ", "", -1) + "\n\n"
	return "- [ ] " + strings.Replace(str, "- ", "", -1) + "\n"
}

// RenderImport 渲染导入语句
func (r *MarkdownRenderer) RenderImport(importName, relativePath string) {
	r.RenderParagraph(fmt.Sprintf("import %s from '!!raw-loader!%s';", importName, relativePath))
}

// RenderDocusaurusRawLoader 渲染Docusaurus raw loader导入和代码块
func (r *MarkdownRenderer) RenderDocusaurusRawLoader(name, relativePath string) {
	// 渲染导入语句
	r.RenderImport(name, relativePath)
	// 渲染代码块
	r.RenderCodeBlockComponent("{"+name+"}", "yaml")
}
