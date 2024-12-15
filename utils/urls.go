package utils

import (
	"fmt"
	"net/url"
	"path"
	"strings"
)

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
