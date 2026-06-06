package gh

import (
	"slices"
	"strings"

	"github.com/samber/lo"
)

// Filter filters repositories by query string.
func (r Repos) Filter(query string) Repos {
	if len(r) == 0 {
		return nil
	}
	if query == "" {
		return r
	}

	query = strings.ToLower(query)

	return lo.Filter(r, func(repo *Repository, _ int) bool {
		return strings.Contains(strings.ToLower(repo.URL), query) ||
			strings.Contains(strings.ToLower(repo.Des), query) ||
			strings.Contains(strings.ToLower(repo.Type), query) ||
			strings.Contains(strings.ToLower(repo.Tag), query)
	})
}

// ExtractTags extracts unique tags from repositories.
func (r Repos) ExtractTags() []string {
	tags := lo.Uniq(lo.Map(lo.Filter(r, func(repo *Repository, _ int) bool {
		return repo.Tag != ""
	}), func(repo *Repository, _ int) string {
		return repo.Tag
	}))
	slices.Sort(tags)

	return tags
}

// ExtractTypesByTag returns all types for a given tag.
func (r Repos) ExtractTypesByTag(tag string) []string {
	return lo.Uniq(lo.Map(lo.Filter(r, func(repo *Repository, _ int) bool {
		return repo.Tag == tag && repo.Type != ""
	}), func(repo *Repository, _ int) string {
		return repo.Type
	}))
}

// QueryReposByTag filters repos by tag (type).
func (r Repos) QueryReposByTag(tag string) Repos {
	return lo.Filter(r, func(repo *Repository, _ int) bool {
		return repo.Type == tag
	})
}

// QueryReposByTagAndType filters repos by tag and type.
func (r Repos) QueryReposByTagAndType(tag, typeName string) Repos {
	return lo.Filter(r, func(repo *Repository, _ int) bool {
		return repo.Tag == tag && repo.Type == typeName
	})
}
