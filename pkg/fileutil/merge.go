package fileutil

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

// ReadSingleFile reads one file and optionally reports its base name.
func ReadSingleFile(src string, setCurrentFile func(string)) ([]byte, error) {
	fileInfo, err := checkPathValidity(src)
	if err != nil {
		return nil, err
	}

	return processSingleFile(src, setCurrentFile, fileInfo)
}

// ReadAndMergeYAMLFilesRecursive recursively reads YAML files and merges their contents.
func ReadAndMergeYAMLFilesRecursive(src string, setCurrentFile func(string)) ([]byte, error) {
	files, err := ListYAMLFilesRecursive(src)
	if err != nil {
		return nil, err
	}

	var mergedData []byte
	for _, path := range files {
		if setCurrentFile != nil {
			setCurrentFile(filepath.Base(path))
		}
		data, err := readFileContent(path)
		if err != nil {
			return nil, err
		}
		mergedData = append(mergedData, data...)
	}

	return mergedData, nil
}
