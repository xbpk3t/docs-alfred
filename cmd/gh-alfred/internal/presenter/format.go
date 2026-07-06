package presenter

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/xbpk3t/docs-alfred/internal/gh/content"
	"github.com/xbpk3t/docs-alfred/internal/gh/index"
	"github.com/xbpk3t/docs-alfred/pkg/wf"
)

// FormatAlfredItems builds Alfred items for the given repos.
// When query is non-empty and no repos match, a "Search GitHub" fallback item is appended.
func FormatAlfredItems(repos ghindex.Repos, docsURL, query string) []wf.AlfredItem {
	items := make([]wf.AlfredItem, 0)

	for _, repo := range repos {
		fullName := ghindex.FullName(repo)
		item := wf.AlfredItem{
			Title:        fullName,
			Subtitle:     formatRepoSubtitle(repo),
			Arg:          ghindex.GetURL(repo),
			Autocomplete: fullName,
			QuicklookURL: ghindex.GetURL(repo),
			Text:         &wf.AlfredText{Copy: ghindex.GetURL(repo), Largetype: fullName},
			Valid:        true,
		}

		item.Icon = &wf.AlfredIcon{Path: repoIconPath(repo)}

		item.Mods = make(map[string]*wf.AlfredMod)

		// alt: copy repo URL (plist alt -> clipboard)
		item.Mods["alt"] = &wf.AlfredMod{
			Valid:    true,
			Arg:      ghindex.GetURL(repo),
			Subtitle: "复制URL: " + ghindex.GetURL(repo),
		}

		if repo.Doc != "" {
			docURL := BuildDocURL(docsURL, repo.Doc)
			item.Mods["cmd"] = &wf.AlfredMod{
				Valid:    true,
				Arg:      docURL,
				Subtitle: "打开文档: " + docURL,
			}
			item.Mods["shift"] = &wf.AlfredMod{
				Valid:    true,
				Arg:      docURL,
				Subtitle: "打开文档: " + docURL,
			}
		}

		if ghindex.HasNix(repo) {
			item.Mods["ctrl"] = &wf.AlfredMod{
				Valid:    true,
				Arg:      repo.NixURL,
				Subtitle: "nixpkgs: " + repo.NixURL,
			}
		}

		items = append(items, item)
	}

	items = appendGitHubSearchFallback(items, query)

	return items
}

// BuildDocURL resolves either an absolute documentation URL or a docs route path.
func BuildDocURL(docsURL, doc string) string {
	if doc == "" {
		return ""
	}
	if strings.HasPrefix(doc, "http://") || strings.HasPrefix(doc, "https://") {
		return doc
	}
	if docsURL == "" {
		docsURL = "https://docs.lucc.dev"
	}
	if base, _, found := strings.Cut(docsURL, "#"); found {
		docsURL = base
	}

	return fmt.Sprintf("%s/#/%s", strings.TrimRight(docsURL, "/"), strings.TrimLeft(doc, "/"))
}

func appendGitHubSearchFallback(items []wf.AlfredItem, query string) []wf.AlfredItem {
	if query == "" {
		return items
	}
	searchURL := fmt.Sprintf(GithubSearchURL, url.QueryEscape(query))

	return append(items, wf.AlfredItem{
		Title:    "Search GitHub: " + query,
		Subtitle: searchURL,
		Arg:      searchURL,
		Valid:    true,
		Icon:     &wf.AlfredIcon{Path: IconSearch},
	})
}

func formatRepoSubtitle(repo *content.Repo) string {
	parts := make([]string, 0, 4)
	if repo == nil {
		return ""
	}

	if repo.IsRelatedRepo {
		parts = append(parts, fmt.Sprintf("[REL#%s]", repo.MainRepo))
	}

	if repo.Type != "" {
		parts = append(parts, formatTypeLabel(repo.Tag, repo.Type, repo.TopicName))
	}
	if ghindex.GetDes(repo) != "" {
		parts = append(parts, ghindex.GetDes(repo))
	}

	return strings.Join(parts, " ")
}

// formatTypeLabel formats the type label with brackets for subtitle display.
func formatTypeLabel(tag, typeName, topicName string) string {
	if tag != "" {
		if topicName != "" {
			return fmt.Sprintf("[%s#%s#%s]", tag, typeName, topicName)
		}
		return fmt.Sprintf("[%s#%s]", tag, typeName)
	}
	return fmt.Sprintf("[%s]", typeName)
}

// formatTypeInfo formats the type info without brackets for plain/rofi display.
func formatTypeInfo(tag, typeName, topicName string) string {
	if tag != "" {
		if topicName != "" {
			return fmt.Sprintf("%s#%s#%s", tag, typeName, topicName)
		}
		return fmt.Sprintf("%s#%s", tag, typeName)
	}
	return typeName
}

// FormatPlain returns plain-text output of repos with labels.
func FormatPlain(repos ghindex.Repos, docsURL string) string {
	var sb strings.Builder

	for i, repo := range repos {
		if i > 0 {
			sb.WriteString("\n\n")
		}
		fmt.Fprintf(&sb, "repo: %s\n", ghindex.GetURL(repo))
		if ghindex.GetDes(repo) != "" {
			fmt.Fprintf(&sb, "desc: %s\n", ghindex.GetDes(repo))
		}
		if repo.Doc != "" {
			docURL := BuildDocURL(docsURL, repo.Doc)
			fmt.Fprintf(&sb, "doc: %s\n", repo.Doc)
			fmt.Fprintf(&sb, "docs: %s\n", docURL)
		}
		if repo.Type != "" {
			fmt.Fprintf(&sb, "type: %s\n", formatTypeInfo(repo.Tag, repo.Type, repo.TopicName))
		}
	}

	return sb.String()
}

// FormatRofi returns a simple list of "fullname - desc" lines for each repo.
func FormatRofi(repos ghindex.Repos) string {
	var lines []string
	for _, repo := range repos {
		line := ghindex.FullName(repo)
		if repo.TopicName != "" {
			line = fmt.Sprintf("%s [%s#%s#%s]", line, repo.Tag, repo.Type, repo.TopicName)
		}
		if ghindex.GetDes(repo) != "" {
			line = fmt.Sprintf("%s - %s", line, ghindex.GetDes(repo))
		}
		lines = append(lines, line)
	}

	return strings.Join(lines, "\n")
}
