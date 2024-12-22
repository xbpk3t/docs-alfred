package gh

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/xbpk3t/docs-alfred/pkg/parser"
	"github.com/xbpk3t/docs-alfred/pkg/render"

	"github.com/samber/lo"
	"github.com/xbpk3t/docs-alfred/pkg"
	"gopkg.in/yaml.v3"
)

const GhURL = "https://github.com/"

// Repository 定义仓库结构
type Repository struct {
	LastUpdated time.Time
	Type        string `yaml:"type"`
	URL         string `yaml:"url"`
	Name        string `yaml:"name,omitempty"`
	User        string
	Des         string    `yaml:"des,omitempty"`
	Doc         string    `yaml:"doc,omitempty"`
	Tag         string    `yaml:"tag,omitempty"`
	Qs          Questions `yaml:"qs,omitempty"`
	Sub         Repos     `yaml:"sub,omitempty"`
	Rep         Repos     `yaml:"rep,omitempty"`
	Cmd         []string  `yaml:"cmd,omitempty"`
	IsStar      bool
	pkg.URLInfo
}

type Repos []Repository

// ConfigRepo 定义配置仓库结构
type ConfigRepo struct {
	Type  string `yaml:"type"`
	Repos Repos  `yaml:"repos"`
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

// GhRenderer Markdown渲染器
type GhRenderer struct {
	render.MarkdownRenderer
	Config ConfigRepos
}

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

	for _, subRepo := range repo.Sub {
		subType := fmt.Sprintf("%s [SUB: %s]", configType, parentFullName)
		repos = append(repos, processRepo(subRepo, subType)...)
	}

	return repos
}

// processDepRepos 处理依赖仓库
func processDepRepos(repo Repository, configType string) Repos {
	var repos Repos
	parentFullName := repo.FullName()

	for _, depRepo := range repo.Rep {
		depType := fmt.Sprintf("%s [DEP: %s]", configType, parentFullName)
		repos = append(repos, processRepo(depRepo, depType)...)
	}

	return repos
}

// Renderer 相关方法
func NewGhRenderer() *GhRenderer {
	return &GhRenderer{}
}

func (g *GhRenderer) Render(data []byte) (string, error) {
	config, err := parser.NewParser[ConfigRepos](data).ParseSingle()
	if err != nil {
		return "", err
	}
	g.Config = config
	return g.renderContent()
}

func (g *GhRenderer) renderContent() (string, error) {
	for _, repo := range g.Config {
		g.RenderHeader(2, repo.Type)
		g.renderRepos(repo.Repos)
	}
	return g.String(), nil
}

func (g *GhRenderer) renderRepos(repos Repos) {
	for _, repo := range repos {
		if repo.Qs != nil {
			g.RenderHeader(3, g.RenderLink(repo.URL, repo.URL))
			g.renderSubComponents(repo)
			g.renderQuestions(repo.Qs)
		}
	}
}

func (g *GhRenderer) renderSubComponents(repo Repository) {
	// 渲染子仓库
	if len(repo.Sub) > 0 {
		g.renderSubRepos(repo.Sub)
	}

	// 渲染命令
	if len(repo.Cmd) > 0 {
		g.RenderCodeBlock("shell", strings.Join(repo.Cmd, "\n"))
	}
}

func (g *GhRenderer) renderSubRepos(repos Repos) {
	if len(repos) > 0 {
		content := RenderRepositoriesAsMarkdownTable(repos)
		g.RenderAdmonition(render.AdmonitionTip, "Sub Repos", content)
	}
}

func (g *GhRenderer) renderQuestions(qs Questions) {
	for _, q := range qs {
		summary := formatQuestionSummary(q)
		details := formatQuestionDetails(q)
		if details == "" {
			g.RenderListItem(summary)
		} else {
			g.RenderFold(summary, details)
		}
	}
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
	FolderPath string   // 配置文件所在文件夹
	FileNames  []string // 要合并的文件名列表
	OutputPath string   // 输出文件路径
}

type ConfigMerger struct {
	options MergeOptions
}

func NewConfigMerger(opts MergeOptions) *ConfigMerger {
	return &ConfigMerger{options: opts}
}

// Merge
func (m *ConfigMerger) Merge() error {
	if err := m.validateInput(); err != nil {
		return fmt.Errorf("验证输入失败: %w", err)
	}

	config, err := m.mergeConfigs()
	if err != nil {
		return fmt.Errorf("合并配置失败: %w", err)
	}

	return m.writeResult(config)
}

func (m *ConfigMerger) validateInput() error {
	if m.options.FolderPath == "" {
		return fmt.Errorf("文件夹路径不能为空")
	}
	if len(m.options.FileNames) == 0 {
		return fmt.Errorf("文件列表不能为空")
	}
	if m.options.OutputPath == "" {
		return fmt.Errorf("输出路径不能为空")
	}
	return nil
}

// mergeConfigs
func (m *ConfigMerger) mergeConfigs() (ConfigRepos, error) {
	var mergedConfig ConfigRepos

	for _, fileName := range m.options.FileNames {
		config, err := m.processFile(fileName)
		if err != nil {
			return nil, fmt.Errorf("处理文件 %s 失败: %w", fileName, err)
		}
		mergedConfig = append(mergedConfig, config...)
	}

	return mergedConfig, nil
}

func (m *ConfigMerger) processFile(fileName string) (ConfigRepos, error) {
	content, err := m.readFile(fileName)
	if err != nil {
		return nil, err
	}

	tag := strings.TrimSuffix(fileName, ".yml")
	rc := NewConfigRepos(content)
	// rc, err := parser.NewParser[ConfigRepos](content).ParseFlatten()
	// if err != nil {
	// 	return nil, fmt.Errorf("解析YAML失败: %w", err)
	// }

	return rc.WithTag(tag), nil
}

func (m *ConfigMerger) readFile(fileName string) ([]byte, error) {
	filePath := filepath.Join(m.options.FolderPath, fileName)
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("读取文件失败: %w", err)
	}
	return content, nil
}

func (m *ConfigMerger) writeResult(config ConfigRepos) error {
	file, err := os.Create(m.options.OutputPath)
	if err != nil {
		return fmt.Errorf("创建输出文件失败: %w", err)
	}
	defer file.Close()

	encoder := yaml.NewEncoder(file)
	defer encoder.Close()

	if err := encoder.Encode(config); err != nil {
		return fmt.Errorf("编码YAML失败: %w", err)
	}

	return nil
}

func NewConfigRepos(f []byte) ConfigRepos {
	var ghs ConfigRepos

	d := yaml.NewDecoder(bytes.NewReader(f))
	for {
		// create new spec here
		spec := new(ConfigRepos)
		// pass a reference to spec reference
		if err := d.Decode(&spec); err != nil {
			// break the loop in case of EOF
			if errors.Is(err, io.EOF) {
				break
			}
			panic(err)
		}

		ghs = append(ghs, *spec...)
	}
	return ghs
}
