package pkg

import (
	"fmt"
	"net/url"
	"path"
	"strings"
)

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

func GetFileName(urlString string) (string, error) {
	parsedURL, err := url.Parse(urlString)
	if err != nil {
		return "", fmt.Errorf("error parsing URL: %v", err)
	}
	return path.Base(parsedURL.Path), nil
}

func BuildDocsURL(parts ...string) string {
	return strings.Join(parts, "/")
}
