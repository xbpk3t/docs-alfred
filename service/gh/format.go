package gh

import (
	"fmt"
	"strings"

	"github.com/xbpk3t/docs-alfred/pkg/wf"
)

// FormatAlfredItems builds Alfred items for the given repos.
func FormatAlfredItems(repos Repos, docsURL string) []wf.AlfredItem {
	var items []wf.AlfredItem

	for _, repo := range repos {
		item := wf.AlfredItem{
			Title:    repo.FullName(),
			Subtitle: repo.GetDes(),
			Arg:      repo.GetURL(),
			Valid:    true,
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
			docURL := fmt.Sprintf("%s/#/%s", docsURL, repo.Doc)
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

// FormatPlain returns plain-text output of repos with labels.
func FormatPlain(repos Repos, docsURL string) string {
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
			docURL := fmt.Sprintf("%s/#/%s", docsURL, repo.Doc)
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
func FormatRofi(repos Repos) string {
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
