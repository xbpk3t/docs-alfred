package render

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/xbpk3t/docs-alfred/pkg/errcode"
)

// FileProcessor 文件处理器
type FileProcessor struct {
	SrcDir     string   // 输入目录
	TargetDir  string   // 输出目录
	InputFile  string   // 输入文件名
	OutputFile string   // 输出文件名
	TargetFile string   // 目标文件名（用于合并模式）
	Exclude    []string // 排除的文件
	IsMerge    bool     // 是否合并模式
}

// GetInputFilePath 获取输入文件完整路径
func (p *FileProcessor) GetInputFilePath() string {
	return filepath.Join(p.SrcDir, p.InputFile)
}

// GetOutputFilePath 获取输出文件完整路径
func (p *FileProcessor) GetOutputFilePath() string {
	if p.TargetFile != "" {
		return filepath.Join(p.TargetDir, p.TargetFile)
	}
	return filepath.Join(p.TargetDir, p.OutputFile)
}

// GetCurrentFileName 获取当前处理的文件名
func (p *FileProcessor) GetCurrentFileName() string {
	if p.InputFile != "" {
		return p.InputFile
	}
	return p.TargetFile
}

// ReadInput 读取输入
func (fp *FileProcessor) ReadInput() ([]byte, error) {
	if fp.IsMerge {
		return fp.readAndMergeFiles()
	}
	return fp.readSingleFile()
}

// readSingleFile 读取单个文件
func (fp *FileProcessor) readSingleFile() ([]byte, error) {
	if fp.InputFile == "" {
		// 如果没有指定输入文件，但指定了输入目录，读取目录下的第一个 yml 文件
		if fp.SrcDir != "" {
			files, err := os.ReadDir(fp.SrcDir)
			if err != nil {
				return nil, errcode.WithError(errcode.ErrListDir, err)
			}

			for _, file := range files {
				if file.IsDir() || filepath.Ext(file.Name()) != ".yml" {
					continue
				}
				fp.InputFile = file.Name()
				break
			}
		}

		if fp.InputFile == "" {
			return nil, errcode.WithError(errcode.ErrReadFile, fmt.Errorf("no input file specified"))
		}
	}

	inputPath := fp.InputFile
	if fp.SrcDir != "" {
		inputPath = filepath.Join(fp.SrcDir, fp.InputFile)
	}
	data, err := os.ReadFile(inputPath)
	if err != nil {
		return nil, errcode.WithError(errcode.ErrReadFile, err)
	}
	return data, nil
}

// readAndMergeFiles 读取并合并文件
func (fp *FileProcessor) readAndMergeFiles() ([]byte, error) {
	files, err := os.ReadDir(fp.SrcDir)
	if err != nil {
		return nil, errcode.WithError(errcode.ErrListDir, err)
	}

	var mergedData []byte
	for _, file := range files {
		if file.IsDir() || filepath.Ext(file.Name()) != ".yml" {
			continue
		}

		data, err := os.ReadFile(filepath.Join(fp.SrcDir, file.Name()))
		if err != nil {
			return nil, errcode.WithError(errcode.ErrReadFile, err)
		}
		mergedData = append(mergedData, data...)
		mergedData = append(mergedData, '\n')
	}

	return mergedData, nil
}

// WriteOutput 写入输出
func (fp *FileProcessor) WriteOutput(content string) error {
	// 确保输出目录存在
	if err := os.MkdirAll(fp.TargetDir, 0o755); err != nil {
		return errcode.WithError(errcode.ErrCreateDir, err)
	}

	// 确定输出文件路径
	outputPath := fp.OutputFile
	if outputPath == "" {
		// 如果没有指定输出文件名，使用输入文件名
		if fp.InputFile != "" {
			outputPath = strings.TrimSuffix(fp.InputFile, ".yml") + ".md"
		} else {
			// 如果没有输入文件名，使用目录名
			outputPath = filepath.Base(fp.SrcDir) + ".md"
		}
	}
	outputPath = filepath.Join(fp.TargetDir, outputPath)

	// 直接写入文件
	if err := os.WriteFile(outputPath, []byte(content), 0o644); err != nil {
		return errcode.WithError(errcode.ErrWriteFile, err)
	}

	return nil
}

// ProcessFile 核心方法：处理单个文件
func ProcessFile(fp *FileProcessor, renderer Renderer) error {
	// 读取文件
	data, err := fp.ReadInput()
	if err != nil {
		return errcode.WithError(errcode.ErrReadFile, err)
	}

	// 渲染内容
	content, err := renderer.Render(data)
	if err != nil {
		return errcode.WithError(errcode.ErrRender, err)
	}

	// 写入文件
	if err := fp.WriteOutput(content); err != nil {
		return errcode.WithError(errcode.ErrWriteFile, err)
	}

	return nil
}

// ChangeFileExtFromYamlToMd 将文件扩展名从 .yml/.yaml 改为 .md
func ChangeFileExtFromYamlToMd(filename string) string {
	ext := filepath.Ext(filename)
	if ext == ".yml" || ext == ".yaml" {
		return strings.TrimSuffix(filename, ext) + ".md"
	}
	return filename + ".md"
}
