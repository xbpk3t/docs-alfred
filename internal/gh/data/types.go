package ghdata

import (
	"github.com/go-viper/mapstructure/v2"
	"github.com/xbpk3t/docs-alfred/internal/gh/content"
)

// Section is a typed representation of a data/gh YAML section.
type Section struct {
	Type  string         `yaml:"type"`
	Repos []content.Repo `yaml:"repo"`
}

// Repo is an alias for content.Repo.
type Repo = content.Repo

// Topic is an alias for content.Topic.
type Topic = content.Topic

type sectionFields struct {
	Type string `yaml:"type"`
}

type repoFields struct {
	URL string `yaml:"url"`
	Des string `yaml:"des"`
	Nix string `yaml:"nix"`
	Doc string `yaml:"doc"`
}

type topicFields struct {
	Topic string `yaml:"topic"`
}

func sectionFromMap(m map[string]any) Section {
	var fields sectionFields
	decodeYAMLMap(m, &fields)

	section := Section{Type: fields.Type}

	if repos, ok := m["repo"].([]any); ok {
		section.Repos = make([]content.Repo, 0, len(repos))
		for _, item := range repos {
			if repoMap, ok := item.(map[string]any); ok {
				section.Repos = append(section.Repos, repoFromMap(repoMap))
			}
		}
	}

	return section
}

func repoFromMap(m map[string]any) content.Repo {
	var fields repoFields
	decodeYAMLMap(m, &fields)

	return content.Repo{
		URL:    fields.URL,
		Des:    fields.Des,
		NixURL: fields.Nix,
		Doc:    fields.Doc,
	}
}

func topicFromMap(m map[string]any) content.Topic {
	var fields topicFields
	decodeYAMLMap(m, &fields)

	topic := content.Topic{Topic: fields.Topic}

	// 解析 topic 内嵌的 repos
	if repos, ok := m["repo"].([]any); ok {
		topic.Repos = make([]*content.Repo, 0, len(repos))
		for _, item := range repos {
			if repoMap, ok := item.(map[string]any); ok {
				repo := repoFromMap(repoMap)
				topic.Repos = append(topic.Repos, &repo)
			}
		}
	}

	return topic
}

func decodeYAMLMap(input, output any) {
	decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		Result:  output,
		TagName: "yaml",
	})
	if err != nil {
		return
	}
	_ = decoder.Decode(input)
}
