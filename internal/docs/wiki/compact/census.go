package compact

import (
	"fmt"

	"github.com/xbpk3t/docs-alfred/internal/docs/wiki/audit"
	"github.com/xbpk3t/docs-alfred/pkg/rankings"
)

// WikiCensusScoreCalc ranks a topic by its lifecycle mass (清零 score) — a
// deterministic, AI-free ordinal over the full-state census. Deep-dive weight
// dominates (the most blog-worthy articles), files and size are supporting
// volume terms. Consumes Item.Fields from audit.TopicMetrics.
type WikiCensusScoreCalc struct{}

// Fields keys expected in Item.Fields (set by the compact wiring).
const (
	FieldFiles    = "files"
	FieldSize     = "size"
	FieldResearch = "research"
)

func (WikiCensusScoreCalc) Calculate(item rankings.Item) (float64, error) {
	files := int(item.Fields[FieldFiles])
	size := int64(item.Fields[FieldSize])
	research := int(item.Fields[FieldResearch])
	if files < 0 || size < 0 || research < 0 {
		return 0, fmt.Errorf("census score: negative metric")
	}
	// Small integer score (< ~60); 2^53 float precision guard in pkg/rankings is
	// irrelevant here, only sign is guarded above.
	return float64(researchBucket(research)*3 + research + min(files, 10) + sizeBucket(size)), nil
}

// researchBucket tiers the deep-dive count: the primary blog-worthiness axis.
func researchBucket(n int) int {
	switch {
	case n >= 5:
		return 5
	case n >= 3:
		return 4
	case n >= 2:
		return 3
	case n >= 1:
		return 2
	default:
		return 0
	}
}

// sizeBucket tiers topic bytes so volume contributes but caps (a giant paste
// without structure ranks below sustained research accumulation).
func sizeBucket(bytes int64) int {
	switch {
	case bytes >= 1<<20: // 1 MiB
		return 5
	case bytes >= 512<<10:
		return 4
	case bytes >= 256<<10:
		return 3
	case bytes >= 64<<10:
		return 2
	case bytes >= 8<<10:
		return 1
	default:
		return 0
	}
}

// censusItem maps one census TopicMetric into a ranking Item (Member = topic
// path, Fields = raw metrics). The ranking scores/sorts; the caller keeps the
// original metric for report rendering — don't cram report data in.
func censusItem(m audit.TopicMetric) rankings.Item {
	return rankings.Item{
		Member: m.Path,
		Fields: map[string]float64{
			FieldFiles:    float64(m.Files),
			FieldSize:     float64(m.Size),
			FieldResearch: float64(m.Research),
		},
	}
}
