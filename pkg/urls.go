package pkg

import "strings"

// URLInfo URL信息结构
type URLInfo struct {
	Name string
	URL  string
	Feed string
	Des  string
}

// GetDisplayName 获取显示名称
func (u *URLInfo) GetDisplayName() string {
	if u.Name != "" {
		return u.Name
	}

	return u.URL
}

// GetLink 获取链接地址
func (u *URLInfo) GetLink() string {
	if u.URL != "" {
		return u.URL
	}

	return u.Feed
}

func JoinSlashParts(s string) string {
	index := strings.Index(s, "/")
	if index != -1 {
		// 拼接 `/` 前后的字符串，并保留 `/` 字符
		return s[:index] + s[index+1:]
	}

	return s
}
