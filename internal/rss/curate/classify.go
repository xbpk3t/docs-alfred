package curate

import (
	"context"
	"fmt"
	"strings"

	"github.com/microsoft/agent-framework-go/agent"
)

// classInputItem is the compact view of a freq-pass candidate given to Step 3.
type classInputItem struct {
	Name   string   `json:"name"`
	Medium string   `json:"medium"`
	Feed   string   `json:"feed"`
	Titles []string `json:"titles,omitempty"`
}

// typeAssignment is one Step-3 output row (feed -> one rss2nl type).
type typeAssignment struct {
	Feed string `json:"feed"`
	Type string `json:"type"`
}

type classifyBatch struct {
	Assignments []typeAssignment `json:"assignments"`
}

// UnmarshalJSON tolerates glm-style bare arrays: accepts both
// {"assignments":[...]} and plain [...].
func (b *classifyBatch) UnmarshalJSON(data []byte) error {
	type plain classifyBatch
	var p plain
	if err := unmarshalObjectOrList(data, &p, &b.Assignments); err != nil {
		return err
	}
	if b.Assignments == nil {
		b.Assignments = p.Assignments
	}

	return nil
}

const step3Instructions = `你是 RSS 源分类器。给定一批已通过频率筛选的源、以及一份可用的订阅 type 列表，为每条源分配**恰一个**主 type（必须是给定列表里原样出现的字符串）。按内容与媒介判断，不臆造新 type，不写 remark。

只输出 JSON 数组，不要散文。`

// step3Classify splits the pool into ClassifyChunkSize-sized chunks and runs a
// concurrent classify call per chunk (bounded by CurateConcurrency), then
// merges the per-chunk assignments in pool order. Chunking keeps each prompt
// short (low tokens, fast) and the fan-out hides per-call latency; if
// ClassifyChunkSize >= the pool it degrades to one call exactly as before.
func step3Classify(
	ctx context.Context,
	cfg *Config,
	types []string,
	pool []classInputItem,
) ([]typeAssignment, error) {
	a := newAgent(cfg, "curate-classify", step3Instructions)

	cs := chunkItems(pool, cfg.ClassifyChunkSize)
	outs := make([][]typeAssignment, len(cs))
	err := runBounded(ctx, len(cs), cfg.CurateConcurrency, func(ctx context.Context, i int) error {
		asg, e := classifyChunk(ctx, cfg, a, types, cs[i])
		outs[i] = asg

		return e
	})
	if err != nil {
		return nil, fmt.Errorf("step3 classify: %w", err)
	}

	var all []typeAssignment
	for _, o := range outs {
		all = append(all, o...)
	}

	return all, nil
}

// classifyChunk builds the prompt for one Step-3 chunk and runs the classify
// agent, returning its type assignments.
func classifyChunk(ctx context.Context, cfg *Config, a *agent.Agent, types []string, pool []classInputItem) ([]typeAssignment, error) {
	var b strings.Builder
	b.WriteString("## 可用的 type 列表\n")
	for _, t := range types {
		fmt.Fprintf(&b, "- %s\n", t)
	}
	b.WriteString("\n## freq 池（name | medium | feed | 最近标题样本）\n")
	for _, it := range pool {
		fmt.Fprintf(&b, "- %s | %s | %s | %s\n", it.Name, it.Medium, it.Feed, strings.Join(it.Titles, " ; "))
	}
	b.WriteString("\n为以上每条输出 {feed, type}。")

	batch, err := run[classifyBatch](ctx, cfg, a, b.String())
	if err != nil {
		return nil, fmt.Errorf("step3 classify: %w", err)
	}

	return batch.Assignments, nil
}
