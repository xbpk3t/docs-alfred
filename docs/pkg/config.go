package pkg

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/xbpk3t/docs-alfred/pkg/render"
	"github.com/xbpk3t/docs-alfred/service/gh"
	"github.com/xbpk3t/docs-alfred/service/goods"
	"github.com/xbpk3t/docs-alfred/service/works"
	"github.com/xbpk3t/docs-alfred/service/ws"
)

// DocsConfig 定义配置结构
type DocsConfig struct {
	Markdown *Markdown `yaml:"md"`   // Using pointer to allow nil checks
	JSON     *JSON     `yaml:"json"` // Using pointer to allow nil checks
	YAML     *YAML     `yaml:"yaml"`
	Src      string    `yaml:"src"` // 源路径
	Cmd      string    `yaml:"cmd"` // 命令类型
	IsDir    bool      `yaml:"-"`   // 是否为文件夹，根据src自动判断
}

// NewDocsConfig 创建新的配置实例
func NewDocsConfig(src, cmd string) *DocsConfig {
	return &DocsConfig{
		Src: src,
		Cmd: cmd,
	}
}

// Process 处理配置
func (dc *DocsConfig) Process() error {
	// 获取绝对路径
	absPath, err := filepath.Abs(dc.Src)
	if err != nil {
		return fmt.Errorf("get absolute path error: %w", err)
	}
	dc.Src = absPath

	// 检查路径是否存在并设置IsDir
	fileInfo, err := os.Stat(dc.Src)
	if err != nil {
		return fmt.Errorf("stat path error: %w", err)
	}
	dc.IsDir = fileInfo.IsDir()

	// 处理 Markdown 输出
	if dc.Markdown != nil {
		if err := dc.parseMarkdown(); err != nil {
			return fmt.Errorf("parse Markdown error: %w", err)
		}
	}

	// 处理 JSON 输出
	if dc.JSON != nil {
		if err := dc.parseJSON(); err != nil {
			return fmt.Errorf("parse JSON error: %w", err)
		}
	}

	if dc.YAML != nil {
		if err := dc.parseYAML(); err != nil {
			slog.Error("parse YAML error", slog.String("file", dc.Src))
			return fmt.Errorf("parse JSON error: %w", err)
		}
	}

	return nil
}

// parseMarkdown 处理Markdown配置
func (dc *DocsConfig) parseMarkdown() error {
	if dc.Markdown == nil {
		return nil
	}

	// 创建渲染器
	renderer, err := dc.createMarkdownRenderer()
	if err != nil {
		return fmt.Errorf("create renderer error: %w", err)
	}

	// 如果是 GithubMarkdownRender，设置当前文件
	if gr, ok := renderer.(*gh.GithubMarkdownRender); ok {
		gr.SetCurrentFile(filepath.Base(dc.Src))
	}

	// 处理文件
	return dc.Markdown.ProcessFile(dc.Src, renderer)
}

// parseJSON 处理JSON配置
func (dc *DocsConfig) parseJSON() error {
	if dc.JSON == nil {
		return nil
	}

	// 创建渲染器
	renderer, err := dc.createJSONRenderer()
	if err != nil {
		return fmt.Errorf("create renderer error: %w", err)
	}

	// 处理文件
	return dc.JSON.ProcessFile(dc.Src, renderer)
}

func (dc *DocsConfig) parseYAML() error {
	if dc.YAML == nil {
		return nil
	}

	// 创建渲染器
	renderer, err := dc.createYAMLRenderer()
	if err != nil {
		return fmt.Errorf("create renderer error: %w", err)
	}

	// 处理文件
	return dc.YAML.ProcessFile(dc.Src, renderer)
}

func (dc *DocsConfig) createJSONRenderer() (render.Renderer, error) {
	// 如果配置了JSON输出，使用JSON渲染器
	if dc.JSON != nil {
		renderer := render.NewJSONRenderer(dc.Cmd, true)

		// 根据不同的命令类型设置解析模式
		switch dc.Cmd {
		case "goods":
			renderer.WithParseMode(render.ParseFlatten)
		case "works", "diary", "gh":
			renderer.WithParseMode(render.ParseMulti)
		default:
			renderer.WithParseMode(render.ParseSingle)
		}

		return renderer, nil
	}
	return nil, fmt.Errorf("please add JSON for entity: %s", dc.Cmd)
}

func (dc *DocsConfig) createYAMLRenderer() (render.Renderer, error) {
	// 如果配置了JSON输出，使用JSON渲染器
	if dc.YAML != nil {
		renderer := render.NewYAMLRenderer(dc.Cmd, true)

		// 根据不同的命令类型设置解析模式
		switch dc.Cmd {
		case "gh":
			gh.NewGithubYAMLRender()
		case "goods":
			renderer.WithParseMode(render.ParseFlatten)
		case "works", "diary":
			renderer.WithParseMode(render.ParseMulti)
		default:
			renderer.WithParseMode(render.ParseSingle)
		}

		return renderer, nil
	}
	return nil, fmt.Errorf("please add JSON for entity: %s", dc.Cmd)
}

// createMarkdownRenderer 创建渲染器
func (dc *DocsConfig) createMarkdownRenderer() (render.Renderer, error) {
	if dc.Markdown != nil {
		// 否则使用对应的Markdown渲染器
		switch dc.Cmd {
		case "works":
			return works.NewWorkRenderer(), nil
		case "gh":
			return gh.NewGithubMarkdownRender(), nil
		case "ws":
			return ws.NewWebStackRenderer(), nil
		case "goods":
			return goods.NewGoodsMarkdownRenderer(), nil
		default:
			return nil, fmt.Errorf("markdown Render fail: unknown command: %s", dc.Cmd)
		}
	}

	return nil, fmt.Errorf("please add markdown for entity: %s", dc.Cmd)
}
