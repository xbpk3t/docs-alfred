package gh

import "slices"

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

// QueryReposByTagAndType returns all repositories matching the given tag and type.
func (r Repos) QueryReposByTagAndType(tag, typeName string) Repos {
	var result Repos
	for _, repo := range r {
		if repo.Tag == tag && repo.Type == typeName {
			result = append(result, repo)
		}
	}

	return result
}

// ExtractTags 从所有仓库中提取唯一的标签列表.
func (r Repos) ExtractTags() []string {
	// 使用 map 来去重
	tagMap := make(map[string]struct{})

	// 遍历所有仓库收集标签
	for _, rp := range r {
		if rp.Tag != "" {
			tagMap[rp.Tag] = struct{}{}
		}
	}

	// 将 map 转换为切片
	tags := make([]string, 0, len(tagMap))
	for tag := range tagMap {
		tags = append(tags, tag)
	}

	// 对标签进行排序，使结果稳定
	slices.Sort(tags)

	return tags
}
