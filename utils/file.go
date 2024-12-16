package utils

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// FileProcessor 文件处理器
type FileProcessor struct {
	InputFile  string
	OutputFile string
}

// ContentRenderer 定义渲染器接口
type ContentRenderer interface {
	Render(data []byte) (string, error)
}

// ReadInput 读取输入文件
func (fp *FileProcessor) ReadInput() ([]byte, error) {
	return os.ReadFile(fp.InputFile)
}

// WriteOutput 写入输出文件
func (fp *FileProcessor) WriteOutput(content []byte) error {
	return os.WriteFile(fp.OutputFile, content, os.ModePerm)
}

// ChangeFileExtFromYamlToMd 将yaml文件扩展名改为md
func ChangeFileExtFromYamlToMd(filename string) string {
	ext := filepath.Ext(filename)
	return strings.TrimSuffix(filename, ext) + ".md"
}

// ProcessFile 处理文件转换
func ProcessFile(inputFile string, renderer ContentRenderer) error {
	fp := &FileProcessor{
		InputFile:  inputFile,
		OutputFile: ChangeFileExtFromYamlToMd(inputFile),
	}

	// 读取文件
	data, err := fp.ReadInput()
	if err != nil {
		return fmt.Errorf("读取文件失败: %w", err)
	}

	// 渲染内容
	content, err := renderer.Render(data)
	if err != nil {
		return fmt.Errorf("渲染失败: %w", err)
	}

	// 写入文件
	if err := fp.WriteOutput([]byte(content)); err != nil {
		return fmt.Errorf("写入文件失败: %w", err)
	}

	return nil
}
