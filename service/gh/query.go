package gh

import (
	"slices"
	"strings"
)

// Filter filters repositories by query string.
func (r Repos) Filter(query string) Repos {
	if query == "" {
		return r
	}

	query = strings.ToLower(query)
	var filtered Repos

	for _, repo := range r {
		if strings.Contains(strings.ToLower(repo.URL), query) ||
			strings.Contains(strings.ToLower(repo.Des), query) ||
			strings.Contains(strings.ToLower(repo.Type), query) ||
			strings.Contains(strings.ToLower(repo.Tag), query) {
			filtered = append(filtered, repo)
		}
	}

	return filtered
}

// ExtractTags extracts unique tags from repositories.
func (r Repos) ExtractTags() []string {
	tagMap := make(map[string]struct{})
	for _, repo := range r {
		if repo.Tag != "" {
			tagMap[repo.Tag] = struct{}{}
		}
	}

	tags := make([]string, 0, len(tagMap))
	for tag := range tagMap {
		tags = append(tags, tag)
	}
	slices.Sort(tags)

	return tags
}

// ExtractTypesByTag returns all types for a given tag.
func (r Repos) ExtractTypesByTag(tag string) []string {
	types := make(map[string]bool)
	for _, repo := range r {
		if repo.Tag == tag {
			types[repo.Type] = true
		}
	}

	result := make([]string, 0, len(types))
	for t := range types {
		if t != "" {
			result = append(result, t)
		}
	}

	return result
}

// QueryReposByTag filters repos by tag (type).
func (r Repos) QueryReposByTag(tag string) Repos {
	var filtered Repos
	for _, rp := range r {
		if rp.Type == tag {
			filtered = append(filtered, rp)
		}
	}

	return filtered
}

// QueryReposByTagAndType filters repos by tag and type.
func (r Repos) QueryReposByTagAndType(tag, typeName string) Repos {
	var result Repos
	for _, repo := range r {
		if repo.Tag == tag && repo.Type == typeName {
			result = append(result, repo)
		}
	}

	return result
}
