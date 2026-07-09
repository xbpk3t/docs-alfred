package cmd

import (
	"fmt"
	"log/slog"

	"github.com/spf13/cobra"
	data "github.com/xbpk3t/docs-alfred/internal/gh/domrules"
	ghindex "github.com/xbpk3t/docs-alfred/internal/gh/index"
	"github.com/xbpk3t/docs-alfred/pkg/output"
)

// output structures for tag → types → topics three-level nesting.
type (
	dumpType struct {
		Type   string   `json:"type"`
		Topics []string `json:"topics,omitempty"`
	}
	dumpTag struct {
		Tag   string     `json:"tag"`
		Types []dumpType `json:"types"`
	}
)

func newDumpCmd(dataPath *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "dump <domain>",
		Short: "Dump data metadata as JSON to stdout",
		Long:  "Load data from a domain's YAML files and output only type-level metadata (type, tag, topics) as JSON to stdout.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			domain, err := parseDataDomainArg(args[0])
			if err != nil {
				return err
			}
			return runDomainDump(domain, *dataPath)
		},
	}

	return cmd
}

func runDomainDump(domain data.DataDomain, dataPath string) error {
	spec, ok := data.SpecForDomain(domain)
	if !ok {
		return fmt.Errorf("unknown data domain %q", domain)
	}
	path := dataPath
	if path == "" {
		path = spec.DefaultPath
	}

	slog.Info("Dumping domain", "domain", domain, "path", path)

	repos, err := ghindex.LoadConfigReposFromDir(path)
	if err != nil {
		return fmt.Errorf("load data: %w", err)
	}

	result := make([]dumpTag, 0, len(repos))
	// group by tag
	typeMap := make(map[string]int) // tag → index in result
	for _, r := range repos {
		idx, ok := typeMap[r.Tag]
		if !ok {
			idx = len(result)
			typeMap[r.Tag] = idx
			result = append(result, dumpTag{Tag: r.Tag})
		}

		topics := make([]string, 0, len(r.Topics))
		for i := range r.Topics {
			topics = append(topics, r.Topics[i].Topic)
		}
		result[idx].Types = append(result[idx].Types, dumpType{
			Type:   r.Type,
			Topics: topics,
		})
	}

	return output.WriteJSON(result)
}
