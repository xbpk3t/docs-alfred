package ws

import (
	"fmt"
	"strings"

	"github.com/xbpk3t/docs-alfred/pkg"
)

// URL 定义单个URL结构
type URL struct {
	Name string `yaml:"name"`
	URL  string `yaml:"url"`
	Des  string `yaml:"des,omitempty"`
	Feed string `yaml:"feed"`
}

// WebStack 定义网站栈结构
type WebStack struct {
	Type string `yaml:"type"`
	URLs []URL  `yaml:"urls"`
}

// WebStacks WebStack集合
type WebStacks []WebStack

// WebStackRenderer Markdown渲染器
type WebStackRenderer struct {
	pkg.MarkdownRenderer
}

// NewWebStackRenderer 创建新的渲染器
func NewWebStackRenderer() *WebStackRenderer {
	return &WebStackRenderer{}
}

// ParseConfig 解析配置文件
func ParseConfig(data []byte) (WebStacks, error) {
	return pkg.Parse[WebStack](data)
}

// Render 渲染为Markdown格式
func (r *WebStackRenderer) Render(data []byte) (string, error) {
	ws, err := ParseConfig(data)
	if err != nil {
		return "", err
	}

	for _, stack := range ws {
		r.RenderHeader(2, stack.Type)
		r.renderURLs(stack.URLs)
	}

	return r.String(), nil
}

// renderURLs 渲染URL列表
func (r *WebStackRenderer) renderURLs(urls []URL) {
	for _, url := range urls {
		name := url.getName()
		link := url.getLink()
		r.RenderListItem(fmt.Sprintf("%s %s", r.RenderLink(name, link), url.Des))
	}
}

// getName 获取显示名称
func (u *URL) getName() string {
	if u.Name != "" {
		return u.Name
	}
	return u.URL
}

// getLink 获取链接地址
func (u *URL) getLink() string {
	if u.URL != "" {
		return u.URL
	}
	return u.Feed
}

// ExtractURLs 提取所有URL
func (ws WebStacks) ExtractURLs() []URL {
	var urls []URL
	for _, stack := range ws {
		urls = append(urls, stack.URLs...)
	}
	return urls
}

// ExtractURLsWithType 提取带类型标记的URL
func (ws WebStacks) ExtractURLsWithType() []URL {
	var urls []URL
	for _, stack := range ws {
		for _, url := range stack.URLs {
			url.Des = fmt.Sprintf("[#%s] %s %s", stack.Type, url.Des, url.URL)
			urls = append(urls, url)
		}
	}
	return urls
}

// Search 搜索URL
func (ws WebStacks) Search(keywords []string) []URL {
	if len(keywords) == 0 {
		return ws.ExtractURLsWithType()
	}

	urls := ws.ExtractURLsWithType()
	for _, keyword := range keywords {
		keyword = strings.ToLower(keyword)
		urls = filterURLs(urls, keyword)
	}
	return urls
}

// filterURLs 过滤URL
func filterURLs(urls []URL, keyword string) []URL {
	var filtered []URL
	for _, url := range urls {
		if matchesKeyword(url, keyword) {
			filtered = append(filtered, url)
		}
	}
	return filtered
}

// matchesKeyword 检查URL是否匹配关键词
func matchesKeyword(url URL, keyword string) bool {
	name := strings.ToLower(url.Name)
	des := strings.ToLower(url.Des)
	return strings.Contains(name, keyword) || strings.Contains(des, keyword)
}
