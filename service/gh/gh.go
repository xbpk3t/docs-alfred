package gh

import (
	"fmt"
	"slices"
	"strings"

	"github.com/xbpk3t/docs-alfred/pkg/render"

	"github.com/samber/lo"
)

const GhURL = "https://github.com/"

// Repository 定义仓库结构
type Repository struct {
	Doc           string `yaml:"doc,omitempty"`
	Name          string `yaml:"name,omitempty"`
	User          string
	Des           string    `yaml:"des,omitempty"`
	URL           string    `yaml:"url"`
	Tag           string    `yaml:"tag,omitempty"`
	Type          string    `yaml:"type"`
	Qs            Questions `yaml:"qs,omitempty"`
	SubRepos      Repos     `yaml:"sub,omitempty"`
	ReplacedRepos Repos     `yaml:"rep,omitempty"`
	RelatedRepos  Repos     `yaml:"rel,omitempty"`
	Cmd           []string  `yaml:"cmd,omitempty"`
	IsStar        bool
}

type Repos []Repository

// ConfigRepo 定义配置仓库结构
type ConfigRepo struct {
	Type  string `yaml:"type"`
	Repos Repos  `yaml:"repo"`
}

type ConfigRepos []ConfigRepo

// Question 定义问题结构
type Question struct {
	Q string   `yaml:"q"` // 问题
	X string   `yaml:"x"` // 简要回答
	P []string `yaml:"p"` // 图片
	U string   `yaml:"u"` // url
	S []string `yaml:"s"` // 子问题
}

type Questions []Question

// Repository 相关方法
func (r *Repository) SetGithubInfo(owner, name string) {
	r.User = owner
	r.Name = name
	r.IsStar = true
}

func (r *Repository) IsValid() bool {
	return r.User != "" && r.Name != ""
}

func (r *Repository) FullName() string {
	return fmt.Sprintf("%s/%s", r.User, r.Name)
}

func (r *Repository) IsSubRepo() bool {
	return r.Type == "sub" || r.Type == "rep"
}

func (r *Repository) GetMainRepo() string {
	if r.IsSubRepo() {
		if mainRepo := extractMainRepoInfo(r.Type); mainRepo != "" {
			return mainRepo
		}
	}
	return r.FullName()
}

func (cr ConfigRepos) WithTag(tag string) ConfigRepos {
	for _, repo := range cr {
		for i := range repo.Repos {
			repo.Repos[i].Tag = tag
		}
	}
	return cr
}

func (cr ConfigRepos) WithType() ConfigRepos {
	for _, repo := range cr {
		for i := range repo.Repos {
			repo.Repos[i].Type = repo.Type
		}
	}
	return cr
}

func (cr ConfigRepos) ToRepos() Repos {
	var repos Repos
	for _, config := range cr {
		for _, repo := range config.Repos {
			repos = append(repos, processRepo(repo, config.Type)...)
		}
	}
	return repos
}

// ExtractTags 从所有仓库中提取唯一的标签列表
func (r Repos) ExtractTags() []string {
	// 使用 map 来去重
	tagMap := make(map[string]struct{})

	// 遍历所有仓库收集标签
	for _, repo := range r {
		if repo.Type != "" {
			tagMap[repo.Type] = struct{}{}
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

// QueryReposByTag 根据标签筛选仓库
func (r Repos) QueryReposByTag(tag string) Repos {
	var filtered Repos

	// 遍历所有仓库，找出匹配标签的仓库
	for _, repo := range r {
		if repo.Type == tag {
			filtered = append(filtered, repo)
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

	// 处理子仓库和依赖仓库
	repos = append(repos, processSubRepos(repo, configType)...)
	repos = append(repos, processDepRepos(repo, configType)...)

	return repos
}

// processMainRepo 处理主仓库信息
func processMainRepo(repo Repository, configType string) *Repository {
	if !isValidGithubURL(repo.URL) {
		return nil
	}

	owner, name, ok := parseGithubURL(repo.URL)
	if !ok {
		return nil
	}

	repo.SetGithubInfo(owner, name)
	repo.Type = configType

	return &repo
}

// processSubRepos 处理子仓库
func processSubRepos(repo Repository, configType string) Repos {
	var repos Repos
	parentFullName := repo.FullName()

	for _, subRepo := range repo.SubRepos {
		subType := fmt.Sprintf("%s [SUB: %s]", configType, parentFullName)
		repos = append(repos, processRepo(subRepo, subType)...)
	}

	return repos
}

// processDepRepos 处理依赖仓库
func processDepRepos(repo Repository, configType string) Repos {
	var repos Repos
	parentFullName := repo.FullName()

	for _, depRepo := range repo.ReplacedRepos {
		depType := fmt.Sprintf("%s [DEP: %s]", configType, parentFullName)
		repos = append(repos, processRepo(depRepo, depType)...)
	}

	return repos
}

// 工具函数
func isValidGithubURL(url string) bool {
	return strings.Contains(url, GhURL)
}

func parseGithubURL(url string) (owner, name string, ok bool) {
	sx, found := strings.CutPrefix(url, GhURL)
	if !found {
		return "", "", false
	}

	splits := strings.Split(sx, "/")
	if len(splits) != 2 {
		return "", "", false
	}

	return splits[0], splits[1], true
}

func extractMainRepoInfo(typeStr string) string {
	parts := strings.Split(typeStr, "[")
	if len(parts) == 2 {
		repoInfo := strings.TrimSuffix(parts[1], "]")
		repoInfo = strings.TrimPrefix(repoInfo, "SUB: ")
		repoInfo = strings.TrimPrefix(repoInfo, "DEP: ")
		return repoInfo
	}
	return ""
}

func formatQuestionSummary(q Question) string {
	if q.U != "" {
		return fmt.Sprintf("[%s](%s)", q.Q, q.U)
	}
	return q.Q
}

func formatQuestionDetails(q Question) string {
	var parts []string
	renderer := render.NewMarkdownRenderer()

	// 处理图片
	if len(q.P) > 0 {
		var images strings.Builder
		for _, img := range q.P {
			renderer.RenderImageWithFigcaption(img)
			images.WriteString(renderer.String())
		}
		parts = append(parts, images.String())
	}

	// 处理子问题
	if len(q.S) > 0 {
		var subQuestions strings.Builder
		for _, sq := range q.S {
			subQuestions.WriteString(fmt.Sprintf("- %s\n", sq))
		}
		parts = append(parts, subQuestions.String())
	}

	// 处理答案
	if q.X != "" {
		if len(parts) > 0 {
			parts = append(parts, "---")
		}
		parts = append(parts, q.X)
	}

	return strings.Join(parts, "\n\n")
}

// RenderRepositoriesAsMarkdownTable 将仓库列表渲染为Markdown表格
func RenderRepositoriesAsMarkdownTable(repos Repos) string {
	if len(repos) == 0 {
		return ""
	}

	var res strings.Builder
	data := lo.Map(repos, func(item Repository, _ int) []string {
		repoName, _ := strings.CutPrefix(item.URL, GhURL)
		return []string{fmt.Sprintf("[%s](%s)", repoName, item.URL), item.Des}
	})

	render.NewMarkdownRenderer().RenderMarkdownTable([]string{"Repo", "Des"}, &res, data)
	return res.String()
}

// MergeConfig 相关结构和方法
type MergeOptions struct {
	FolderPath string
	OutputPath string
	FileNames  []string
}
