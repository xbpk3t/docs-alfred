package cmd

import (
	"fmt"

	"github.com/xbpk3t/docs-alfred/alfred/gh/internal/alfred"
	"github.com/xbpk3t/docs-alfred/alfred/gh/internal/cons"

	aw "github.com/deanishe/awgo"
	"github.com/spf13/cobra"
	"github.com/xbpk3t/docs-alfred/service/ws"
)

var wsCmd = &cobra.Command{
	Use:              "ws",
	Short:            "Searching from workspaces",
	PersistentPreRun: handlePreRun,
	Run:              handleWsCommand,
}

func handleWsCommand(cmd *cobra.Command, args []string) {
	builder := alfred.NewItemBuilder(wf)

	f, err := ws.ParseConfig(data)
	if err != nil {
		wf.NewItem("Error parsing config").
			Subtitle(err.Error()).
			Icon(aw.IconError)
		wf.SendFeedback()
		return
	}

	if f == nil {
		wf.NewItem("Invalid configuration").
			Subtitle("No workspace configuration found").
			Icon(aw.IconWarning)
		wf.SendFeedback()
		return
	}

	if len(args) > 0 {
		items := f.Search(args)
		if len(items) == 0 {
			wf.NewItem("No matching workspaces found").
				Subtitle(fmt.Sprintf("No workspaces found matching: %s", args[0])).
				Icon(aw.IconWarning)
		} else {
			renderURLItems(items, builder)
		}
	} else {
		renderURLItems(f.ExtractURLs(), builder)
	}

	wf.SendFeedback()
}

func renderURLItems(items []ws.URL, builder *alfred.ItemBuilder) {
	for _, item := range items {
		name := item.Name
		if name == "" {
			name = item.URL
		}

		wfItem := builder.BuildBasicItem(
			name,
			item.Des,
			item.URL,
			cons.IconCheck,
		)
		builder.AddCommonModifiers(wfItem, item.URL, item.Des)
	}
}
