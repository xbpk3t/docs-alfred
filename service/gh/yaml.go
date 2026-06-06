package gh

import (
	"path"
	"strings"

	yaml "github.com/goccy/go-yaml"
	"github.com/xbpk3t/docs-alfred/pkg/parser"
	"github.com/xbpk3t/docs-alfred/pkg/render"
	"github.com/xbpk3t/docs-alfred/service"
)

type GithubYAMLRender struct {
	*render.YAMLRenderer

	currentFile string
	tag         string
}

func NewGithubYAMLRender(tag string) *GithubYAMLRender {
	return &GithubYAMLRender{
		YAMLRenderer: render.NewYAMLRenderer(string(service.ServiceGithub), true),
		tag:          tag,
	}
}

// GetCurrentFileName 获取当前处理的文件名.
func (g *GithubYAMLRender) GetCurrentFileName() string {
	return g.currentFile
}

//// SetCurrentFile 设置当前处理的文件名
// func (g *GithubYAMLRender) SetCurrentFile(filename string) {
//	g.currentFile = filename
//}

func (g *GithubYAMLRender) Render(data []byte) (string, error) {
	// 解析YAML数据为ConfigRepos类型
	rc, err := parser.NewParser[ConfigRepo](data).WithFileName(g.GetCurrentFileName()).ParseFlatten()
	if err != nil {
		return "", err
	}

	// 从目录名注入 tag（仅在数据源未显式设置时注入）
	for i := range rc {
		if rc[i].Tag == "" {
			rc[i].Tag = g.tag
		}
		normalizeConfigRepo(&rc[i])
	}

	// 将数据编码为YAML格式
	result, err := yaml.Marshal(rc)
	if err != nil {
		return "", err
	}

	return string(result), nil
}

func normalizeConfigRepo(config *ConfigRepo) {
	base := topicBase(config.Tag, config.Type)

	normalizeTopics(config.Topics, base)
	normalizeRepoTopics(&config.Using, base, true)
	for _, repo := range config.Repos {
		normalizeRepoTopics(repo, base, false)
	}
}

func normalizeRepoTopics(repo *Repository, base string, useBase bool) {
	if repo == nil {
		return
	}

	topicBase := base
	if !useBase {
		repoName := repoNameFromURL(repo.URL)
		if repoName == "" {
			return
		}
		topicBase = joinPath(base, repoName)
	}

	normalizeTopics(repo.Topics, topicBase)
}

func normalizeTopics(topics Topics, base string) {
	for i := range topics {
		normalizeTopic(&topics[i], base)
	}
}

func normalizeTopic(topic *Topic, base string) {
	if topic.Meta != nil {
		if topic.Meta.HasPic {
			topic.HasPic = true
		}
		if topic.Meta.IsX {
			topic.IsX = true
		}
	}

	topicDir := topicDirName(topic)
	topicBase := joinPath(base, topicDir)
	if topic.PicDir == "" && topic.HasPic && topicBase != "" {
		topic.PicDir = topicBase
	}

	normalizeTopics(topic.Sub, topicBase)

	topic.Meta = nil
}

func topicBase(tag, typeName string) string {
	if tag == "" || typeName == "" {
		return ""
	}

	return joinPath(tag, typeName)
}

func topicDirName(topic *Topic) string {
	if topic == nil {
		return ""
	}
	if topic.Meta != nil && topic.Meta.Slug != "" {
		return topic.Meta.Slug
	}

	return topic.Topic
}

func repoNameFromURL(urlStr string) string {
	if urlStr == "" {
		return ""
	}
	cleaned := strings.TrimPrefix(urlStr, "https://")
	cleaned = strings.TrimPrefix(cleaned, "http://")
	cleaned = strings.TrimSuffix(cleaned, "/")
	parts := strings.Split(cleaned, "/")
	if len(parts) == 0 {
		return ""
	}

	return strings.TrimSuffix(parts[len(parts)-1], ".git")
}

func joinPath(parts ...string) string {
	cleanParts := make([]string, 0, len(parts))
	for _, part := range parts {
		if part != "" {
			cleanParts = append(cleanParts, part)
		}
	}
	if len(cleanParts) == 0 {
		return ""
	}

	return path.Join(cleanParts...)
}
