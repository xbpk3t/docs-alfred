package cmd

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/spf13/cobra"
	data "github.com/xbpk3t/docs-alfred/internal/gh/domrules"
	"github.com/xbpk3t/docs-alfred/internal/gh/ghcheck"
	ghindex "github.com/xbpk3t/docs-alfred/internal/gh/index"
	"github.com/xbpk3t/docs-alfred/pkg/output"
)

// defaultDumpKinds is the formal topic map for dump (temp excluded).
var defaultDumpKinds = []string{
	ghcheck.KindMechanism,
	ghcheck.KindType,
	ghcheck.KindRepo,
	ghcheck.KindTools,
}

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
	var kindsFlag string

	cmd := &cobra.Command{
		Use:   "dump <domain>",
		Short: "Dump data metadata as JSON to stdout",
		Long: `Load data from a domain's YAML files and output type-level metadata (type, tag, topics) as JSON.

For domain gh, topics are filtered by topic.kind.
Default: mech,type,repo,tools. Override with --kinds (comma-separated, e.g. mech,temp).`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			domain, err := parseDataDomainArg(args[0])
			if err != nil {
				return err
			}
			return runDomainDump(domain, *dataPath, kindsFlag)
		},
	}

	cmd.Flags().StringVar(&kindsFlag, "kinds", "",
		"Comma-separated topic.kinds to include (default: mech,type,repo,tools)")

	return cmd
}

func runDomainDump(domain data.DataDomain, dataPath, kindsFlag string) error {
	spec, ok := data.SpecForDomain(domain)
	if !ok {
		return fmt.Errorf("unknown data domain %q", domain)
	}
	path := dataPath
	if path == "" {
		path = spec.DefaultPath
	}

	kinds, err := parseDumpKinds(kindsFlag)
	if err != nil {
		return err
	}

	slog.Info("Dumping domain", "domain", domain, "path", path, "kinds", kindsFlagOrDefault(kindsFlag))

	repos, err := ghindex.LoadConfigReposFromDir(path)
	if err != nil {
		return fmt.Errorf("load data: %w", err)
	}

	result := make([]dumpTag, 0, len(repos))
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
			if _, ok := kinds[strings.TrimSpace(r.Topics[i].Kind)]; !ok {
				continue
			}
			topics = append(topics, r.Topics[i].Topic)
		}
		result[idx].Types = append(result[idx].Types, dumpType{
			Type:   r.Type,
			Topics: topics,
		})
	}

	return output.WriteJSON(result)
}

func parseDumpKinds(flag string) (map[string]struct{}, error) {
	list := defaultDumpKinds
	if raw := strings.TrimSpace(flag); raw != "" {
		list = nil
		for _, p := range strings.Split(raw, ",") {
			k := strings.TrimSpace(p)
			if k == "" {
				return nil, fmt.Errorf("--kinds: empty kind in %q", flag)
			}
			list = append(list, k)
		}
	}

	set := make(map[string]struct{}, len(list))
	for _, k := range list {
		set[k] = struct{}{}
	}

	return set, nil
}

func kindsFlagOrDefault(flag string) string {
	if strings.TrimSpace(flag) == "" {
		return strings.Join(defaultDumpKinds, ",")
	}

	return strings.TrimSpace(flag)
}
