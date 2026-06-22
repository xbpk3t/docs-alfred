package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/samber/mo"
	"github.com/spf13/cobra"
	"github.com/xbpk3t/docs-alfred/ccx/internal"
)

func newSessionChainCmd() *cobra.Command {
	var flags struct {
		session    string
		jsonOutput bool
		rawOutput  bool
	}

	cmd := &cobra.Command{
		Use:   "chain",
		Short: "Walk session chain",
		Long:  `Walk the Claude Code session chain by following parentUuid links.`,
		RunE: func(_ *cobra.Command, _ []string) error {
			chain, err := internal.WalkSessionChain(flags.session)
			if err != nil {
				return fmt.Errorf("walk session chain: %w", err)
			}

			if flags.jsonOutput {
				return writeJSON(chain)
			}

			if flags.rawOutput {
				return writeRaw(chain)
			}

			return writeHumanReadable(chain)
		},
	}

	cmd.Flags().BoolVar(&flags.jsonOutput, "json", false, "Output as JSON")
	cmd.Flags().BoolVar(&flags.rawOutput, "raw", false, "Output as tab-separated values")
	cmd.Flags().StringVar(&flags.session, "session", "", "Session ID (overrides CLAUDE_CODE_SESSION_ID)")

	return cmd
}

func writeJSON(chain []internal.ChainRecord) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")

	return encoder.Encode(chain)
}

func writeRaw(chain []internal.ChainRecord) error {
	for _, entry := range chain {
		prevSessionID := mo.PointerToOption(entry.PrevSessionID).OrElse("")
		endedAt := mo.PointerToOption(entry.EndedAt).OrElse("")
		if _, err := fmt.Fprintf(os.Stdout, "%s\t%s\t%s\t%s\t%s\t%s\t%v\n",
			entry.SessionID, prevSessionID, entry.StartedAt,
			endedAt, entry.Display, entry.TranscriptPath, entry.IsSidechain); err != nil {
			return err
		}
	}

	return nil
}

func writeHumanReadable(chain []internal.ChainRecord) error {
	if _, err := fmt.Fprintf(os.Stdout, "Session Chain (%d entries):\n", len(chain)); err != nil {
		return err
	}
	for i, entry := range chain {
		prevSessionID := mo.PointerToOption(entry.PrevSessionID).OrElse("null")
		endedAt := mo.PointerToOption(entry.EndedAt).OrElse("null")
		sidechainInfo := ""
		if entry.IsSidechain {
			sidechainInfo = " [sidechain]"
		}
		if _, err := fmt.Fprintf(os.Stdout, "  [%d] %s%s (prev: %s, started: %s, ended: %s)\n",
			i, entry.SessionID, sidechainInfo, prevSessionID, entry.StartedAt, endedAt); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(os.Stdout, "      display: %s\n", entry.Display); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(os.Stdout, "      transcript: %s\n", entry.TranscriptPath); err != nil {
			return err
		}
	}

	return nil
}
