package presenter

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/xbpk3t/docs-alfred/pkg/wf"
	"github.com/xbpk3t/docs-alfred/service/ghindex"
)

// FormatAlfredItems builds Alfred items for the given repos.
// When query is non-empty and no repos match, a "Search GitHub" fallback item is appended.
func FormatAlfredItems(repos ghindex.Repos, docsURL, query string) []wf.AlfredItem {
	var items []wf.AlfredItem

	for _, repo := range repos {
		fullName := repo.FullName()
		item := wf.AlfredItem{
			Title:        fullName,
			Subtitle:     formatRepoSubtitle(repo),
			Arg:          repo.GetURL(),
			Autocomplete: fullName,
			QuicklookURL: repo.GetURL(),
			Text:         &wf.AlfredText{Copy: repo.GetURL(), Largetype: fullName},
			Valid:        true,
		}

		item.Icon = &wf.AlfredIcon{Path: repoIconPath(repo)}

		item.Mods = make(map[string]*wf.AlfredMod)

		// alt: copy repo URL (plist alt -> clipboard)
		item.Mods["alt"] = &wf.AlfredMod{
			Valid:    true,
			Arg:      repo.GetURL(),
			Subtitle: "复制URL: " + repo.GetURL(),
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

func formatRepoSubtitle(repo *ghindex.Repository) string {
	parts := make([]string, 0, 4)
	if repo == nil {
		return ""
	}

	if repo.IsSubRepo {
		parts = append(parts, fmt.Sprintf("[SUB#%s]", repo.MainRepo))
	}
	if repo.IsReplacedRepo {
		parts = append(parts, fmt.Sprintf("[REP#%s]", repo.MainRepo))
	}
	if repo.IsRelatedRepo {
		parts = append(parts, fmt.Sprintf("[REL#%s]", repo.MainRepo))
	}

	if repo.Type != "" {
		if repo.Tag != "" {
			parts = append(parts, fmt.Sprintf("[%s#%s]", repo.Tag, repo.Type))
		} else {
			parts = append(parts, fmt.Sprintf("[%s]", repo.Type))
		}
	}
	if repo.GetDes() != "" {
		parts = append(parts, repo.GetDes())
	}

	return strings.Join(parts, " ")
}

// FormatPlain returns plain-text output of repos with labels.
func FormatPlain(repos ghindex.Repos, docsURL string) string {
	var sb strings.Builder

	for i, repo := range repos {
		if i > 0 {
			sb.WriteString("\n\n")
		}
		fmt.Fprintf(&sb, "repo: %s\n", repo.GetURL())
		if repo.GetDes() != "" {
			fmt.Fprintf(&sb, "desc: %s\n", repo.GetDes())
		}
		if repo.Doc != "" {
			docURL := BuildDocURL(docsURL, repo.Doc)
			fmt.Fprintf(&sb, "doc: %s\n", repo.Doc)
			fmt.Fprintf(&sb, "docs: %s\n", docURL)
		}
		if repo.Type != "" {
			typeInfo := repo.Type
			if repo.Tag != "" {
				typeInfo = fmt.Sprintf("%s#%s", repo.Tag, repo.Type)
			}
			fmt.Fprintf(&sb, "type: %s\n", typeInfo)
		}
	}

	return sb.String()
}

// FormatRofi returns a simple list of "fullname - desc" lines for each repo.
func FormatRofi(repos ghindex.Repos) string {
	var lines []string
	for _, repo := range repos {
		line := repo.FullName()
		if repo.GetDes() != "" {
			line = fmt.Sprintf("%s - %s", line, repo.GetDes())
		}
		lines = append(lines, line)
	}

	return strings.Join(lines, "\n")
}
