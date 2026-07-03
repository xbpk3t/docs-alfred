package ghindex

import (
	"slices"
	"strings"

	"github.com/samber/lo"
	"github.com/xbpk3t/docs-alfred/internal/gh/content"
	"github.com/xbpk3t/docs-alfred/pkg/urlutil"
)

type repoMatch struct {
	repo  *content.Repo
	score int
	index int
}

// FilterRepos filters repositories by query string.
func FilterRepos(r Repos, query string) Repos {
	if len(r) == 0 {
		return nil
	}
	query = normalizeSearchQuery(query)
	if query == "" {
		return r
	}

	matches := make([]repoMatch, 0, len(r))
	for i, repo := range r {
		if score, ok := matchRepo(repo, query); ok {
			matches = append(matches, repoMatch{repo: repo, score: score, index: i})
		}
	}
	slices.SortStableFunc(matches, func(a, b repoMatch) int {
		if a.score < b.score {
			return -1
		}
		if a.score > b.score {
			return 1
		}
		if a.index < b.index {
			return -1
		}
		if a.index > b.index {
			return 1
		}

		return 0
	})

	return lo.Map(matches, func(match repoMatch, _ int) *content.Repo {
		return match.repo
	})
}

func normalizeSearchQuery(query string) string {
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		return ""
	}
	if repo, ok := urlutil.GitHubOwnerRepo(query); ok {
		return strings.ToLower(repo.Owner + "/" + repo.Name)
	}
	if repo, ok := urlutil.GitHubOwnerRepo("https://" + query); ok {
		return strings.ToLower(repo.Owner + "/" + repo.Name)
	}

	return strings.TrimSuffix(query, ".git")
}

func matchRepo(repo *content.Repo, query string) (int, bool) {
	if repo == nil {
		return 0, false
	}
	fullName := strings.ToLower(FullName(repo))
	name := repoNameFromFullName(fullName)

	switch {
	case fullName == query:
		return 0, true
	case name == query:
		return 1, true
	case strings.HasSuffix(fullName, query):
		return 2, true
	case strings.Contains(fullName, query):
		return 3, true
	}

	// Slash queries are path-oriented, matching old Alfred item-title filtering.
	// Do not match metadata for queries like /git, otherwise github.com-style URL
	// prefixes and prose descriptions drown out the intended repo-path matches.
	if strings.Contains(query, "/") {
		return 0, false
	}

	switch {
	case strings.Contains(strings.ToLower(repo.Tag), query):
		return 4, true
	case strings.Contains(strings.ToLower(repo.Type), query):
		return 4, true
	case strings.Contains(strings.ToLower(repo.Des), query):
		return 5, true
	}

	return 0, false
}

func repoNameFromFullName(fullName string) string {
	_, name, found := strings.Cut(fullName, "/")
	if !found {
		return fullName
	}

	return name
}

// ExtractTags extracts unique tags from repositories.
func ExtractTags(r Repos) []string {
	tags := lo.Uniq(lo.Map(lo.Filter(r, func(repo *content.Repo, _ int) bool {
		return repo.Tag != ""
	}), func(repo *content.Repo, _ int) string {
		return repo.Tag
	}))
	slices.Sort(tags)

	return tags
}

// ExtractTypesByTag returns all types for a given tag.
func ExtractTypesByTag(r Repos, tag string) []string {
	return lo.Uniq(lo.Map(lo.Filter(r, func(repo *content.Repo, _ int) bool {
		return repo.Tag == tag && repo.Type != ""
	}), func(repo *content.Repo, _ int) string {
		return repo.Type
	}))
}

// QueryReposByTag filters repos by tag (type).
func QueryReposByTag(r Repos, tag string) Repos {
	return lo.Filter(r, func(repo *content.Repo, _ int) bool {
		return repo.Type == tag
	})
}

// QueryReposByTagAndType filters repos by tag and type.
func QueryReposByTagAndType(r Repos, tag, typeName string) Repos {
	return lo.Filter(r, func(repo *content.Repo, _ int) bool {
		return repo.Tag == tag && repo.Type == typeName
	})
}
