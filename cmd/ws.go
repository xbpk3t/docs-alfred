package cmd

import (
	"fmt"

	aw "github.com/deanishe/awgo"
	"github.com/spf13/cobra"
	"github.com/xbpk3t/docs-alfred/pkg/ws"
)

var wsCmd = &cobra.Command{
	Use:   "ws",
	Short: "A brief description of your command",
	Run: func(cmd *cobra.Command, args []string) {
		ws, err := ws.NewConfigWs(data)
		if err != nil {
			wf.FatalError(err)
		}
		tks := ws.SearchWs(args)

		for _, ws := range tks {
			item := wf.NewItem(ws.Name).
				Title(ws.Name).
				Subtitle(ws.Des).
				Valid(true).
				Quicklook(ws.URL).
				Autocomplete(ws.Name).
				Arg(ws.URL).
				Icon(&aw.Icon{Value: "icons/check.svg"})

			item.Cmd().Subtitle(fmt.Sprintf("Quicklook: %s", ws.URL)).Arg(ws.Des)
			item.Opt().Subtitle(fmt.Sprintf("Copy URL: %s", ws.URL)).Arg(ws.URL)
		}

		wf.SendFeedback()
	},
}
