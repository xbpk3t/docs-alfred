package gh

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/xbpk3t/docs-alfred/pkg/parser"

	"github.com/xbpk3t/docs-alfred/pkg/errcode"
	"gopkg.in/yaml.v3"
)

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
	rc, err := parser.NewParser[ConfigRepos](content).ParseSingle()
	if err != nil {
		return nil, errcode.ErrParseConfig
	}

	return rc.WithTag(tag), nil
}

func (m *ConfigMerger) readFile(fileName string) ([]byte, error) {
	filePath := filepath.Join(m.options.FolderPath, fileName)
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, errcode.WithError(errcode.ErrReadFile, err)
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
