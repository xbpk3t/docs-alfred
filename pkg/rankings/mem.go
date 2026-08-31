package rankings

import (
	"context"
	"sort"
	"sync"
)

// MemStorage 是 Storage 的内存实现，进程内生产可用（无外部依赖）。
// 每个 key 对应一个独立的排行榜。线程安全。
type MemStorage struct {
	data map[string]map[string]float64
	mu   sync.RWMutex
}

func NewMemStorage() *MemStorage {
	return &MemStorage{
		data: make(map[string]map[string]float64),
	}
}

func (s *MemStorage) Update(_ context.Context, key, member string, score float64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.data[key]; !ok {
		s.data[key] = make(map[string]float64)
	}
	s.data[key][member] = score
	return nil
}

func (s *MemStorage) TopN(_ context.Context, key string, n int) ([]RankItem, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	members, ok := s.data[key]
	if !ok || len(members) == 0 {
		return nil, nil
	}

	entries := make([]struct {
		member string
		score  float64
	}, 0, len(members))
	for m, sc := range members {
		entries = append(entries, struct {
			member string
			score  float64
		}{m, sc})
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].score > entries[j].score // 降序
	})

	if n > len(entries) {
		n = len(entries)
	}
	result := make([]RankItem, n)
	for i := 0; i < n; i++ {
		result[i] = RankItem{
			Member: entries[i].member,
			Score:  entries[i].score,
			Rank:   i + 1,
		}
	}
	return result, nil
}

func (s *MemStorage) Rank(_ context.Context, key, member string) (int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	members, ok := s.data[key]
	if !ok {
		return -1, nil
	}
	score, exists := members[member]
	if !exists {
		return -1, nil
	}

	// 计算排名：有多少人分数比我高
	rank := 1
	for _, sc := range members {
		if sc > score {
			rank++
		}
	}
	return rank, nil
}

// GetScore 直接获取某个 member 的分数（消费方渲染用）。
func (s *MemStorage) GetScore(key, member string) (float64, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if m, ok := s.data[key]; ok {
		sc, exists := m[member]
		return sc, exists
	}
	return 0, false
}
