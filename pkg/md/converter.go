package md

import (
	"strings"

	html2md "github.com/JohannesKaufmann/html-to-markdown"
	"github.com/JohannesKaufmann/html-to-markdown/plugin"
)

// HTMLToMarkdown converts HTML into GitHub-flavored Markdown.
func HTMLToMarkdown(input string) (string, error) {
	converter := html2md.NewConverter("", true, nil)
	converter.Use(plugin.GitHubFlavored())

	markdown, err := converter.ConvertString(input)
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(markdown), nil
}
