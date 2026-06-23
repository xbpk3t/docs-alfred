package ghdata

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFindEntries_ByURL(t *testing.T) {
	tmpDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "go.yml"), []byte(`- type: language
  repo:
    - url: https://github.com/acme/tool
      des: A tool
      zk: zk link
      topics:
        - topic: overview
          record:
            - date: 2024-01-01
              des: initial
`), 0644))

	entries, err := FindEntries(tmpDir, "", "https://github.com/acme/tool")
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, 100, entries[0].Score)
	assert.Contains(t, entries[0].File, "go.yml")
}

func TestFindEntries_ByQuery(t *testing.T) {
	tmpDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "go.yml"), []byte(`- type: language
  repo:
    - url: https://github.com/acme/tool
      des: awesome tool
`), 0644))

	entries, err := FindEntries(tmpDir, "tool", "")
	require.NoError(t, err)
	require.NotEmpty(t, entries)
	assert.True(t, entries[0].Score > 0)
}

func TestFindEntries_NoMatch(t *testing.T) {
	tmpDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "go.yml"), []byte(`- type: language
  repo:
    - url: https://github.com/acme/tool
      des: a tool
`), 0644))

	entries, err := FindEntries(tmpDir, "zzzznonexistent", "")
	require.NoError(t, err)
	assert.Empty(t, entries)
}

func TestSortEntries(t *testing.T) {
	entries := []FindEntry{
		{Score: 50},
		{Score: 100},
		{Score: 75},
	}
	SortEntries(entries)
	assert.Equal(t, 100, entries[0].Score)
	assert.Equal(t, 75, entries[1].Score)
	assert.Equal(t, 50, entries[2].Score)
}

func TestFormatEntriesResult_Empty(t *testing.T) {
	result := FormatEntriesResult(nil)
	assert.Contains(t, result, "No entries found")
}

func TestFormatEntriesResult_WithEntries(t *testing.T) {
	entries := []FindEntry{
		{
			File:     "data/gh/go.yml",
			RepoURL:  "https://github.com/acme/tool",
			Relation: "repo",
			SecType:  "language",
			Topic:    "overview",
			Des:      "a tool",
			Zk:       "zk link",
			Records:  2,
			Score:    100,
		},
	}
	result := FormatEntriesResult(entries)
	assert.Contains(t, result, "Found 1 result(s)")
	assert.Contains(t, result, "https://github.com/acme/tool")
	assert.Contains(t, result, "overview")
	assert.Contains(t, result, "records: 2")
}

func TestScoreByURLMatch(t *testing.T) {
	tests := []struct {
		name      string
		repoURL   string
		query     string
		wantScore int
	}{
		{"exact", "https://github.com/a/b", "https://github.com/a/b", 100},
		{"contains", "https://github.com/a/b", "github.com/a", 80},
		{"no match", "https://github.com/a/b", "zzzzz", 0},
		{"empty query", "https://github.com/a/b", "", 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score := scoreByURLMatch(tt.repoURL, tt.query)
			assert.Equal(t, tt.wantScore, score)
		})
	}
}

func TestScoreByNameMatch(t *testing.T) {
	tests := []struct {
		name      string
		repoURL   string
		query     string
		wantScore int
	}{
		{"exact name", "https://github.com/a/myrepo", "myrepo", 90},
		{"contains name", "https://github.com/a/myrepo", "repo", 70},
		{"no match", "https://github.com/a/myrepo", "zzzzz", 0},
		{"empty", "https://github.com/a/myrepo", "", 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score := scoreByNameMatch(tt.repoURL, tt.query)
			assert.Equal(t, tt.wantScore, score)
		})
	}
}

func TestScoreByTextMatch(t *testing.T) {
	tests := []struct {
		name      string
		des       string
		zk        string
		query     string
		wantScore int
	}{
		{"des match", "awesome tool", "", "awesome", 60},
		{"zk match", "", "zk link", "link", 60},
		{"no match", "tool", "link", "zzzzz", 0},
		{"empty", "", "", "", 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score := scoreByTextMatch(tt.des, tt.zk, tt.query)
			assert.Equal(t, tt.wantScore, score)
		})
	}
}

func TestScoreByTopicMatch(t *testing.T) {
	tests := []struct {
		name      string
		topics    []Topic
		query     string
		wantScore int
	}{
		{"match", []Topic{{Topic: "overview"}}, "overview", 85},
		{"no match", []Topic{{Topic: "overview"}}, "zzzzz", 0},
		{"empty topic name", []Topic{{Topic: ""}}, "test", 0},
		{"empty query", []Topic{{Topic: "overview"}}, "", 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score := scoreByTopicMatch(tt.topics, tt.query)
			assert.Equal(t, tt.wantScore, score)
		})
	}
}

func TestIsFuzzyMatch(t *testing.T) {
	tests := []struct {
		name   string
		query  string
		target string
		want   bool
	}{
		{"short query", "ab", "target", false},
		{"empty target", "query", "", false},
		{"exact", "test", "test", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, isFuzzyMatch(tt.query, tt.target))
		})
	}
}

func TestScoreEntryByExactURL(t *testing.T) {
	repo := &Repo{
		URL: "https://github.com/acme/tool",
		Topics: []Topic{
			{Topic: "overview"},
			{Topic: "usage"},
		},
	}
	ev := &WalkerEvent{
		File:         "go.yml",
		Relation:     evTypeRepo,
		Section:      Section{Type: "language"},
	}

	// Match
	entry := scoreEntryByExactURL(repo, repo.URL, "https://github.com/acme/tool", ev)
	assert.Equal(t, 100, entry.Score)
	assert.Equal(t, "overview, usage", entry.Topic)

	// No match
	entry2 := scoreEntryByExactURL(repo, repo.URL, "https://github.com/other/repo", ev)
	assert.Equal(t, 0, entry2.Score)
}

func TestScoreEntryByTextQuery(t *testing.T) {
	repo := &Repo{
		URL: "https://github.com/acme/tool",
		Des: "awesome tool",
		Zk:  "zk link",
		Topics: []Topic{
			{Topic: "overview", Record: []Record{{Date: "2024-01-01", Des: "r1"}}},
		},
	}
	ev := &WalkerEvent{
		File:     "go.yml",
		Relation: evTypeRepo,
		Section:  Section{Type: "language"},
	}

	// URL match
	entry := scoreEntryByTextQuery(repo, repo.URL, "github.com/acme/tool", ev)
	assert.True(t, entry.Score > 0)
	assert.Equal(t, 1, entry.Records)

	// Name match
	entry2 := scoreEntryByTextQuery(repo, repo.URL, "tool", ev)
	assert.True(t, entry2.Score > 0)

	// Text match
	entry3 := scoreEntryByTextQuery(repo, repo.URL, "awesome", ev)
	assert.True(t, entry3.Score > 0)

	// Topic match
	entry4 := scoreEntryByTextQuery(repo, repo.URL, "overview", ev)
	assert.True(t, entry4.Score > 0)

	// No match
	entry5 := scoreEntryByTextQuery(repo, repo.URL, "zzzzzzzz", ev)
	assert.Equal(t, 0, entry5.Score)
}

func TestFormatEntriesResult_WithSecTypeAndZk(t *testing.T) {
	entries := []FindEntry{
		{
			File:    "data/gh/go.yml",
			RepoURL: "https://github.com/acme/tool",
			SecType: "language",
			Zk:      "zk link",
			Score:   80,
		},
	}
	result := FormatEntriesResult(entries)
	assert.Contains(t, result, "type:  language")
	assert.Contains(t, result, "zk:    zk link")
}

func TestFormatEntriesResult_WithoutSecTypeAndZk(t *testing.T) {
	entries := []FindEntry{
		{
			File:    "data/gh/go.yml",
			RepoURL: "https://github.com/acme/tool",
			Relation: "repo",
			Score:   80,
		},
	}
	result := FormatEntriesResult(entries)
	assert.NotContains(t, result, "type:")
	assert.NotContains(t, result, "zk:")
}

func TestScoreByURLMatch_NoMatch(t *testing.T) {
	score := scoreByURLMatch("https://github.com/a/xyz", "qqq")
	assert.Equal(t, 0, score)
}

func TestScoreByNameMatch_FuzzyMatch2(t *testing.T) {
	score := scoreByNameMatch("https://github.com/a/repository", "repositori")
	assert.Equal(t, 55, score)
}

func TestScoreByNameMatch_FuzzyMatchOnly(t *testing.T) {
	score := scoreByNameMatch("https://github.com/a/repository", "repositori")
	assert.Equal(t, 55, score)
}

func TestScoreByTextMatch_FuzzyMatch(t *testing.T) {
	// "repositori" is not a substring of "repository manager" but fuzzy matches
	score := scoreByTextMatch("repository manager", "", "repositori")
	assert.Equal(t, 40, score)
}

func TestScoreByTopicMatch_FuzzyMatch(t *testing.T) {
	// "repositori" doesn't contain-match "repository" but fuzzy matches (levenshtein 1)
	score := scoreByTopicMatch([]Topic{{Topic: "repository"}}, "repositori")
	assert.Equal(t, 65, score)
}

func TestIsFuzzyMatch_LongQuery(t *testing.T) {
	// Query longer than 5 chars allows 2 edits
	assert.True(t, isFuzzyMatch("reposito", "repository"))
}

func TestIsFuzzyMatch_WordMatch(t *testing.T) {
	// Match against individual words in target using fuzzy.MatchFold
	assert.True(t, isFuzzyMatch("tool", "tool kit"))
}

func TestFindEntries_EmptyDir(t *testing.T) {
	tmpDir := t.TempDir()
	entries, err := FindEntries(tmpDir, "test", "")
	require.NoError(t, err)
	assert.Empty(t, entries)
}

func TestFindEntries_EmptyQueryAndURL(t *testing.T) {
	tmpDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "go.yml"), []byte(`- type: language
  repo:
    - url: https://github.com/acme/tool
`), 0644))

	// Both query and findURL empty → no score → empty result
	entries, err := FindEntries(tmpDir, "", "")
	require.NoError(t, err)
	assert.Empty(t, entries)
}

func TestFindEntries_EmptyRepoURL(t *testing.T) {
	tmpDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "go.yml"), []byte(`- type: language
  repo:
    - des: no url
`), 0644))

	entries, err := FindEntries(tmpDir, "test", "")
	require.NoError(t, err)
	assert.Empty(t, entries)
}

func TestScoreEntry_ExactURLMatch(t *testing.T) {
	repo := &Repo{
		URL: "https://github.com/acme/tool",
		Topics: []Topic{
			{Topic: "overview"},
		},
	}
	ev := &WalkerEvent{
		File:     "go.yml",
		Relation: evTypeRepo,
		Section:  Section{Type: "language"},
	}

	entry := scoreEntry(repo, repo.URL, "", "https://github.com/acme/tool", ev)
	assert.Equal(t, 100, entry.Score)
}

func TestScoreEntry_TextQuery(t *testing.T) {
	repo := &Repo{
		URL: "https://github.com/acme/tool",
		Des: "a great tool",
	}
	ev := &WalkerEvent{
		File:     "go.yml",
		Relation: evTypeRepo,
		Section:  Section{Type: "language"},
	}

	entry := scoreEntry(repo, repo.URL, "great", "", ev)
	assert.True(t, entry.Score > 0)
}
