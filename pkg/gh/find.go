package gh

import (
	"fmt"
	"os"
	"slices"
	"strings"
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
	lowerQuery := strings.ToLower(query)

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

//nolint:errcheck // stdout CLI output is best-effort
func FormatEntries(entries []FindEntry) {
	if len(entries) == 0 {
		fmt.Fprintln(os.Stdout, "No entries found.")

		return
	}

	fmt.Fprintf(os.Stdout, "Found %d result(s):\n\n", len(entries))
	for i := range entries {
		e := &entries[i]
		title := e.Topic
		if title == "" {
			title = e.RepoURL
		}
		fmt.Fprintf(os.Stdout, "[%d] %s\n", i+1, title)
		fmt.Fprintf(os.Stdout, "    file:  %s\n", e.File)
		fmt.Fprintf(os.Stdout, "    url:   %s\n", e.RepoURL)
		fmt.Fprintf(os.Stdout, "    rel:   %s\n", e.Relation)
		if e.SecType != "" {
			fmt.Fprintf(os.Stdout, "    type:  %s\n", e.SecType)
		}
		if e.Zk != "" {
			fmt.Fprintf(os.Stdout, "    zk:    %s\n", e.Zk)
		}
		if e.Records > 0 {
			fmt.Fprintf(os.Stdout, "    records: %d\n", e.Records)
		}
		fmt.Fprintln(os.Stdout)
	}
}

// ---- internal scoring helpers ----

func scoreEntry(repo map[string]any, repoURL, lowerQuery, findURL string, ev *WalkerEvent) FindEntry {
	if findURL != "" {
		return scoreEntryByExactURL(repo, repoURL, findURL, ev)
	}

	return scoreEntryByTextQuery(repo, repoURL, lowerQuery, ev)
}

func scoreEntryByExactURL(repo map[string]any, repoURL, findURL string, ev *WalkerEvent) FindEntry {
	if !strings.EqualFold(strings.TrimRight(repoURL, "/"), strings.TrimRight(findURL, "/")) {
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
	if strings.EqualFold(repoURL, lowerQuery) {
		return 100
	}
	if strings.Contains(strings.ToLower(repoURL), lowerQuery) {
		return 80
	}

	return 0
}

func scoreByNameMatch(repoURL, lowerQuery string) int {
	parts := strings.Split(repoURL, "/")
	if len(parts) == 0 {
		return 0
	}
	repoName := strings.ToLower(parts[len(parts)-1])
	if repoName == lowerQuery {
		return 90
	}
	if strings.Contains(repoName, lowerQuery) {
		return 70
	}

	return 0
}

func scoreByTextMatch(des, zk, lowerQuery string) int {
	if strings.Contains(strings.ToLower(des), lowerQuery) ||
		strings.Contains(strings.ToLower(zk), lowerQuery) {
		return 60
	}

	return 0
}

func scoreByTopicMatch(topics []any, lowerQuery string) int {
	for _, t := range topics {
		if topic, ok := t.(map[string]any); ok {
			if topicName, ok := topic["topic"].(string); ok {
				if strings.Contains(strings.ToLower(topicName), lowerQuery) {
					return 85
				}
			}
		}
	}

	return 0
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
