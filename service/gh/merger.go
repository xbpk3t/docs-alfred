package gh

import (
	"os"
	"path/filepath"

	"github.com/xbpk3t/docs-alfred/pkg/errcode"
	"gopkg.in/yaml.v3"
)

// Merger 配置合并器
type Merger struct {
	inputFiles  []string
	outputFile  string
	outputDir   string
	tagProvider func(string) string
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
func (m *Merger) mergeConfigs() (interface{}, error) {
	var result interface{}
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
func (m *Merger) processFile(fileName string) (interface{}, error) {
	data, err := os.ReadFile(fileName)
	if err != nil {
		return nil, errcode.WithError(errcode.ErrReadFile, err)
	}

	var config interface{}
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
func (m *Merger) writeResult(result interface{}) error {
	if m.outputDir != "" {
		if err := os.MkdirAll(m.outputDir, 0755); err != nil {
			return errcode.WithError(errcode.ErrCreateDir, err)
		}
	}

	file, err := os.Create(filepath.Join(m.outputDir, m.outputFile))
	if err != nil {
		return errcode.WithError(errcode.ErrCreateFile, err)
	}
	defer file.Close()

	encoder := yaml.NewEncoder(file)
	defer encoder.Close()

	if err := encoder.Encode(result); err != nil {
		return errcode.WithError(errcode.ErrEncodeYAML, err)
	}

	return nil
}

// merge 合并两个配置
func (m *Merger) merge(a, b interface{}) interface{} {
	if a == nil {
		return b
	}
	// 实现合并逻辑
	return b
}

// setTag 设置标签
func (m *Merger) setTag(config interface{}, tag string) error {
	// 实现设置标签逻辑
	return nil
}
