package render

import (
	"os"
	"path/filepath"

	"github.com/xbpk3t/docs-alfred/pkg/errcode"
)

// FileProcessor 文件处理器
type FileProcessor struct {
	InputDir   string // 输入目录
	OutputDir  string // 输出目录
	InputFile  string // 输入文件（单文件模式）
	OutputFile string // 输出文件（单文件模式）
	IsMerge    bool   // 是否合并模式
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
	inputPath := fp.InputFile
	if fp.InputDir != "" {
		inputPath = filepath.Join(fp.InputDir, fp.InputFile)
	}
	data, err := os.ReadFile(inputPath)
	if err != nil {
		return nil, errcode.WithError(errcode.ErrReadFile, err)
	}
	return data, nil
}

// readAndMergeFiles 读取并合并文件
func (fp *FileProcessor) readAndMergeFiles() ([]byte, error) {
	files, err := os.ReadDir(fp.InputDir)
	if err != nil {
		return nil, errcode.WithError(errcode.ErrListDir, err)
	}

	var mergedData []byte
	for _, file := range files {
		if file.IsDir() || filepath.Ext(file.Name()) != ".yml" {
			continue
		}

		data, err := os.ReadFile(filepath.Join(fp.InputDir, file.Name()))
		if err != nil {
			return nil, errcode.WithError(errcode.ErrReadFile, err)
		}
		mergedData = append(mergedData, data...)
		mergedData = append(mergedData, '\n')
	}

	return mergedData, nil
}

// WriteOutput 写入输出
func (fp *FileProcessor) WriteOutput(content []byte) error {
	// 确保输出目录存在
	if fp.OutputDir != "" {
		if err := os.MkdirAll(fp.OutputDir, 0o755); err != nil {
			return errcode.WithError(errcode.ErrCreateDir, err)
		}
	}

	// 确定输出文件路径
	outputPath := fp.OutputFile
	if fp.OutputDir != "" {
		if outputPath == "" {
			// 如果没有指定输出文件名，使用输入目录名
			outputPath = filepath.Base(fp.InputDir) + ".md"
		}
		outputPath = filepath.Join(fp.OutputDir, outputPath)
	}

	// 创建临时目录
	tmpDir := filepath.Join(fp.OutputDir, filepath.Base(fp.InputDir))
	if err := os.MkdirAll(tmpDir, 0o755); err != nil {
		return errcode.WithError(errcode.ErrCreateDir, err)
	}

	// 写入临时文件
	tmpFile := filepath.Join(tmpDir, filepath.Base(outputPath))
	if err := os.WriteFile(tmpFile, content, 0o644); err != nil {
		return errcode.WithError(errcode.ErrWriteFile, err)
	}

	// 移动到最终位置
	if err := os.Rename(tmpFile, outputPath); err != nil {
		return errcode.WithError(errcode.ErrFileProcess, err)
	}

	// 清理临时目录
	if err := os.RemoveAll(tmpDir); err != nil {
		return errcode.WithError(errcode.ErrFileProcess, err)
	}

	return nil
}
