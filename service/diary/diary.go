package diary

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/golang-module/carbon/v2"
	"github.com/spf13/cast"
	"github.com/xbpk3t/docs-alfred/pkg/errcode"
	"github.com/xbpk3t/docs-alfred/pkg/render"
)

// DiaryRenderer 实现 markdown 渲染器接口
type DiaryRenderer struct {
	srcDir    string // 源目录
	targetDir string // 目标目录
	fileName  string // 自定义文件名
	render.MarkdownRenderer
}

// NewDiaryRenderer 创建新的渲染器
func NewDiaryRenderer(srcDir, targetDir, fileName string) *DiaryRenderer {
	return &DiaryRenderer{
		srcDir:    srcDir,
		targetDir: targetDir,
		fileName:  fileName,
	}
}

// Render 实现渲染接口
func (r *DiaryRenderer) Render(data []byte) (string, error) {
	// 获取diary目录的绝对路径
	diaryDir, err := r.getAbsolutePath(r.srcDir)
	if err != nil {
		return "", err
	}

	// 获取并处理年份目录
	if err := r.processYearDirectories(diaryDir); err != nil {
		return "", err
	}

	// 如果需要，写入文件
	if err := r.writeToFileIfNeeded(); err != nil {
		return "", err
	}

	return r.String(), nil
}

// getAbsolutePath 获取绝对路径
func (r *DiaryRenderer) getAbsolutePath(dir string) (string, error) {
	if !filepath.IsAbs(dir) {
		workDir, err := os.Getwd()
		if err != nil {
			return "", errcode.WithError(errcode.ErrWorkDir, err)
		}
		return filepath.Join(workDir, dir), nil
	}
	return dir, nil
}

// processYearDirectories 处理年份目录
func (r *DiaryRenderer) processYearDirectories(diaryDir string) error {
	// 获取所有年份目录
	years, err := filepath.Glob(filepath.Join(diaryDir, "202*"))
	if err != nil {
		return errcode.WithError(errcode.ErrListDir, err)
	}

	// 按年份排序
	sort.Strings(years)

	// 处理每个年份目录
	for _, yearPath := range years {
		if err := r.processYearDirectory(yearPath); err != nil {
			return err
		}
	}

	return nil
}

// processYearDirectory 处理单个年份目录
func (r *DiaryRenderer) processYearDirectory(yearPath string) error {
	year := filepath.Base(yearPath)

	// 获取该年份下的所有yaml文件
	yamlFiles, err := filepath.Glob(filepath.Join(yearPath, "*.yml"))
	if err != nil {
		return errcode.WithError(errcode.ErrListDir, err)
	}

	// 按文件名排序
	sort.Strings(yamlFiles)

	// 处理每个yaml文件
	for _, yamlFile := range yamlFiles {
		if err := r.processYamlFile(yamlFile, year); err != nil {
			return errcode.WithError(errcode.ErrFileProcess, err)
		}
	}

	return nil
}

// processYamlFile 处理单个yaml文件
func (r *DiaryRenderer) processYamlFile(yamlFile, year string) error {
	weekNum := strings.TrimSuffix(filepath.Base(yamlFile), ".yml")
	date, err := r.calculateDate(year, weekNum)
	if err != nil {
		return err
	}

	r.renderWeekContent(date, weekNum, year)
	return nil
}

// calculateDate 计算日期
func (r *DiaryRenderer) calculateDate(year, weekNum string) (carbon.Carbon, error) {
	weekNumber := 0
	numStr, ok := strings.CutPrefix(weekNum, "w")
	if !ok {
		return carbon.Carbon{}, errcode.ErrParseWeekNumber
	}
	weekNumber = cast.ToInt(numStr)

	yearStart := carbon.CreateFromDate(cast.ToInt(year), 1, 1)
	return yearStart.AddDays((weekNumber - 1) * 7), nil
}

// renderWeekContent 渲染周内容
func (r *DiaryRenderer) renderWeekContent(date carbon.Carbon, weekNum, year string) {
	// 写入标题
	r.RenderHeader(render.HeadingLevel2, date.ToDateString()+" ("+weekNum+")")

	// 使用RenderDocusaurusRawLoader渲染导入和代码块
	r.RenderDocusaurusRawLoader(weekNum, filepath.Join("../diary", year, weekNum+".yml"))
}

// writeToFileIfNeeded 如果需要则写入文件
func (r *DiaryRenderer) writeToFileIfNeeded() error {
	if r.targetDir == "" || r.fileName == "" {
		return nil
	}

	if err := os.MkdirAll(r.targetDir, 0o755); err != nil {
		return errcode.WithError(errcode.ErrCreateDir, err)
	}

	content := r.String()
	outputPath := filepath.Join(r.targetDir, r.fileName)
	if err := os.WriteFile(outputPath, []byte(content), 0o644); err != nil {
		return errcode.WithError(errcode.ErrWriteFile, err)
	}

	return nil
}
