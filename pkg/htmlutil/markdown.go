package htmlutil

import (
	"strings"

	md "github.com/JohannesKaufmann/html-to-markdown"
	"github.com/JohannesKaufmann/html-to-markdown/plugin"
)

// ToMarkdown converts HTML into GitHub-flavored Markdown.
func ToMarkdown(input string) (string, error) {
	converter := md.NewConverter("", true, nil)
	converter.Use(plugin.GitHubFlavored())

	markdown, err := converter.ConvertString(input)
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(markdown), nil
}
