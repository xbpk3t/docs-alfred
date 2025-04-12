package gh

// ExtractTypesByTag returns all types for a given tag
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

// QueryReposByTagAndType returns all repositories matching the given tag and type
func (r Repos) QueryReposByTagAndType(tag, typeName string) Repos {
	var result Repos
	for _, repo := range r {
		if repo.Tag == tag && repo.Type == typeName {
			result = append(result, repo)
		}
	}
	return result
}
