package presenter

import (
	"fmt"
	"strings"

	"github.com/xbpk3t/docs-alfred/pkg/wf"
	"github.com/xbpk3t/docs-alfred/service/ghindex"
)

// FormatAlfredItems builds Alfred items for the given repos.
func FormatAlfredItems(repos ghindex.Repos, docsURL string) []wf.AlfredItem {
	var items []wf.AlfredItem

	for _, repo := range repos {
		fullName := repo.FullName()
		item := wf.AlfredItem{
			Title:        fullName,
			Subtitle:     formatRepoSubtitle(repo),
			Arg:          repo.GetURL(),
			Autocomplete: fullName,
			Text:         &wf.AlfredText{Copy: repo.GetURL(), Largetype: fullName},
			Valid:        true,
		}

		switch {
		case repo.HasQs() && repo.Doc != "":
			item.Icon = &wf.AlfredIcon{Path: IconQsDoc}
		case repo.HasQs():
			item.Icon = &wf.AlfredIcon{Path: IconQs}
		case repo.Doc != "":
			item.Icon = &wf.AlfredIcon{Path: IconDoc}
		default:
			item.Icon = &wf.AlfredIcon{Path: IconSearch}
		}

		item.Mods = make(map[string]*wf.AlfredMod)
		if repo.Doc != "" {
			docURL := BuildDocURL(docsURL, repo.Doc)
			item.Mods["cmd"] = &wf.AlfredMod{
				Valid:    true,
				Arg:      docURL,
				Subtitle: "Open documentation",
			}
		}

		items = append(items, item)
	}

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

func formatRepoSubtitle(repo *ghindex.Repository) string {
	parts := make([]string, 0, 2)
	if repo != nil && repo.Type != "" {
		if repo.Tag != "" {
			parts = append(parts, fmt.Sprintf("[%s#%s]", repo.Tag, repo.Type))
		} else {
			parts = append(parts, fmt.Sprintf("[%s]", repo.Type))
		}
	}
	if repo != nil && repo.GetDes() != "" {
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
