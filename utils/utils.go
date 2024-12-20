package utils

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/olekukonko/tablewriter"
)

func Fetch(url string) ([]byte, error) {
	resp, err := http.Get(url)
	if err != nil {
		slog.Error("request error", slog.Any("Error", err))
		return nil, err
	}
	if resp == nil {
		slog.Error("response is nil")
		return nil, errors.New("response is nil")
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

// func IsURL(str string) bool {
// 	u, err := url.ParseRequestURI(str)
// 	return err == nil && u.Scheme != "" && u.Host != ""
// }

// RenderMarkdownTable 封装了创建和渲染Markdown表格的逻辑
func RenderMarkdownTable(header []string, res *strings.Builder, data [][]string) {
	table := tablewriter.NewWriter(res)
	table.SetAutoWrapText(false)
	table.SetHeader(header)
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

// FormatDate 函数用于将 time.Time 格式化为 "2006-01-02 15:04:05" 形式的字符串
func FormatDate(t time.Time) string {
	return t.Format("2006-01-02 15:04:05")
}

func WeekNumOfYear() int {
	_, weekNum := time.Now().ISOWeek()

	return weekNum
}

// EnsureBaseURL 检查给定的相对URL是否包含基础URL，如果没有，则将其拼接
func EnsureBaseURL(baseUrl, relativeUrl string) (string, error) {
	// 解析基础URL
	base, err := url.Parse(baseUrl)
	if err != nil {
		return "", err
	}

	// 解析相对URL
	relative, err := url.Parse(relativeUrl)
	if err != nil {
		return "", err
	}

	// 如果相对URL已经包含方案（如http或https），则不需要拼接基础URL
	if relative.Scheme != "" && relative.Host != "" {
		return relativeUrl, nil
	}

	// 将相对URL拼接到基础URL
	base.ResolveReference(relative)

	// 返回拼接后的URL
	return base.String(), nil
}

// RenderMarkdownFold
func RenderMarkdownFold(summary, details string) string {
	return fmt.Sprintf("\n\n<details>\n<summary>%s</summary>\n\n%s\n\n</details>\n\n", summary, details)
}

func RenderMarkdownImageWithFigcaption(url string) string {
	// split last part of title from url
	title := ExtractTitleFromURL(url)

	return fmt.Sprintf("![image](%s)\n<center>*%s*</center>\n\n", url, title)
}

func ExtractTitleFromURL(url string) string {
	parts := strings.Split(url, "/")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	} else {
		return url
	}
}

// docusaurus admonitions const
const (
	AdmonitionTip    = "tip"
	AdmonitionInfo   = "info"
	AdmonitionWarn   = "warning"
	AdmonitionDanger = "danger"
)

func RenderMarkdownAdmonitions(admonitionType, title, rex string) string {
	var res strings.Builder

	if title == "" {
		title = strings.ToUpper(admonitionType)
	}

	res.WriteString("\n---\n")
	res.WriteString(fmt.Sprintf(":::%s[%s]\n\n", admonitionType, title))

	res.WriteString(rex)

	res.WriteString("\n\n:::\n\n")
	return res.String()
}
