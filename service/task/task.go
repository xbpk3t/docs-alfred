package task

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/xbpk3t/docs-alfred/pkg/errcode"
	"github.com/xbpk3t/docs-alfred/pkg/render"
)

// TaskRenderer 实现 markdown 渲染器接口
type TaskRenderer struct {
	srcDir    string // 源目录
	targetDir string // 目标目录
	fileName  string // 自定义文件名
	render.MarkdownRenderer
}

// NewTaskRenderer 创建新的渲染器
func NewTaskRenderer(srcDir, targetDir, fileName string) *TaskRenderer {
	return &TaskRenderer{
		srcDir:    srcDir,
		targetDir: targetDir,
		fileName:  fileName,
	}
}

// Render 实现渲染接口
func (r *TaskRenderer) Render(data []byte) (string, error) {
	// 获取task目录的绝对路径
	taskDir, err := r.getAbsolutePath(r.srcDir)
	if err != nil {
		return "", err
	}

	// 写入头部内容
	r.renderHeader()

	// 获取并处理task文件
	if err := r.processTaskFiles(taskDir); err != nil {
		return "", err
	}

	// 如果需要，写入文件
	if err := r.writeToFileIfNeeded(); err != nil {
		return "", err
	}

	return r.String(), nil
}

// getAbsolutePath 获取绝对路径
func (r *TaskRenderer) getAbsolutePath(dir string) (string, error) {
	if !filepath.IsAbs(dir) {
		workDir, err := os.Getwd()
		if err != nil {
			return "", errcode.WithError(errcode.ErrWorkDir, err)
		}
		return filepath.Join(workDir, dir), nil
	}
	return dir, nil
}

// renderHeader 渲染头部内容
func (r *TaskRenderer) renderHeader() {
	r.RenderMetadata(map[string]string{
		"slug": "/",
	})
}

// processTaskFiles 处理task文件
func (r *TaskRenderer) processTaskFiles(taskDir string) error {
	// 获取所有task-*.yml文件
	taskFiles, err := filepath.Glob(filepath.Join(taskDir, "task-*.yml"))
	if err != nil {
		return errcode.WithError(errcode.ErrListDir, err)
	}

	// 按文件名排序
	sort.Strings(taskFiles)

	// 处理每个task文件
	for _, taskFile := range taskFiles {
		if err := r.processTaskFile(taskFile); err != nil {
			return errcode.WithError(errcode.ErrFileProcess, err)
		}
	}

	return nil
}

// processTaskFile 处理单个task文件
func (r *TaskRenderer) processTaskFile(taskFile string) error {
	// 获取任务名称
	baseName := filepath.Base(taskFile)
	name := strings.TrimSuffix(baseName, ".yml")
	name = strings.TrimPrefix(name, "task-")

	// 渲染任务内容
	r.renderTaskContent(name, taskFile)
	return nil
}

// renderTaskContent 渲染任务内容
func (r *TaskRenderer) renderTaskContent(name, taskFile string) {
	// 写入标题
	r.RenderHeader(render.HeadingLevel2, name)

	// 使用RenderDocusaurusRawLoader渲染导入和代码块
	r.RenderDocusaurusRawLoader(name, filepath.Join("..", taskFile))
}

// writeToFileIfNeeded 如果需要则写入文件
func (r *TaskRenderer) writeToFileIfNeeded() error {
	if r.targetDir == "" || r.fileName == "" {
		return nil
	}

	if err := os.MkdirAll(r.targetDir, 0o755); err != nil {
		return errcode.WithError(errcode.ErrCreateDir, err)
	}

	content := r.String()
	outputPath := filepath.Join(r.targetDir, r.fileName)
	if err := os.WriteFile(outputPath, []byte(content), 0o644); err != nil {
		return errcode.WithError(errcode.ErrWriteFile, err)
	}

	return nil
}
