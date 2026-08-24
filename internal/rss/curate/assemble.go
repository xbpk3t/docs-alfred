package curate

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/goccy/go-yaml"

	rss "github.com/xbpk3t/docs-alfred/internal/rss/feed"
)

// keepEntry is one merged keep row resolved to an rss2nl FE field.
type keepEntry struct {
	Type    string
	Feed    string
	URL     string
	Des     string
	IsMedia bool
}

// dropEntry is one rejected feed grouped by reason for the drops report.
type dropEntry struct {
	Name   string `json:"name"`
	Feed   string `json:"feed"`
	Status string `json:"status"`
	Reason string `json:"reason"`
}

// Summary is the machine-readable outcome of a curate run.
type Summary struct {
	GeneratedAt string   `json:"generatedAt"`
	Model       string   `json:"model"`
	SeedFiles   []string `json:"seedFiles"`
	Resolved    int      `json:"resolved"`
	FreqPass    int      `json:"freqPass"`
	FreqFail    int      `json:"freqFail"`
	Kept        int      `json:"kept"`
	Dropped     int      `json:"dropped"`
	Posts90DMin int      `json:"posts90dMin"`
	LastMaxDays int      `json:"lastMaxDays"`
}

// groupKeepsByType groups keep entries into the rss2nl.yml shape.
func groupKeepsByType(keeps []keepEntry) []rss.FeedsDetail {
	byType := make(map[string][]rss.Feeds)
	for _, k := range keeps {
		byType[k.Type] = append(byType[k.Type], rss.Feeds{
			Feed:    k.Feed,
			URL:     k.URL,
			Des:     strings.TrimSpace(k.Des),
			IsMedia: k.IsMedia,
		})
	}
	types := make([]string, 0, len(byType))
	for t := range byType {
		types = append(types, t)
	}
	sort.Strings(types)

	detail := make([]rss.FeedsDetail, 0, len(types))
	for _, t := range types {
		detail = append(detail, rss.FeedsDetail{Type: t, Feeds: byType[t]})
	}

	return detail
}

// renderKeepYAML serializes the keep preview as rss2nl.yml `rss:` section,
// prefixed with a bestblogs import marker for future git-blame. The shape is
// 1:1 with rss.FeedsDetail / rss.Feeds.
func renderKeepYAML(detail []rss.FeedsDetail, now time.Time) ([]byte, error) {
	doc := map[string][]rss.FeedsDetail{"rss": detail}
	body, err := yaml.Marshal(doc)
	if err != nil {
		return nil, fmt.Errorf("marshal keep yaml: %w", err)
	}
	marker := fmt.Sprintf("# --- bestblogs import %s ---\n", now.Format("2006-01-02"))

	return append([]byte(marker), body...), nil
}

// renderKeepsMarkdown renders the readable keep list.
func renderKeepsMarkdown(keeps []keepEntry, now time.Time) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# bestblogs curate — keep preview (%s)\n\n", now.Format("2006-01-02"))
	for _, k := range keeps {
		link := k.URL
		if link == "" {
			link = k.Feed
		}
		fmt.Fprintf(&b, "- **%s** [%s](%s)", k.Type, k.Feed, link)
		if k.Des != "" {
			fmt.Fprintf(&b, " — %s", k.Des)
		}
		b.WriteString("\n")
	}
	b.WriteString("\n")

	return b.String()
}

// renderDropsMarkdown renders the per-reason drops report.
func renderDropsMarkdown(drops []dropEntry, failed map[string]string, now time.Time) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# bestblogs 淘汰明细 %s\n\n", now.Format("2006-01-02"))
	b.WriteString("## 统一归类\n")
	byReason := map[string][]dropEntry{}
	for _, d := range drops {
		byReason[d.Status] = append(byReason[d.Status], d)
	}
	keys := make([]string, 0, len(byReason))
	for k := range byReason {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Fprintf(&b, "### %s (%d)\n", k, len(byReason[k]))
		for _, d := range byReason[k] {
			fmt.Fprintf(&b, "- %s — %s\n", d.Name, d.Reason)
		}
		b.WriteString("\n")
	}
	if len(failed) > 0 {
		b.WriteString("## 处理失败（未裁决）\n")
		for name, msg := range failed {
			fmt.Fprintf(&b, "- %s: %s\n", name, msg)
		}
		b.WriteString("\n")
	}

	return b.String()
}
