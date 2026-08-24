package curate

import (
	"context"
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/xbpk3t/docs-alfred/pkg/urlutil"
)

// attrRe matches an OPML <outline text="…"> / title="…" label on a line.
var attrRe = regexp.MustCompile(`(?:text|title)="([^"]*)"`)

const noRSS = "no_rss"

// resolveBatch is the strict JSON contract Step 1 must return.
type resolveBatch struct {
	Candidates []Candidate `json:"candidates"`
}

// UnmarshalJSON tolerates glm-style bare arrays: {"candidates":[...]} or [...].
func (b *resolveBatch) UnmarshalJSON(data []byte) error {
	type plain resolveBatch
	var p plain
	if err := unmarshalObjectOrList(data, &p, &b.Candidates); err != nil {
		return err
	}
	if b.Candidates == nil {
		b.Candidates = p.Candidates
	}

	return nil
}

const step1Instructions = `你是 RSS 源抽取器。给定一份精选种子清单（可能是榜单、表格、网页刮表或纯文本），你逐条抽取其中的可订阅源并输出一个 candidate JSON 数组。

对每条源：
- name: 源的名字
- medium: 媒介（blog / podcast / newsletter / youtube / wechat / video 等）
- feed: 已确认的 RSS/Atom 订阅链接；若无法确定真实订阅链接，留空字符串
- url: 源主页（可选）
- note: 一句备注
- source_category: 源在清单里所属的栏目/类别（若有）
- gino_pri: 仅在清单明确标注外部评级时填入（low/med/high），否则省略

规则：
1. 绝不编造 feed 链接。feed 只能来自：清单中给出的订阅地址、已知聚合源会给出、或可从小宇宙 id 拼出 RSSHub 模板。
2. 若一条源找不到可信的 feed 链接，请**不要**填占位或编造，把该条候选的 feed 留空，交给下游判定 no_rss。
3. 聚合源（如"bz视频"等聚合类）直接引用给定/已知的聚合 feed。
4. 只输出 JSON 数组，不要任何解释性散文。`

// resolvePrompt bundles everything the resolver sees.
type resolvePrompt struct {
	Seed       string
	Aliases    string              // name -> existing feed url lines
	Existing   map[string][]string // existing rss2nl.yml type -> feeds, for reuse/aggregation
	XZTemplate string
}

func buildResolvePrompt(p resolvePrompt) string {
	var b strings.Builder
	b.WriteString("请从下面的精选清单中抽取所有可订阅源：\n\n")
	b.WriteString("## 清单\n")
	b.WriteString(p.Seed)
	b.WriteString("\n\n")
	if p.Aliases != "" {
		b.WriteString("## 可直接复用的聚合/已知源（name -> 既定订阅链接，命中则直接用，不要再自造）\n")
		b.WriteString(p.Aliases)
		b.WriteString("\n")
	}
	if p.Existing != nil {
		b.WriteString("## 现有 rss2nl.yml 已订阅 feeds（识别聚合时可复用；已在其中则标注 dedupe）\n")
		for t, feeds := range p.Existing {
			fmt.Fprintf(&b, "- <%s>: %s\n", t, strings.Join(feeds, ", "))
		}
	}
	if p.XZTemplate != "" {
		b.WriteString("## 小宇宙订阅模板\n")
		b.WriteString(p.XZTemplate)
		b.WriteString("\n")
	}
	b.WriteString("\n开始抽取。")

	return b.String()
}

// step1Resolve runs the MAF resolver and then the deterministic URL validator
// (finalizeCandidate) before any step can ingest a synthetically-invented URL.
func step1Resolve(ctx context.Context, cfg *Config, p resolvePrompt) ([]Candidate, error) {
	a := newAgent(cfg, "curate-resolve", step1Instructions)
	batch, err := run[resolveBatch](ctx, cfg, a, buildResolvePrompt(p))
	if err != nil {
		return nil, fmt.Errorf("step1 resolve: %w", err)
	}

	// Run's single finalize loop (run.go) is the one gate for every candidate,
	// deterministic and AI alike; no finalize here.
	return batch.Candidates, nil
}

// finalizeCandidate is the deterministic gate between AI and the mechanical
// funnel: alias reuse wins, everything else must be a real http(s) URL,
// otherwise the candidate is refused as no_rss. It never reads GinoPri.
func finalizeCandidate(c *Candidate, known map[string]string) {
	trim := strings.ToLower(strings.TrimSpace(c.Name))
	if trim == "" {
		c.Fatal, c.Feed = noRSS, ""
		c.Note = "missing name; refused before fetch"
		return
	}
	// 1) known aggregate alias -> reuse the feed directly (拼接即可).
	if feed, ok := known[trim]; ok && feed != "" {
		c.Feed = feed
		c.GinoPri = "" // 别名复用不进评级闸

		return
	}
	// 2) otherwise the feed must be a structurally valid http(s) URL.
	if !validFeedURL(c.Feed) {
		c.Fatal, c.Note, c.Feed = noRSS, "could not resolve a valid feed URL; refused", ""
		return
	}
}

func validFeedURL(raw string) bool {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return false
	}

	return (u.Scheme == "http" || u.Scheme == "https") && u.Host != "" && u.Host != "localhost"
}

// inferMedium applies a cheap, deterministic medium label so media feeds
// (youtube video channels) set IsMedia downstream, even for the deterministic
// extraction path.
func inferMedium(raw string) string {
	lower := strings.ToLower(raw)
	switch {
	case strings.Contains(lower, "youtube.com/feeds/videos.xml"), strings.Contains(lower, "youtube.com"):
		return "youtube"
	default:
		return ""
	}
}

// extractDeterministic reuses urlutil (the same extractor wiki's uses) to
// pull URL-bearing feeds straight out of a seed line. Each line with an
// http(s) URL becomes a candidate {Feed, Name(label)}; lines with no URL are
// left for the MAF fallback. This is the deterministic "extract URLs in Go"
// half — it never invents a URL.
func extractDeterministic(seed string) []Candidate {
	var out []Candidate
	for _, line := range strings.Split(seed, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		refs := urlutil.ExtractURLRefs(line, urlutil.ExtractOptions{
			BareURLs:    true,
			HTTPOnly:    true,
			Normalize:   true,
			Deduplicate: false,
		})
		if len(refs) == 0 {
			continue
		}
		c := Candidate{Feed: refs[0].URL}
		if m := attrRe.FindStringSubmatch(line); len(m) > 1 {
			c.Name = strings.TrimSpace(m[1])
		}
		if c.Name == "" {
			if u, err := url.Parse(c.Feed); err == nil {
				c.Name = u.Host
			}
		}
		c.Medium = inferMedium(c.Feed)
		out = append(out, c)
	}

	return out
}
