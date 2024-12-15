package gh

import (
	"fmt"
	"strings"
	"time"
)

type Repository struct {
	LastUpdated time.Time
	Type        string `yaml:"type"`
	URL         string `yaml:"url"`
	Name        string `yaml:"name,omitempty"`
	User        string
	Des         string   `yaml:"des,omitempty"`
	Doc         string   `yaml:"doc,omitempty"`
	Tag         string   `yaml:"tag,omitempty"`
	Qs          Qs       `yaml:"qs,omitempty"`
	Sub         Repos    `yaml:"sub,omitempty"`
	Rep         Repos    `yaml:"rep,omitempty"`
	Cmd         []string `yaml:"cmd,omitempty"`
	IsStar      bool
}

type Repos []Repository

// IsSubRepo 判断是否为子仓库或关联仓库
func (r Repository) IsSubRepo() bool {
	return r.Type == "sub" || r.Type == "rep"
}

// FullName 返回仓库的完整名称
func (r Repository) FullName() string {
	return fmt.Sprintf("%s/%s", r.User, r.Name)
}

// GetMainRepo 获取主仓库信息
func (r Repository) GetMainRepo() string {
	// 从Type中提取主仓库信息
	// 格式例如: "lib [SUB: owner/repo]" 或 "tool [DEP: owner/repo]"
	if r.IsSubRepo() {
		parts := strings.Split(r.Type, "[")
		if len(parts) == 2 {
			// 提取 owner/repo 部分
			repoInfo := strings.TrimSuffix(parts[1], "]")
			repoInfo = strings.TrimPrefix(repoInfo, "SUB: ")
			repoInfo = strings.TrimPrefix(repoInfo, "DEP: ")
			return repoInfo
		}
	}
	return r.FullName()
}
