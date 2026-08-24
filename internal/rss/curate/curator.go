package curate

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/microsoft/agent-framework-go/agent"
)

// curateItem is one source handed to a per-type curator.
type curateItem struct {
	Name   string   `json:"name"`
	Medium string   `json:"medium"`
	Feed   string   `json:"feed"`
	Note   string   `json:"note,omitempty"`
	Titles []string `json:"titles,omitempty"`
}

// curatorOut is the strict JSON contract of one per-type curator run.
type curatorOut struct {
	Verdicts []verdict `json:"verdicts"`
}

// UnmarshalJSON tolerates glm-style bare arrays: {"verdicts":[...]} or [...].
func (o *curatorOut) UnmarshalJSON(data []byte) error {
	type plain curatorOut
	var p plain
	if err := unmarshalObjectOrList(data, &p, &o.Verdicts); err != nil {
		return err
	}
	if o.Verdicts == nil {
		o.Verdicts = p.Verdicts
	}

	return nil
}

// curatedGroup collects one type's verdicts plus failure isolation.
type curatedGroup struct {
	Type     string    `json:"type"`
	Err      string    `json:"err,omitempty"`
	Verdicts []verdict `json:"verdicts"`
	Failed   bool      `json:"failed,omitempty"`
}

const step4Instructions = `你是 RSS 源质量审核的单一分类 curator。你会收到**某一 type** 的子集源，逐条裁决。看这里编码的硬否决 rubric：
- eol: 已停更或项目关闭
- marketing: 纯营销 / SEO 内容农场
- ai_slop: 泛化、无品味的 LLM 填充内容
- too_noisy: 噪声大、阅读价值被稀释
- low_signal: 低信号 / 内容稀薄
- 以上都过就是 keep

对每条输出 verdict {name, decision(keep|drop), reason_code(上述码之一), reason_text}。name 必须与输入完全一致；只能有一个 decision。保持品味一致，宁缺毋滥。只输出 JSON 数组，不要散文。`

// step4Curate fans out one agent per (type, chunk). Each type's subset is split
// into cfg.ClassifyChunkSize-sized chunks so a single curator call never carries
// a whole large type's worth of context: oversized prompts take long enough to
// generate that the origin trips a Cloudflare 524 mid-stream, cascading the
// whole type into "type failed". With small concurrent chunks a block succeeds;
// partial success is kept (only a type with zero successful verdicts is flagged
// failed). The fan-out is bounded by CurateConcurrency; goroutine i owns slot i
// so there is no map race.
func step4Curate(ctx context.Context, cfg *Config, groups map[string][]curateItem) map[string]curatedGroup {
	a := newAgent(cfg, "curate-curator", step4Instructions)

	keys := sortedTypeKeys(groups)
	type job struct {
		typ   string
		items []curateItem
	}
	jobs := make([]job, 0, len(keys))
	for _, typ := range keys {
		for _, c := range chunkItems(groups[typ], cfg.ClassifyChunkSize) {
			jobs = append(jobs, job{typ: typ, items: c})
		}
	}

	res := make([]struct {
		err error
		out curatorOut
	}, len(jobs))
	_ = runBounded(ctx, len(jobs), cfg.CurateConcurrency, func(ctx context.Context, i int) error {
		o, e := curateOne(ctx, cfg, a, jobs[i].typ, jobs[i].items)
		res[i].out, res[i].err = o, e

		return nil // partial failure never aborts the other jobs
	})

	merged := make(map[string]curatedGroup, len(keys))
	for i := range res {
		g := merged[jobs[i].typ]
		g.Type = jobs[i].typ
		if res[i].err == nil {
			g.Verdicts = append(g.Verdicts, res[i].out.Verdicts...)
		} else {
			g.Failed, g.Err = true, res[i].err.Error()
		}
		merged[jobs[i].typ] = g
	}
	// A type with at least one successful verdict still counts as judged; only a
	// wholly-failed type is flagged Failed (its feeds land "unrated" in drops).
	for typ, g := range merged {
		if len(g.Verdicts) > 0 {
			g.Failed, g.Err = false, ""
		}
		merged[typ] = g
	}

	return merged
}

// sortedTypeKeys returns the group keys in stable order so per-type fan-out and
// assembly are deterministic across runs.
func sortedTypeKeys(groups map[string][]curateItem) []string {
	keys := make([]string, 0, len(groups))
	for k := range groups {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	return keys
}

func curateOne(ctx context.Context, cfg *Config, a *agent.Agent, typ string, items []curateItem) (curatorOut, error) {
	var b strings.Builder
	fmt.Fprintf(&b, "## 当前分类 type = %s\n", typ)
	b.WriteString("## 子集源（name | medium | feed | note | 标题样本）\n")
	for _, it := range items {
		fmt.Fprintf(&b, "- %s | %s | %s | %s | %s\n", it.Name, it.Medium, it.Feed, it.Note, strings.Join(it.Titles, " ; "))
	}
	b.WriteString("\n裁决以上每条。")

	return run[curatorOut](ctx, cfg, a, b.String())
}
