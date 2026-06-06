package gh

import (
	"fmt"
	"slices"
	"strings"

	"github.com/lithammer/fuzzysearch/fuzzy"
	"github.com/xbpk3t/docs-alfred/pkg/urlutil"
)

// FindEntry represents a gh entry found by search with its relevance score.
type FindEntry struct {
	File     string
	RepoURL  string
	Relation string
	SecType  string
	Topic    string
	Des      string
	Zk       string
	Records  int
	Score    int
}

// FindEntries searches for entries in gh data matching a text query or exact URL.
func FindEntries(ghRoot, query, findURL string) ([]FindEntry, error) {
	var entries []FindEntry
	lowerQuery := strings.ToLower(strings.TrimSpace(query))

	err := WalkGhRepos(ghRoot, func(ev WalkerEvent) error {
		if ev.Type != evTypeRepo {
			return nil
		}

		repo := ev.Repo
		repoURL, _ := repo["url"].(string)
		if repoURL == "" {
			return nil
		}

		e := scoreEntry(repo, repoURL, lowerQuery, findURL, &ev)
		if e.Score > 0 {
			entries = append(entries, e)
		}

		return nil
	})

	return entries, err
}

// SortEntries sorts entries by score descending.
func SortEntries(entries []FindEntry) {
	slices.SortStableFunc(entries, func(a, b FindEntry) int {
		return b.Score - a.Score
	})
}

// FormatEntriesResult returns a human-readable result list.
func FormatEntriesResult(entries []FindEntry) string {
	var out strings.Builder

	if len(entries) == 0 {
		out.WriteString("No entries found.\n")

		return out.String()
	}

	fmt.Fprintf(&out, "Found %d result(s):\n\n", len(entries))
	for i := range entries {
		e := &entries[i]
		title := e.Topic
		if title == "" {
			title = e.RepoURL
		}
		fmt.Fprintf(&out, "[%d] %s\n", i+1, title)
		fmt.Fprintf(&out, "    file:  %s\n", e.File)
		fmt.Fprintf(&out, "    url:   %s\n", e.RepoURL)
		fmt.Fprintf(&out, "    rel:   %s\n", e.Relation)
		if e.SecType != "" {
			fmt.Fprintf(&out, "    type:  %s\n", e.SecType)
		}
		if e.Zk != "" {
			fmt.Fprintf(&out, "    zk:    %s\n", e.Zk)
		}
		if e.Records > 0 {
			fmt.Fprintf(&out, "    records: %d\n", e.Records)
		}
		out.WriteString("\n")
	}

	return out.String()
}

// ---- internal scoring helpers ----

func scoreEntry(repo map[string]any, repoURL, lowerQuery, findURL string, ev *WalkerEvent) FindEntry {
	if findURL != "" {
		return scoreEntryByExactURL(repo, repoURL, findURL, ev)
	}

	return scoreEntryByTextQuery(repo, repoURL, lowerQuery, ev)
}

func scoreEntryByExactURL(repo map[string]any, repoURL, findURL string, ev *WalkerEvent) FindEntry {
	if !urlutil.Equal(repoURL, findURL) {
		return FindEntry{} // no match
	}

	topics, _ := repo["topics"].([]any)
	var topicNames []string
	for _, t := range topics {
		if topic, ok := t.(map[string]any); ok {
			if tn, ok := topic["topic"].(string); ok && tn != "" {
				topicNames = append(topicNames, tn)
			}
		}
	}
	des, _ := repo["des"].(string)
	secType, _ := ev.Section["type"].(string)

	return FindEntry{
		File:     "data/gh/" + ev.File,
		RepoURL:  repoURL,
		Relation: ev.Relation,
		SecType:  secType,
		Topic:    strings.Join(topicNames, ", "),
		Des:      des,
		Score:    100,
	}
}

func scoreByURLMatch(repoURL, lowerQuery string) int {
	if lowerQuery == "" {
		return 0
	}
	if urlutil.Equal(repoURL, lowerQuery) {
		return 100
	}
	if strings.Contains(strings.ToLower(repoURL), lowerQuery) {
		return 80
	}
	if isFuzzyMatch(lowerQuery, repoURL) {
		return 50
	}

	return 0
}

func scoreByNameMatch(repoURL, lowerQuery string) int {
	if lowerQuery == "" {
		return 0
	}
	repoName := strings.ToLower(urlutil.RepoName(repoURL))
	if repoName == lowerQuery {
		return 90
	}
	if strings.Contains(repoName, lowerQuery) {
		return 70
	}
	if isFuzzyMatch(lowerQuery, repoName) {
		return 55
	}

	return 0
}

func scoreByTextMatch(des, zk, lowerQuery string) int {
	if lowerQuery == "" {
		return 0
	}
	if strings.Contains(strings.ToLower(des), lowerQuery) ||
		strings.Contains(strings.ToLower(zk), lowerQuery) {
		return 60
	}
	if isFuzzyMatch(lowerQuery, des) || isFuzzyMatch(lowerQuery, zk) {
		return 40
	}

	return 0
}

func scoreByTopicMatch(topics []any, lowerQuery string) int {
	if lowerQuery == "" {
		return 0
	}
	for _, t := range topics {
		topic, ok := t.(map[string]any)
		if !ok {
			continue
		}
		topicName, ok := topic["topic"].(string)
		if !ok {
			continue
		}
		if strings.Contains(strings.ToLower(topicName), lowerQuery) {
			return 85
		}
		if isFuzzyMatch(lowerQuery, topicName) {
			return 65
		}
	}

	return 0
}

func isFuzzyMatch(query, target string) bool {
	if len([]rune(query)) < 3 || target == "" {
		return false
	}

	return fuzzy.MatchFold(query, target)
}

func scoreEntryByTextQuery(repo map[string]any, repoURL, lowerQuery string, ev *WalkerEvent) FindEntry {
	topics, _ := repo["topics"].([]any)

	var score int
	score = max(score, scoreByURLMatch(repoURL, lowerQuery))
	score = max(score, scoreByNameMatch(repoURL, lowerQuery))

	des, _ := repo["des"].(string)
	zk, _ := repo["zk"].(string)
	score = max(score, scoreByTextMatch(des, zk, lowerQuery))
	score = max(score, scoreByTopicMatch(topics, lowerQuery))

	if score <= 0 {
		return FindEntry{}
	}

	var topicNames []string
	recordCount := 0
	for _, t := range topics {
		if topic, ok := t.(map[string]any); ok {
			if tn, ok := topic["topic"].(string); ok && tn != "" {
				topicNames = append(topicNames, tn)
			}
			if recs, ok := topic["record"].([]any); ok {
				recordCount += len(recs)
			}
		}
	}

	secType, _ := ev.Section["type"].(string)

	return FindEntry{
		File:     "data/gh/" + ev.File,
		RepoURL:  repoURL,
		Relation: ev.Relation,
		SecType:  secType,
		Topic:    strings.Join(topicNames, ", "),
		Des:      des,
		Zk:       zk,
		Records:  recordCount,
		Score:    score,
	}
}
