package utils

import (
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/olekukonko/tablewriter"
)

func Fetch(url string) ([]byte, error) {
	resp, err := http.Get(url)
	if err != nil {
		slog.Error("request error", slog.Any("Error", err))
		return nil, err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return data, nil
}

func GetFilesOfFolder(dir, fileType string) ([]string, error) {
	var files []string
	sep := string(os.PathSeparator)

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if info.IsDir() {
			subFiles, err := GetFilesOfFolder(dir+sep+info.Name(), fileType)
			if err != nil {
				return err
			}
			files = append(files, subFiles...)
		} else {
			// 过滤指定格式的文件
			ok := strings.HasSuffix(info.Name(), fileType)
			if ok {
				files = append(files, dir+sep+info.Name())
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	// for _, fi := range dirPath {
	//
	// }
	return files, nil
}

func IsURL(str string) bool {
	u, err := url.ParseRequestURI(str)
	return err == nil && u.Scheme != "" && u.Host != ""
}

// RenderMarkdownTable 封装了创建和渲染Markdown表格的逻辑
func RenderMarkdownTable(res *strings.Builder, data [][]string) {
	table := tablewriter.NewWriter(res)
	table.SetAutoWrapText(false)
	table.SetHeader([]string{"Repo", "Des"})
	table.SetBorders(tablewriter.Border{Left: true, Top: false, Right: true, Bottom: false})
	table.SetCenterSeparator("|")
	table.AppendBulk(data) // 添加大量数据
	table.Render()
}

func JoinSlashParts(s string) string {
	index := strings.Index(s, "/")
	if index != -1 {
		// 拼接 `/` 前后的字符串，并保留 `/` 字符
		return s[:index] + s[index+1:]
	}
	return s
}

func ChangeFileExtFromYamlToMd(fp string) string {
	filename := filepath.Base(fp)
	ext := strings.ToLower(filepath.Ext(filename))
	// 检查文件扩展名是否为 .yml 或 .yaml
	if ext == ".yml" || ext == ".yaml" {
		// 截取文件名（不包含扩展名）
		base := filename[:len(filename)-len(ext)]
		// 拼接新的扩展名 .md
		return base + ".md"
	}
	// 如果不是 .yml 或 .yaml 文件，返回原文件名
	return filename
}
