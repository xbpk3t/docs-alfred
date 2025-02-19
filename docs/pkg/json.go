package pkg

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/gookit/goutil/fsutil"
	"github.com/xbpk3t/docs-alfred/pkg/render"
	"github.com/xbpk3t/docs-alfred/pkg/utils"
)

type JSON struct {
	Dst             string `yaml:"dst"`             // 输出目录
	MergeOutputFile string `yaml:"mergeOutputFile"` // 合并后的输出文件名

	// 内部使用的字段
	currentFile string   // 当前处理的文件名
	Exclude     []string `yaml:"exclude"` // 排除的文件
	IsMerge     bool     `yaml:"isMerge"` // 是否合并模式
}

// GetCurrentFileName 获取当前处理的文件名
func (j *JSON) GetCurrentFileName() string {
	return j.currentFile
}

// SetCurrentFile 设置当前处理的文件名
func (j *JSON) SetCurrentFile(filename string) {
	j.currentFile = filename
}

// ReadInput 读取输入
func (j *JSON) ReadInput(src string, isDir bool) ([]byte, error) {
	if isDir {
		return j.readAndMergeFiles(src)
	}
	return j.readSingleFile(src)
}

// readSingleFile 读取单个文件
func (j *JSON) readSingleFile(src string) ([]byte, error) {
	// 检查src是否是目录
	fileInfo, err := os.Stat(src)
	if err != nil {
		return nil, fmt.Errorf("stat path error: %w", err)
	}

	var inputPath string
	if fileInfo.IsDir() {
		// 如果是目录，读取第一个yml文件
		files, err := os.ReadDir(src)
		if err != nil {
			return nil, fmt.Errorf("read dir error: %w", err)
		}

		for _, file := range files {
			if file.IsDir() || filepath.Ext(file.Name()) != ".yml" {
				continue
			}
			inputPath = filepath.Join(src, file.Name())
			j.SetCurrentFile(file.Name())
			break
		}

		if inputPath == "" {
			return nil, fmt.Errorf("no yml file found in directory: %s", src)
		}
	} else {
		// 如果是文件，直接使用
		inputPath = src
		j.SetCurrentFile(filepath.Base(src))
	}

	data, err := os.ReadFile(inputPath)
	if err != nil {
		return nil, fmt.Errorf("read file error: %w", err)
	}
	return data, nil
}

// readAndMergeFiles 读取并合并文件
func (j *JSON) readAndMergeFiles(src string) ([]byte, error) {
	return utils.ReadAndMergeFilesRecursively(src, j.Exclude, j.SetCurrentFile)
}

// WriteOutput 写入输出
func (j *JSON) WriteOutput(content string, filename string) error {
	// 确保输出目录存在
	outputDir := filepath.Dir(j.Dst)
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return fmt.Errorf("create dir error: %w", err)
	}

	if err := os.WriteFile(j.Dst, []byte(content), 0o644); err != nil {
		return fmt.Errorf("write file error: %w", err)
	}

	return nil
}

// ProcessFile 核心方法：处理单个文件
func (j *JSON) ProcessFile(src string, renderer render.Renderer) error {
	// 读取文件
	data, err := j.ReadInput(src, fsutil.IsDir(src))
	if err != nil {
		slog.Error("read file error", slog.String("file", src), slog.String("error", err.Error()))
		return fmt.Errorf("read file error: %w", err)
	}

	// 渲染内容
	content, err := renderer.Render(data)
	if err != nil {
		slog.Error("render error", slog.String("file", src), slog.String("error", err.Error()))
		return fmt.Errorf("render error: %w", err)
	}

	// 确定输出文件名
	outputFilename := j.MergeOutputFile
	if outputFilename == "" {
		if fsutil.IsDir(src) {
			outputFilename = filepath.Base(src) + ".json"
		} else {
			outputFilename = strings.TrimSuffix(filepath.Base(src), filepath.Ext(filepath.Base(src))) + ".json"
		}
	}

	// 写入文件
	if err := j.WriteOutput(content, outputFilename); err != nil {
		return fmt.Errorf("write file error: %w", err)
	}

	return nil
}
