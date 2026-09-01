package cmd

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/spf13/cobra"
	data "github.com/xbpk3t/docs-alfred/internal/gh/domrules"
	ghindex "github.com/xbpk3t/docs-alfred/internal/gh/index"
	"github.com/xbpk3t/docs-alfred/pkg/output"
)

// topicSel narrows a dump to a single topic: tag→type→topic, each optional.
// Topic names repeat across the tree (and even within a type), so the 3-level
// filter pinpoints one; the first match is emitted.
type topicSel struct {
	Tag   string
	Type  string
	Topic string
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
	var topic topicSel

	cmd := &cobra.Command{
		Use:   "dump <domain>",
		Short: "Dump data metadata as JSON to stdout",
		Long: `Load data from a domain's YAML files and output type-level metadata (type, tag, topics) as JSON.

For domain gh, topics use the same kind filter as TopicCatalog
(default: mech,type,repo,tools). Override with --kinds.

With --topic (optionally narrowed by --tag/--type), dump the full content of a
single topic (record/des/qs/table) instead of metadata. Topic names repeat, so
give as many of --tag/--type/--topic as you can to pinpoint the one you want;
on duplicate match the first is emitted.

	data-cli dump gh --type ss --topic goods`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			domain, err := parseDataDomainArg(args[0])
			if err != nil {
				return err
			}
			return runDomainDump(domain, *dataPath, kindsFlag, topic)
		},
	}

	cmd.Flags().StringVar(&kindsFlag, "kinds", "",
		"Comma-separated topic.kinds to include (default: mech,type,repo,tools)")
	cmd.Flags().StringVar(&topic.Tag, "tag", "",
		"When set with --topic, emit the matching topic from this tag")
	cmd.Flags().StringVar(&topic.Type, "type", "",
		"When set with --topic, emit the matching topic from this type")
	cmd.Flags().StringVar(&topic.Topic, "topic", "",
		"Dump the full content of one topic instead of type-level metadata")

	return cmd
}

func runDomainDump(domain data.DataDomain, dataPath, kindsFlag string, topic topicSel) error {
	path, err := data.DomainDefaultPath(domain, dataPath)
	if err != nil {
		return err
	}

	slog.Info("Dumping domain", "domain", domain, "path", path, "kinds", kindsFlagOrDefault(kindsFlag))

	repos, err := ghindex.LoadConfigReposFromDir(path)
	if err != nil {
		return fmt.Errorf("load data: %w", err)
	}

	if topic.Topic != "" {
		return dumpTopic(repos, topic)
	}

	kinds, err := parseDumpKinds(kindsFlag)
	if err != nil {
		return err
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
			if !ghindex.KindAllowed(string(r.Topics[i].Kind), kinds) {
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

// dumpTopic emits the full content of the topic selected by s (first match).
// Any of tag/type may be empty to loosen the search; --topic itself is required.
func dumpTopic(repos ghindex.ConfigRepos, s topicSel) error {
	for _, r := range repos {
		if s.Tag != "" && r.Tag != s.Tag {
			continue
		}
		if s.Type != "" && r.Type != s.Type {
			continue
		}
		for i := range r.Topics {
			if r.Topics[i].Topic != s.Topic {
				continue
			}
			return output.WriteJSON(&r.Topics[i])
		}
	}
	return fmt.Errorf("topic %q not found", s.Topic)
}

func parseDumpKinds(flag string) (map[string]struct{}, error) {
	list := ghindex.DefaultTopicKinds
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

	return ghindex.KindSet(list), nil
}

func kindsFlagOrDefault(flag string) string {
	if strings.TrimSpace(flag) == "" {
		return strings.Join(ghindex.DefaultTopicKinds, ",")
	}

	return strings.TrimSpace(flag)
}
