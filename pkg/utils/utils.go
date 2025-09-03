package utils

import (
	"fmt"
	"os"
	"path/filepath"
)

// ReadSingleFileWithExt 读取指定扩展名的单个文件
func ReadSingleFileWithExt(src string, setCurrentFile func(string)) ([]byte, error) {
	fileInfo, err := os.Stat(src)
	if err != nil {
		return nil, fmt.Errorf("failed to stat file %s: %w", src, err)
	}

	if fileInfo.IsDir() {
		return nil, fmt.Errorf("expected file but got directory: %s", src)
	}

	if setCurrentFile != nil {
		setCurrentFile(filepath.Base(src))
	}

	data, err := os.ReadFile(src)
	if err != nil {
		return nil, fmt.Errorf("failed to read file %s: %w", src, err)
	}

	return data, nil
}

// ReadAndMergeFilesRecursively 递归读取并合并文件
func ReadAndMergeFilesRecursively(src string, setCurrentFile func(string)) ([]byte, error) {
	files, err := os.ReadDir(src)
	if err != nil {
		return nil, fmt.Errorf("read dir error: %w", err)
	}

	var mergedData []byte
	for _, file := range files {
		fullPath := filepath.Join(src, file.Name())

		if file.IsDir() {
			// 递归处理子目录
			subData, err := ReadAndMergeFilesRecursively(fullPath, setCurrentFile)
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
		if filepath.Ext(file.Name()) != ".yml" {
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
	}

	return mergedData, nil
}
