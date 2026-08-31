package rankings

import (
	"math"
	"time"
)

// ---------- 无衰减 ----------

// NoDecay 不做任何衰减，适合累计榜。
type NoDecay struct{}

func (d *NoDecay) Apply(rawScore float64, _ time.Duration) float64 {
	return rawScore
}

// ---------- 指数衰减 ----------

// ExponentialDecay 指数衰减：score * e^(-lambda * hours)。
// Lambda 越大衰减越快。适合「近期内容优先」的场景。
type ExponentialDecay struct {
	Lambda float64 // 每小时衰减速率
}

func (d *ExponentialDecay) Apply(rawScore float64, elapsed time.Duration) float64 {
	hours := elapsed.Hours()
	return rawScore * math.Exp(-d.Lambda*hours)
}

// ---------- 线性衰减 ----------

// LinearDecay 线性衰减：score * max(0, 1 - dailyRate * days)。
// DailyRate=0.1 表示每天衰减 10%，10 天后归零。
type LinearDecay struct {
	DailyRate float64
}

func (d *LinearDecay) Apply(rawScore float64, elapsed time.Duration) float64 {
	days := elapsed.Hours() / 24
	factor := 1.0 - d.DailyRate*days
	if factor < 0 {
		factor = 0
	}
	return rawScore * factor
}
