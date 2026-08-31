package compact

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xbpk3t/docs-alfred/pkg/rankings"
)

func TestWikiCensusScoreCalc(t *testing.T) {
	calc := WikiCensusScoreCalc{}

	t.Run("research dominates files/size", func(t *testing.T) {
		research := calcScore(t, calc, (censusSpec{research: 5, files: 0, sizeKB: 1024}).asItem()) // 1 MiB
		flat := calcScore(t, calc, (censusSpec{research: 0, files: 100, sizeKB: 0}).asItem())      // many flat files
		assert.Greater(t, research, flat)
	})
}

func TestWikiCensusScoreCalc_NegativeRejected(t *testing.T) {
	calc := WikiCensusScoreCalc{}
	_, err := calc.Calculate(rankings.Item{
		Fields: map[string]float64{FieldResearch: -1},
	})
	require.Error(t, err)
}

// --- helpers ---

type censusSpec struct {
	research int
	files    int
	sizeKB   int
}

func (c censusSpec) asItem() rankings.Item {
	return rankings.Item{
		Fields: map[string]float64{
			FieldResearch: float64(c.research),
			FieldFiles:    float64(c.files),
			FieldSize:     float64(c.sizeKB) * 1024,
		},
	}
}

func calcScore(t *testing.T, calc WikiCensusScoreCalc, item rankings.Item) float64 {
	t.Helper()
	s, err := calc.Calculate(item)
	require.NoError(t, err)
	return s
}
