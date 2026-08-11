package cmd

import (
	"fmt"
	"log/slog"

	"github.com/spf13/cobra"
	data "github.com/xbpk3t/docs-alfred/internal/gh/domrules"
	goods "github.com/xbpk3t/docs-alfred/internal/gh/goods"
	"github.com/xbpk3t/docs-alfred/pkg/output"
)

func newGoodsCmd(dataPath *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "goods",
		Short: "Goods data commands",
	}

	cmd.AddCommand(newUsingCmd(dataPath))

	return cmd
}

func newUsingCmd(dataPath *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "using",
		Short: "Extract goods items in use (isUsing: true)",
		Long: `Load goods data from the goods domain and output items with isUsing: true.

Output is grouped by tag → type → topic, mirroring the dump command shape:
	[
	  {
	    "tag": "goods",
	    "types": [
	      {
	        "type": "耐用品",
	        "topics": [
	          {
	            "topic": "收纳袋",
	            "items": [
	              { "name": "...", "brand": "...", "price": "...", ... }
	            ]
	          }
	        ]
	      }
	    ]
	  }
	]`,
		RunE: func(cmd *cobra.Command, args []string) error {
			spec, ok := data.SpecForDomain(data.DomainGoods)
			if !ok {
				return fmt.Errorf("unknown data domain %q", data.DomainGoods)
			}
			path := *dataPath
			if path == "" {
				path = spec.DefaultPath
			}

			slog.Info("Extracting goods in use", "domain", data.DomainGoods, "path", path)

			result, err := goods.ExtractUsing(path)
			if err != nil {
				return fmt.Errorf("extract goods using: %w", err)
			}

			return output.WriteJSON(result)
		},
	}

	return cmd
}
