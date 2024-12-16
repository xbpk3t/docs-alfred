package work

import (
	"fmt"
	"github.com/xbpk3t/docs-alfred/utils"
	"strings"
)

type WorksRenderer struct {
	utils.MarkdownRenderer
	Config Docs
}

func (w *WorksRenderer) Render(data []byte) (string, error) {
	config := NewConfigQs(data)
	seenTags := make(map[string]bool)

	for _, doc := range config {
		if !seenTags[doc.Tag] {
			w.RenderHeader(2, doc.Tag)
			seenTags[doc.Tag] = true
		}

		if doc.Tag != doc.Type {
			w.RenderHeader(3, doc.Type)
		}

		if doc.Qs != nil {
			w.Write(addMarkdownQsFormatWorks(doc.Qs))
		}
	}
	return w.String(), nil
}

func addMarkdownQsFormatWorks(qs Qs) string {
	var builder strings.Builder

	for _, q := range qs {
		summary := formatSummaryWithWs(q)
		details := formatDetailsWithWs(q)
		if details == "" {
			builder.WriteString(fmt.Sprintf("- %s\n", summary))
		} else {
			builder.WriteString(utils.RenderMarkdownFold(summary, details))
		}
	}

	return builder.String()
}

// formatSummary 格式化摘要
func formatSummaryWithWs(q QsN) string {
	if q.U != "" {
		return fmt.Sprintf("[%s](%s)", q.Q, q.U)
	}
	return q.Q
}

// formatDetails 格式化详情
func formatDetailsWithWs(q QsN) string {
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
