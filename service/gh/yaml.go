package gh

import (
	"strings"

	"github.com/goccy/go-yaml"
	"github.com/xbpk3t/docs-alfred/pkg/parser"
)

type GithubYAMLRender struct {
	currentFile string
}

func NewGithubYAMLRender() *GithubYAMLRender {
	return &GithubYAMLRender{}
}

// GetCurrentFileName 获取当前处理的文件名
func (g *GithubYAMLRender) GetCurrentFileName() string {
	return g.currentFile
}

// SetCurrentFile 设置当前处理的文件名
func (g *GithubYAMLRender) SetCurrentFile(filename string) {
	g.currentFile = filename
}

func (gfr *GithubYAMLRender) Render(data []byte) (string, error) {
	// 解析YAML数据为ConfigRepos类型
	rc, err := parser.NewParser[ConfigRepos](data).WithFileName(gfr.GetCurrentFileName()).ParseSingle()
	if err != nil {
		return "", err
	}

	// 获取tag（从文件名）
	tag := strings.TrimSuffix(gfr.GetCurrentFileName(), ".yml")

	// 为每个配置添加tag和type
	rc = rc.WithType().WithTag(tag)

	// 将数据编码为YAML格式
	result, err := yaml.Marshal(rc)
	if err != nil {
		return "", err
	}

	return string(result), nil
}
