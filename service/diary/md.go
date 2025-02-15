package diary

import (
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/dromara/carbon/v2"
	"github.com/xbpk3t/docs-alfred/pkg/render"
)

// DiaryMarkdownRender 日记渲染器
type DiaryMarkdownRender struct {
	renderer render.MarkdownRenderer
}

// NewDiaryMarkdownRender 创建日记渲染器
func NewDiaryMarkdownRender() *DiaryMarkdownRender {
	return &DiaryMarkdownRender{
		renderer: render.NewMarkdownRenderer(),
	}
}

type weekFile struct {
	name     string
	filename string
	week     int
}

// Render 渲染内容
func (r *DiaryMarkdownRender) Render(data []byte) (string, error) {
	// 获取目录路径
	dirPath := string(data)
	dirPath = strings.TrimSpace(dirPath)

	year, fp := filepath.Base(dirPath), filepath.Base(filepath.Dir(dirPath))

	// 读取目录下的所有文件
	files, err := os.ReadDir(dirPath)
	if err != nil {
		return "", err
	}

	// 收集所有 yml 文件并解析周数
	var weekFiles []weekFile
	for _, file := range files {
		if file.IsDir() || filepath.Ext(file.Name()) != ".yml" {
			continue
		}
		name := strings.TrimSuffix(file.Name(), ".yml")
		weekNum := strings.TrimPrefix(name, "w")
		if week, err := strconv.Atoi(weekNum); err == nil {
			weekFiles = append(weekFiles, weekFile{
				name:     name,
				week:     week,
				filename: file.Name(),
			})
		}
	}

	// 按周数排序
	sort.Slice(weekFiles, func(i, j int) bool {
		return weekFiles[i].week < weekFiles[j].week
	})

	// 解析年份
	yearNum, err := strconv.Atoi(year)
	if err != nil {
		yearNum = carbon.Now().Year() // 如果解析失败，使用当前年份
	}

	for _, wf := range weekFiles {
		// 计算日期
		date := carbon.CreateFromDate(yearNum, 1, 1).AddWeeks(wf.week - 1)

		// 渲染标题
		r.renderer.RenderHeader(render.HeadingLevel2, date.Format("Y-m-d")+" ("+wf.name+")")

		// 渲染导入语句和代码块
		importPath := filepath.Join(fp, year, wf.filename)
		r.renderer.RenderImport(wf.name, "../"+importPath)
		r.renderer.RenderContainer("{"+wf.name+"}", "yaml")
	}

	return r.renderer.String(), nil
}
