package utils

import (
	"os"
	"strings"
)

// GetCSVFiles 获取指定目录下的所有CSV文件
func GetCSVFiles(dir string) []string {
	var files []string

	entries, err := os.ReadDir(dir)
	if err != nil {
		return files
	}

	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(strings.ToLower(entry.Name()), ".csv") {
			files = append(files, entry.Name())
		}
	}

	return files
}

// GetXLSXFiles 获取指定目录下的所有XLSX文件
func GetXLSXFiles(dir string) []string {
	var files []string

	entries, err := os.ReadDir(dir)
	if err != nil {
		return files
	}

	for _, entry := range entries {
		if !entry.IsDir() && (strings.HasSuffix(strings.ToLower(entry.Name()), ".xlsx") ||
			strings.HasSuffix(strings.ToLower(entry.Name()), ".xls")) {
			files = append(files, entry.Name())
		}
	}

	return files
}

// GetAllFiles 获取指定目录下的所有文件
func GetAllFiles(dir string) []string {
	var files []string

	entries, err := os.ReadDir(dir)
	if err != nil {
		return files
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			files = append(files, entry.Name())
		}
	}

	return files
}
