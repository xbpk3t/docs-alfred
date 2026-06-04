package gh

import (
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
		tag:         tag,
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
	}

	// 将数据编码为YAML格式
	result, err := yaml.Marshal(rc)
	if err != nil {
		return "", err
	}

	return string(result), nil
}
