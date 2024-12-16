package utils

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// MergeOptions 合并选项
type MergeOptions struct {
	FolderPath string   // 配置文件所在文件夹
	FileNames  []string // 要合并的文件名列表
	OutputPath string   // 输出文件路径
}

// Merger 配置合并器
type Merger[T any] struct {
	options MergeOptions
}

// NewMerger 创建新的配置合并器
func NewMerger[T any](opts MergeOptions) *Merger[T] {
	return &Merger[T]{options: opts}
}

// Merge 执行合并操作
func (m *Merger[T]) Merge() error {
	if err := m.validateInput(); err != nil {
		return fmt.Errorf("验证输入失败: %w", err)
	}

	config, err := m.mergeConfigs()
	if err != nil {
		return fmt.Errorf("合并配置失败: %w", err)
	}

	return m.writeResult(config)
}

// validateInput 验证输入参数
func (m *Merger[T]) validateInput() error {
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

// mergeConfigs 合并所有配置文件
func (m *Merger[T]) mergeConfigs() ([]T, error) {
	var mergedConfig []T

	for _, fileName := range m.options.FileNames {
		config, err := m.processFile(fileName)
		if err != nil {
			return nil, fmt.Errorf("处理文件 %s 失败: %w", fileName, err)
		}
		mergedConfig = append(mergedConfig, config...)
	}

	return mergedConfig, nil
}

// processFile 处理单个配置文件
func (m *Merger[T]) processFile(fileName string) ([]T, error) {
	// 读取文件内容
	content, err := m.readFile(fileName)
	if err != nil {
		return nil, err
	}

	// 解析配置
	var config []T
	if err := yaml.Unmarshal(content, &config); err != nil {
		return nil, fmt.Errorf("解析YAML失败: %w", err)
	}

	// 如果类型T包含Tag字段，设置tag值
	tag := strings.TrimSuffix(fileName, ".yml")
	if err := m.setTag(config, tag); err != nil {
		return nil, fmt.Errorf("设置标签失败: %w", err)
	}

	return config, nil
}

// readFile 读取文件内容
func (m *Merger[T]) readFile(fileName string) ([]byte, error) {
	filePath := filepath.Join(m.options.FolderPath, fileName)
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("读取文件失败: %w", err)
	}
	return content, nil
}

// setTag 设置标签（如果类型支持）
func (m *Merger[T]) setTag(config []T, tag string) error {
	// 使用反射检查并设置Tag字段
	for i := range config {
		if tagger, ok := any(config[i]).(interface{ SetTag(string) }); ok {
			tagger.SetTag(tag)
		}
	}
	return nil
}

// writeResult 写入合并结果
func (m *Merger[T]) writeResult(config []T) error {
	// 创建输出目录（如果不存在）
	if err := os.MkdirAll(filepath.Dir(m.options.OutputPath), 0o755); err != nil {
		return fmt.Errorf("创建输出目录失败: %w", err)
	}

	// 创建输出文件
	file, err := os.Create(m.options.OutputPath)
	if err != nil {
		return fmt.Errorf("创建输出文件失败: %w", err)
	}
	defer file.Close()

	// 创建YAML编码器
	encoder := yaml.NewEncoder(file)
	defer encoder.Close()

	// 写入配置
	if err := encoder.Encode(config); err != nil {
		return fmt.Errorf("编码YAML失败: %w", err)
	}

	return nil
}

// MergeFiles 便捷函数，用于合并文件
func MergeFiles[T any](folderPath string, fileNames []string, outputPath string) error {
	merger := NewMerger[T](MergeOptions{
		FolderPath: folderPath,
		FileNames:  fileNames,
		OutputPath: outputPath,
	})
	return merger.Merge()
}
