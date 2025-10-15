package repo

import (
	"reflect"
)

// RepoInfo 定义仓库信息接口.
type RepoInfo interface {
	// GetName 获取仓库名称.
	GetName() string
	// GetDes 获取仓库描述.
	GetDes() string
	// GetURL 获取仓库URL.
	GetURL() string
}

// GetRepoList 使用反射获取仓库列表.
func GetRepoList(repos any) []RepoInfo {
	var repoList []RepoInfo

	// 获取repos的反射值
	value := reflect.ValueOf(repos)

	// 如果是指针，获取其指向的元素
	if value.Kind() == reflect.Ptr {
		value = value.Elem()
	}

	// 如果是切片
	if value.Kind() == reflect.Slice {
		for i := 0; i < value.Len(); i++ {
			item := value.Index(i).Interface()
			if repo, ok := item.(RepoInfo); ok {
				repoList = append(repoList, repo)
			}
		}
	}

	return repoList
}
