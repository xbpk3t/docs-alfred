package render

import (
	"os"
)

// FileProcessor 文件处理器
type FileProcessor struct {
	InputFile  string
	OutputFile string
}

// ReadInput 读取输入文件
func (fp *FileProcessor) ReadInput() ([]byte, error) {
	return os.ReadFile(fp.InputFile)
}

// WriteOutput 写入输出文件
func (fp *FileProcessor) WriteOutput(content []byte) error {
	return os.WriteFile(fp.OutputFile, content, os.ModePerm)
}
