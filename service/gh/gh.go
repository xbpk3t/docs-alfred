package gh

import (
	"strings"
)

const GhURL = "https://github.com/"

// Repository 定义仓库结构
type Repository struct {
	Doc            string    `yaml:"doc,omitempty"`
	Des            string    `yaml:"des,omitempty"`
	URL            string    `yaml:"url"`
	Tag            string    `yaml:"tag,omitempty"`
	Type           string    `yaml:"type"`
	MainRepo       string    // 如果是sub, replaced, related repos 就需要设置这个参数（gh-merge中会自动设置）
	Qs             Questions `yaml:"qs,omitempty"`
	SubRepos       Repos     `yaml:"sub,omitempty"`
	ReplacedRepos  Repos     `yaml:"rep,omitempty"`
	RelatedRepos   Repos     `yaml:"rel,omitempty"`
	Cmd            []string  `yaml:"cmd,omitempty"`
	IsSubRepo      bool
	IsReplacedRepo bool
	IsRelatedRepo  bool
	IsX            bool `yaml:"isX,omitempty"`
}

type Repos []Repository

// ConfigRepo 定义配置仓库结构
type ConfigRepo struct {
	Type  string `yaml:"type"`
	Tag   string `yaml:"tag"`
	Repos Repos  `yaml:"repo"`
}

type ConfigRepos []ConfigRepo

// QA 定义问题结构
type QA struct {
	Question     string   `yaml:"q"` // 问题
	Answer       string   `yaml:"x"` // 简要回答
	Pictures     []string `yaml:"p"` // 图片
	URLs         string   `yaml:"u"` // url
	SubQuestions []string `yaml:"s"` // 子问题
}

type Questions []QA

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
		repo.SubRepos[i].MainRepo = repo.FullName()
		repos = append(repos, processRepo(repo.SubRepos[i], repo.Type)...)
	}

	// 处理替换仓库
	for i := range repo.ReplacedRepos {
		repo.ReplacedRepos[i].IsReplacedRepo = true
		repo.ReplacedRepos[i].Type = repo.Type
		repo.ReplacedRepos[i].MainRepo = repo.FullName()
		repos = append(repos, processRepo(repo.ReplacedRepos[i], repo.Type)...)
	}

	// 处理相关仓库
	for i := range repo.RelatedRepos {
		repo.RelatedRepos[i].IsRelatedRepo = true
		repo.RelatedRepos[i].Type = repo.Type
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
	return len(r.Qs) > 0
}

func (r *Repository) HasSubRepos() bool {
	return len(r.SubRepos) > 0 || len(r.ReplacedRepos) > 0 || len(r.RelatedRepos) > 0
}
