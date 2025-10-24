package pkg

import (
	"fmt"
	"os"
	"path/filepath"
)

// checkPathValidity 检查路径有效性.
func checkPathValidity(src string) (os.FileInfo, error) {
	fileInfo, err := os.Stat(src)
	if err != nil {
		return nil, fmt.Errorf("failed to stat file %s: %w", src, err)
	}

	return fileInfo, nil
}

// readFileContent 读取文件内容.
func readFileContent(fullPath string) ([]byte, error) {
	data, err := os.ReadFile(fullPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file %s: %w", fullPath, err)
	}

	return data, nil
}

// processSingleFile 处理单个文件.
func processSingleFile(src string, setCurrentFile func(string), fileInfo os.FileInfo) ([]byte, error) {
	if fileInfo.IsDir() {
		return nil, fmt.Errorf("expected file but got directory: %s", src)
	}

	if setCurrentFile != nil {
		setCurrentFile(filepath.Base(src))
	}

	data, err := readFileContent(src)
	if err != nil {
		return nil, err
	}

	return data, nil
}

// ReadSingleFileWithExt 读取指定扩展名的单个文件.
func ReadSingleFileWithExt(src string, setCurrentFile func(string)) ([]byte, error) {
	fileInfo, err := checkPathValidity(src)
	if err != nil {
		return nil, err
	}

	return processSingleFile(src, setCurrentFile, fileInfo)
}

// readDirectory 读取目录内容.
func readDirectory(src string) ([]os.DirEntry, error) {
	files, err := os.ReadDir(src)
	if err != nil {
		return nil, fmt.Errorf("read dir error: %w", err)
	}

	return files, nil
}

// processDirectoryEntry 处理目录条目.
func processDirectoryEntry(src string, file os.DirEntry, setCurrentFile func(string)) ([]byte, error) {
	fullPath := filepath.Join(src, file.Name())

	if file.IsDir() {
		// 递归处理子目录
		subData, err := ReadAndMergeFilesRecursively(fullPath, setCurrentFile)
		if err != nil {
			return nil, err
		}

		return subData, nil
	}

	// 跳过非 yml/yaml 文件和被排除的文件
	if ext := filepath.Ext(file.Name()); ext != ".yml" && ext != ".yaml" {
		return nil, nil // 返回nil表示跳过该文件
	}

	// 设置当前处理的文件名
	if setCurrentFile != nil {
		setCurrentFile(file.Name())
	}

	// 读取文件内容
	data, err := readFileContent(fullPath)
	if err != nil {
		return nil, fmt.Errorf("read file error: %w", err)
	}

	return data, nil
}

// appendDirectoryData 追加目录数据.
func appendDirectoryData(mergedData, subData []byte) []byte {
	if len(subData) > 0 {
		mergedData = append(mergedData, subData...)
		mergedData = append(mergedData, '\n')
	}

	return mergedData
}

// appendFileData 追加文件数据.
func appendFileData(mergedData, data []byte) []byte {
	return append(mergedData, data...)
}

// ReadAndMergeFilesRecursively 递归读取并合并文件.
func ReadAndMergeFilesRecursively(src string, setCurrentFile func(string)) ([]byte, error) {
	files, err := readDirectory(src)
	if err != nil {
		return nil, err
	}

	var mergedData []byte
	for _, file := range files {
		data, err := processDirectoryEntry(src, file, setCurrentFile)
		if err != nil {
			return nil, err
		}

		// 如果数据为nil，表示跳过该文件
		if data == nil {
			continue
		}

		// 处理不同类型的条目
		if file.IsDir() {
			mergedData = appendDirectoryData(mergedData, data)
		} else {
			mergedData = appendFileData(mergedData, data)
		}
	}

	return mergedData, nil
}
