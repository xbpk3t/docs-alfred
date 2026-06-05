package repo

// RepoInfo 定义仓库信息接口.
type RepoInfo interface {
	// GetName 获取仓库名称.
	GetName() string
	// GetDes 获取仓库描述.
	GetDes() string
	// GetURL 获取仓库URL.
	GetURL() string
}

// GetRepoList converts a slice of RepoInfo implementors to a slice of RepoInfo.
func GetRepoList[T RepoInfo](repos []T) []RepoInfo {
	result := make([]RepoInfo, len(repos))
	for i, r := range repos {
		result[i] = r
	}

	return result
}
