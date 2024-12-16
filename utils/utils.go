package utils

import (
	"strings"

	"github.com/olekukonko/tablewriter"
)

// RenderMarkdownTable 封装了创建和渲染Markdown表格的逻辑
func RenderMarkdownTable(header []string, res *strings.Builder, data [][]string) {
	table := tablewriter.NewWriter(res)
	table.SetAutoWrapText(false)
	table.SetHeader(header)
	table.SetBorders(tablewriter.Border{Left: true, Top: false, Right: true, Bottom: false})
	table.SetCenterSeparator("|")
	table.AppendBulk(data) // 添加大量数据
	table.Render()
}

func JoinSlashParts(s string) string {
	index := strings.Index(s, "/")
	if index != -1 {
		// 拼接 `/` 前后的字符串，并保留 `/` 字符
		return s[:index] + s[index+1:]
	}
	return s
}
