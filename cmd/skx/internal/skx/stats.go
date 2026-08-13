package skx

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"time"
)

// StatsEntry is one routing statistic.
type StatsEntry struct {
	Name   string `json:"name"`
	LastTs string `json:"last_ts"`
	Target string `json:"target"`
	Count  int    `json:"count"`
}

// StatsRanking is one row of the ranked summary.
type StatsRanking struct {
	Name    string  `json:"name"`
	LastTs  string  `json:"last_ts"`
	Sug     string  `json:"sug"`
	Rank    int     `json:"rank"`
	Count   int     `json:"count"`
	Percent float64 `json:"pct"`
}

// LoadStats reads the routing stats file (array shape) and returns the
// entries. A missing file yields an empty list, not an error.
func LoadStats(path string) ([]StatsEntry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read stats %s: %w", path, err)
	}
	var entries []StatsEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil, fmt.Errorf("parse stats %s: %w", path, err)
	}
	return entries, nil
}

// RankStats turns the raw entries into a ranked list with a suggestion per
// row, mirroring the deterministic rules that stats-zzz.yml used to encode.
func RankStats(entries []StatsEntry, total int) []StatsRanking {
	if len(entries) == 0 {
		return nil
	}
	if total <= 0 {
		for _, e := range entries {
			total += e.Count
		}
	}
	if total <= 0 {
		total = 1
	}

	sorted := append([]StatsEntry(nil), entries...)
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].Count != sorted[j].Count {
			return sorted[i].Count > sorted[j].Count
		}
		return sorted[i].Name < sorted[j].Name
	})

	out := make([]StatsRanking, 0, len(sorted))
	for i, e := range sorted {
		pct := float64(e.Count) / float64(total) * 100
		out = append(out, StatsRanking{
			Rank:    i + 1,
			Name:    e.Name,
			Count:   e.Count,
			Percent: pct,
			LastTs:  e.LastTs,
			Sug:     suggest(e.Count, pct),
		})
	}
	return out
}

// suggest applies the demotion rules from stats-zzz.
func suggest(count int, pct float64) string {
	switch {
	case count > 0 && pct > 10:
		return "keep"
	case count > 0 && count < 3 && pct < 1:
		return "demote"
	case count == 0:
		return "drop"
	default:
		return ""
	}
}

// SaveStats writes the entries to the stats file (array shape, 0600).
func SaveStats(path string, entries []StatsEntry) error {
	data, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("write stats %s: %w", path, err)
	}
	return nil
}

// RecordHit increments the counter for name (array shape, find-or-append)
// and writes the file back. Missing file starts an empty array.
func RecordHit(path, name, target string) error {
	entries, err := LoadStats(path)
	if err != nil {
		return err
	}
	ts := time.Now().Format(time.RFC3339)

	idx := -1
	for i := range entries {
		if entries[i].Name == name {
			idx = i
			break
		}
	}
	if idx >= 0 {
		entries[idx].Count++
		entries[idx].LastTs = ts
		if target != "" {
			entries[idx].Target = target
		}
	} else {
		entries = append(entries, StatsEntry{Name: name, Count: 1, LastTs: ts, Target: target})
	}

	data, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("write stats %s: %w", path, err)
	}
	return nil
}
