package gh

import (
	"slices"
	"strings"
)

const GhURL = "https://github.com/"

// Repository 定义仓库结构
type Repository struct {
	Doc            string   `yaml:"doc,omitempty"`
	Des            string   `yaml:"des,omitempty"`
	URL            string   `yaml:"url"`
	Tag            string   `yaml:"tag,omitempty"`
	Type           string   `yaml:"type"`
	MainRepo       string   // 如果是sub, replaced, related repos 就需要设置这个参数（gh-merge中会自动设置）
	Topics         Topics   `yaml:"topics,omitempty" json:"topics,omitempty"`
	SubRepos       Repos    `yaml:"sub,omitempty"`
	ReplacedRepos  Repos    `yaml:"rep,omitempty"`
	RelatedRepos   Repos    `yaml:"rel,omitempty"`
	Cmd            []string `yaml:"cmd,omitempty"`
	IsSubRepo      bool
	IsReplacedRepo bool
	IsRelatedRepo  bool
	Score          int `yaml:"score,omitempty"`
}

type Repos []Repository

// ConfigRepo 定义配置仓库结构
type ConfigRepo struct {
	Type  string `yaml:"type"`
	Tag   string `yaml:"tag"`
	Repos Repos  `yaml:"repo"`
}

type ConfigRepos []ConfigRepo

// Topic 定义问题结构
type Topic struct {
	Topic    string   `yaml:"topic" json:"topic"`       // 问题
	Des      string   `yaml:"des" json:"des,omitempty"` // 简要回答
	Pictures []string `yaml:"pic" json:"pic,omitempty"` // 图片
	URLs     string   `yaml:"url" json:"url,omitempty"` // url
	Qs       []string `yaml:"qs" json:"qs,omitempty"`   // 子问题
}

type Topics []Topic

func (r *Repository) IsValid() bool {
	return strings.Contains(r.URL, GhURL)
}

func (r *Repository) FullName() string {
	if !r.IsValid() {
		return ""
	}
	if sx, found := strings.CutPrefix(r.URL, GhURL); found {
		return sx
	}
	return ""
}

// GetDes 获取仓库描述
func (r *Repository) GetDes() string {
	return r.Des
}

// GetURL 获取仓库URL
func (r *Repository) GetURL() string {
	return r.URL
}

func (cr ConfigRepos) ToRepos() Repos {
	var repos Repos
	for _, config := range cr {
		for _, rp := range config.Repos {
			// 设置 Tag 字段
			rp.Tag = config.Tag
			repos = append(repos, processRepo(rp, config.Type)...)
		}
	}
	return repos
}

// QueryReposByTag 根据标签筛选仓库
func (r Repos) QueryReposByTag(tag string) Repos {
	var filtered Repos

	// 遍历所有仓库，找出匹配标签的仓库
	for _, rp := range r {
		if rp.Type == tag {
			filtered = append(filtered, rp)
		}
	}

	return filtered
}

// processRepo 处理仓库及其子仓库
func processRepo(repo Repository, configType string) Repos {
	var repos Repos

	// 处理主仓库
	if mainRepo := processMainRepo(repo, configType); mainRepo != nil {
		repos = append(repos, *mainRepo)
	}

	// 处理所有类型的子仓库
	repos = append(repos, processAllSubRepos(repo)...)

	return repos
}

// processMainRepo 处理主仓库信息
func processMainRepo(repo Repository, configType string) *Repository {
	if !isValidGithubURL(repo.URL) {
		return nil
	}
	repo.Type = configType
	return &repo
}

// processAllSubRepos 处理所有类型的子仓库
func processAllSubRepos(repo Repository) Repos {
	var repos Repos

	// 处理子仓库
	for i := range repo.SubRepos {
		repo.SubRepos[i].IsSubRepo = true
		repo.SubRepos[i].Type = repo.Type
		repo.SubRepos[i].Tag = repo.Tag // 传递 Tag 到子仓库
		repo.SubRepos[i].MainRepo = repo.FullName()
		repos = append(repos, processRepo(repo.SubRepos[i], repo.Type)...)
	}

	// 处理替换仓库
	for i := range repo.ReplacedRepos {
		repo.ReplacedRepos[i].IsReplacedRepo = true
		repo.ReplacedRepos[i].Type = repo.Type
		repo.ReplacedRepos[i].Tag = repo.Tag // 传递 Tag 到替换仓库
		repo.ReplacedRepos[i].MainRepo = repo.FullName()
		repos = append(repos, processRepo(repo.ReplacedRepos[i], repo.Type)...)
	}

	// 处理相关仓库
	for i := range repo.RelatedRepos {
		repo.RelatedRepos[i].IsRelatedRepo = true
		repo.RelatedRepos[i].Type = repo.Type
		repo.RelatedRepos[i].Tag = repo.Tag // 传递 Tag 到相关仓库
		repo.RelatedRepos[i].MainRepo = repo.FullName()
		repos = append(repos, processRepo(repo.RelatedRepos[i], repo.Type)...)
	}

	return repos
}

// 工具函数
func isValidGithubURL(url string) bool {
	return strings.Contains(url, GhURL)
}

// MergeOptions 相关结构和方法
type MergeOptions struct {
	FolderPath string
	OutputPath string
	FileNames  []string
}

// IsSubOrDepOrRelRepo 判断是否为
func (r *Repository) IsSubOrDepOrRelRepo() bool {
	return r.IsSubRepo || r.IsReplacedRepo || r.IsRelatedRepo
}

func (r *Repository) HasQs() bool {
	return len(r.Topics) > 0
}

func (r *Repository) HasSubRepos() bool {
	return len(r.SubRepos) > 0 || len(r.ReplacedRepos) > 0 || len(r.RelatedRepos) > 0
}

// ExtractTags 从所有仓库中提取唯一的标签列表
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
