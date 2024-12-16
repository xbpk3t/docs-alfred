package utils

import (
	"fmt"
	"strings"
)

// MarkdownRenderer Markdown渲染器
type MarkdownRenderer struct {
	builder strings.Builder
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
	m.Write(fmt.Sprintf("<details><summary>%s</summary>\n\n%s\n</details>\n",
		summary, details))
}

// RenderTable 渲染表格
func (m *MarkdownRenderer) RenderTable(headers []string, rows [][]string) {
	// 渲染表头
	m.Write("|" + strings.Join(headers, "|") + "|\n")
	m.Write("|" + strings.Repeat("---|", len(headers)) + "\n")

	// 渲染数据行
	for _, row := range rows {
		m.Write("|" + strings.Join(row, "|") + "|\n")
	}
}

// RenderCodeBlock 渲染代码块
func (m *MarkdownRenderer) RenderCodeBlock(language, code string) {
	m.Write(fmt.Sprintf("```%s\n%s\n```\n", language, code))
}
