// Package rankings is a generic leaderboard primitive: submit scored items,
// apply time decay, and read top-N / rank per time window.
//
// Domain-neutral (Item/Member/Score only) — consumers supply their own
// ScoreCalculator, DecayStrategy, TimeWindow, and Storage.
package rankings

import (
	"context"
	"fmt"
	"math"
	"time"
)

// ---------- 核心接口 ----------

// ScoreCalculator 计算单条记录的原始分数。
// 实现方必须在 Calculate 内部做浮点精度校验：最终 score 的绝对值不得超出 safeScoreLimit。
type ScoreCalculator interface {
	// Calculate returns the raw score for one item, guarding float precision.
	Calculate(item Item) (float64, error)
}

// DecayStrategy 对原始分数施加时间衰减。
// elapsed 是事件时间到当前时刻的时长。
type DecayStrategy interface {
	// Apply returns rawScore after elapsed-time decay.
	Apply(rawScore float64, elapsed time.Duration) float64
}

// TimeWindow 决定排行榜的时间隔离粒度。
// CurrentKey 返回当前窗口的唯一标识，用于拼接存储 key。
type TimeWindow interface {
	// CurrentKey returns the token identifying the current window.
	CurrentKey() string
}

// Storage 是排行榜的存储抽象。
// key 由调用方拼接（prefix:windowKey），Storage 实现只关心 key 本身。
type Storage interface {
	// Update upserts member with score at key.
	Update(ctx context.Context, key, member string, score float64) error
	// TopN returns the top n members at key, ranked desc.
	TopN(ctx context.Context, key string, n int) ([]RankItem, error)
	// Rank returns the 1-based rank of member at key, or -1 when absent.
	Rank(ctx context.Context, key, member string) (int, error)
}

// ---------- 数据结构 ----------

// Item 是一条排行榜数据。
// Fields 放原始业务字段（如 amount、views），Calculator 从这里取值。
type Item struct {
	Timestamp time.Time
	Fields    map[string]float64
	Member    string
}

// RankItem 是排行榜的一条返回结果。
type RankItem struct {
	Member string
	Score  float64
	Rank   int
}

// ---------- 排行榜实例 ----------

// Ranking 是一个排行榜实例。
// 通过 NewRanking 创建，组合 ScoreCalculator / DecayStrategy / TimeWindow / Storage。
type Ranking struct {
	name    string
	calc    ScoreCalculator
	decay   DecayStrategy
	window  TimeWindow
	storage Storage
	prefix  string
}

// NewRanking 创建排行榜实例。
// prefix 用于 key 前缀（如 "rank:gift"），windowKey 由 TimeWindow 生成。
func NewRanking(name, prefix string, calc ScoreCalculator, decay DecayStrategy, window TimeWindow, storage Storage) *Ranking {
	return &Ranking{
		name:    name,
		calc:    calc,
		decay:   decay,
		window:  window,
		storage: storage,
		prefix:  prefix,
	}
}

// Submit 提交一条数据：计算分数 → 施加衰减 → 写入存储。
func (r *Ranking) Submit(ctx context.Context, item Item) error {
	rawScore, err := r.calc.Calculate(item)
	if err != nil {
		return fmt.Errorf("ranking %s: calculate: %w", r.name, err)
	}
	elapsed := time.Since(item.Timestamp)
	finalScore := r.decay.Apply(rawScore, elapsed)
	key := r.prefix + ":" + r.window.CurrentKey()
	if err := r.storage.Update(ctx, key, item.Member, finalScore); err != nil {
		return fmt.Errorf("ranking %s: storage update: %w", r.name, err)
	}
	return nil
}

// TopN 返回当前窗口的前 n 名。
func (r *Ranking) TopN(ctx context.Context, n int) ([]RankItem, error) {
	key := r.prefix + ":" + r.window.CurrentKey()
	return r.storage.TopN(ctx, key, n)
}

// MyRank 返回 member 在当前窗口的排名（1-based）。不存在时返回 -1。
func (r *Ranking) MyRank(ctx context.Context, member string) (int, error) {
	key := r.prefix + ":" + r.window.CurrentKey()
	return r.storage.Rank(ctx, key, member)
}

// ---------- 精度安全常量 ----------

// safeScoreLimit 是 float64 能精确表示整数的上界 (2^53)。
// 超过此值，低位精度丢失，排序结果不可靠。
const safeScoreLimit = float64(1 << 53)

// validateScore 校验 score 是否在安全区间内。
func validateScore(score float64, calcName string) error {
	if math.Abs(score) > safeScoreLimit {
		return fmt.Errorf("%s: score %.0f exceeds float64 safe integer limit (2^53 = %.0f)", calcName, score, safeScoreLimit)
	}
	return nil
}
