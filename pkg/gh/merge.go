// // pkg/gh/merge.go
//
// package gh
//
// import (
// 	"fmt"
// 	"gopkg.in/yaml.v3"
// 	"os"
// 	"path/filepath"
// 	"slices"
// 	"strings"
// )
//
// var (
// 	ghFiles    []string
// 	folderName string
// )
//
// // MergeConfigFiles 合并配置文件
// func MergeConfigFiles() (ConfigRepos, error) {
// 	var cr ConfigRepos
//
// 	files, err := os.ReadDir(folderName)
// 	if err != nil {
// 		return nil, fmt.Errorf("读取目录失败: %v", err)
// 	}
//
// 	for _, file := range files {
// 		if shouldProcessFile(file) {
// 			configs, err := processConfigFile(file)
// 			if err != nil {
// 				return nil, err
// 			}
// 			cr = append(cr, configs...)
// 		}
// 	}
//
// 	return cr, nil
// }
//
// // shouldProcessFile 判断是否应该处理该文件
// func shouldProcessFile(file os.DirEntry) bool {
// 	return !file.IsDir() && slices.Contains(ghFiles, file.Name())
// }
//
// // processConfigFile 处理单个配置文件
// func processConfigFile(file os.DirEntry) (ConfigRepos, error) {
// 	filePath := filepath.Join(folderName, file.Name())
// 	content, err := os.ReadFile(filePath)
// 	if err != nil {
// 		return nil, fmt.Errorf("读取文件 %s 失败: %v", file.Name(), err)
// 	}
//
// 	tag := strings.TrimSuffix(file.Name(), ".yml")
// 	return NewConfigRepos(content).WithTag(tag), nil
// }
//
// // WriteToYAML 将数据写入YAML文件
// func WriteToYAML(data ConfigRepos, outputPath string) error {
// 	file, err := os.Create(outputPath)
// 	if err != nil {
// 		return fmt.Errorf("创建文件失败: %v", err)
// 	}
// 	defer file.Close()
//
// 	encoder := yaml.NewEncoder(file)
// 	defer encoder.Close()
//
// 	if err := encoder.Encode(data); err != nil {
// 		return fmt.Errorf("编码YAML失败: %v", err)
// 	}
// 	return nil
// }

// pkg/gh/merge.go

package gh

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// MergeOptions 定义合并配置的选项
type MergeOptions struct {
	FolderPath string   // 配置文件所在文件夹
	FileNames  []string // 要合并的文件名列表
	OutputPath string   // 输出文件路径
}

// ConfigMerger 配置合并器
type ConfigMerger struct {
	options MergeOptions
}

// NewConfigMerger 创建新的配置合并器
func NewConfigMerger(opts MergeOptions) *ConfigMerger {
	return &ConfigMerger{
		options: opts,
	}
}

// Merge 执行合并操作
func (m *ConfigMerger) Merge() error {
	// 1. 验证输入
	if err := m.validateInput(); err != nil {
		return fmt.Errorf("输入验证失败: %w", err)
	}

	// 2. 读取并合并配置
	mergedConfig, err := m.mergeConfigs()
	if err != nil {
		return fmt.Errorf("合并配置失败: %w", err)
	}

	// 3. 写入结果
	if err := m.writeResult(mergedConfig); err != nil {
		return fmt.Errorf("写入结果失败: %w", err)
	}

	return nil
}

// validateInput 验证输入参数
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

// mergeConfigs 合并所有配置文件
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

// processFile 处理单个配置文件
func (m *ConfigMerger) processFile(fileName string) (ConfigRepos, error) {
	// 1. 读取文件
	content, err := m.readFile(fileName)
	if err != nil {
		return nil, err
	}

	// 2. 解析配置
	config := m.parseConfig(content, fileName)

	return config, nil
}

// readFile 读取文件内容
func (m *ConfigMerger) readFile(fileName string) ([]byte, error) {
	filePath := filepath.Join(m.options.FolderPath, fileName)
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("读取文件失败: %w", err)
	}
	return content, nil
}

// parseConfig 解析配置内容
func (m *ConfigMerger) parseConfig(content []byte, fileName string) ConfigRepos {
	tag := strings.TrimSuffix(fileName, ".yml")
	return NewConfigRepos(content).WithTag(tag)
}

// writeResult 写入合并结果
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

// 为了向后兼容,保留简单的函数接口
func MergeConfigFiles(folderPath string, fileNames []string) (ConfigRepos, error) {
	merger := NewConfigMerger(MergeOptions{
		FolderPath: folderPath,
		FileNames:  fileNames,
		OutputPath: "gh.yml",
	})

	var mergedConfig ConfigRepos
	if err := merger.Merge(); err != nil {
		return nil, err
	}

	return mergedConfig, nil
}
