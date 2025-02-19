package utils

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
)

// ReadAndMergeFilesRecursively 递归读取并合并文件
func ReadAndMergeFilesRecursively(src string, exclude []string, setCurrentFile func(string)) ([]byte, error) {
	files, err := os.ReadDir(src)
	if err != nil {
		return nil, fmt.Errorf("read dir error: %w", err)
	}

	var mergedData []byte
	for _, file := range files {
		fullPath := filepath.Join(src, file.Name())

		if file.IsDir() {
			// 递归处理子目录
			subData, err := ReadAndMergeFilesRecursively(fullPath, exclude, setCurrentFile)
			if err != nil {
				return nil, err
			}
			if len(subData) > 0 {
				mergedData = append(mergedData, subData...)
				mergedData = append(mergedData, '\n')
			}
			continue
		}

		// 跳过非 yml 文件和被排除的文件
		if filepath.Ext(file.Name()) != ".yml" || slices.Contains(exclude, file.Name()) {
			continue
		}

		// 设置当前处理的文件名
		if setCurrentFile != nil {
			setCurrentFile(file.Name())
		}

		// 读取文件内容
		data, err := os.ReadFile(fullPath)
		if err != nil {
			return nil, fmt.Errorf("read file error: %w", err)
		}

		mergedData = append(mergedData, data...)
		mergedData = append(mergedData, '\n')
	}

	return mergedData, nil
}
