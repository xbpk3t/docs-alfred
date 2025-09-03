package merger

import (
	"os"
	"path/filepath"

	yaml "github.com/goccy/go-yaml"
	"github.com/xbpk3t/docs-alfred/pkg/errcode"
)

// Merger 配置合并器
type Merger struct {
	tagProvider func(string) string
	outputFile  string
	outputDir   string
	inputFiles  []string
}

// NewMerger 创建新的合并器
func NewMerger(inputFiles []string, outputFile, outputDir string, tagProvider func(string) string) *Merger {
	return &Merger{
		inputFiles:  inputFiles,
		outputFile:  outputFile,
		outputDir:   outputDir,
		tagProvider: tagProvider,
	}
}

// Merge 执行合并
func (m *Merger) Merge() error {
	if err := m.validateInput(); err != nil {
		return errcode.WithError(errcode.ErrValidateInput, err)
	}

	result, err := m.mergeConfigs()
	if err != nil {
		return errcode.WithError(errcode.ErrMergeConfig, err)
	}

	return m.writeResult(result)
}

// validateInput 验证输入
func (m *Merger) validateInput() error {
	if len(m.inputFiles) == 0 {
		return errcode.ErrInvalidInput
	}

	return nil
}

// mergeConfigs 合并配置
func (m *Merger) mergeConfigs() (any, error) {
	var result any
	for _, file := range m.inputFiles {
		config, err := m.processFile(file)
		if err != nil {
			return nil, errcode.WithError(errcode.ErrFileProcess, err)
		}
		result = m.merge(result, config)
	}

	return result, nil
}

// processFile 处理单个文件
func (m *Merger) processFile(fileName string) (any, error) {
	data, err := os.ReadFile(fileName)
	if err != nil {
		return nil, errcode.WithError(errcode.ErrReadFile, err)
	}

	var config any
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, errcode.WithError(errcode.ErrParseYAML, err)
	}

	if m.tagProvider != nil {
		if err := m.setTag(config, m.tagProvider(fileName)); err != nil {
			return nil, errcode.WithError(errcode.ErrSetTag, err)
		}
	}

	return config, nil
}

// writeResult 写入结果
func (m *Merger) writeResult(result any) error {
	if m.outputDir != "" {
		if err := os.MkdirAll(m.outputDir, 0o750); err != nil {
			return errcode.WithError(errcode.ErrCreateDir, err)
		}
	}

	file, err := os.Create(filepath.Join(m.outputDir, m.outputFile))
	if err != nil {
		return errcode.WithError(errcode.ErrCreateFile, err)
	}
	defer func() {
		// 优先捕获文件关闭错误（如磁盘写入失败）
		if cerr := file.Close(); err == nil && cerr != nil {
			err = errcode.WithError(errcode.ErrCloseFile, cerr)
		}
	}()

	encoder := yaml.NewEncoder(file)
	// 显式关闭编码器，确保缓冲区刷新
	if err = encoder.Encode(result); err != nil {
		_ = file.Close() // 立即关闭文件（避免defer覆盖错误）

		return errcode.WithError(errcode.ErrEncodeYAML, err)
	}

	if err := encoder.Encode(result); err != nil {
		return errcode.WithError(errcode.ErrEncodeYAML, err)
	}

	return nil
}

// merge 合并两个配置
func (m *Merger) merge(a, b any) any {
	if a == nil {
		return b
	}
	// 实现合并逻辑
	return b
}

// setTag 设置标签
func (m *Merger) setTag(_ any, _ string) error {
	// 实现设置标签逻辑
	return nil
}
