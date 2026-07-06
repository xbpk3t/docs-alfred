package ghdata

import (
	"github.com/go-viper/mapstructure/v2"
	"github.com/xbpk3t/docs-alfred/internal/gh/content"
)

// Section is a typed representation of a data/gh YAML section.
type Section struct {
	Type        string           `yaml:"type"`
	Repos       []content.Repo   `yaml:"repo"`
	Topics      []content.Topic  `yaml:"topics"`
	Record      []content.Record `yaml:"record"`
	HasRecord   bool             `yaml:"-"`
	RecordValid bool             `yaml:"-"`
}

// Repo is an alias for content.Repo.
type Repo = content.Repo

// Topic is an alias for content.Topic.
type Topic = content.Topic

// Record is an alias for content.Record.
type Record = content.Record

// TopicMeta is an alias for content.TopicMeta.
type TopicMeta = content.TopicMeta

type sectionFields struct {
	Type string `yaml:"type"`
}

type repoFields struct {
	URL string `yaml:"url"`
	Des string `yaml:"des"`
	Zk  string `yaml:"zk"`
	Nix string `yaml:"nix"`
	Doc string `yaml:"doc"`
}

type topicFields struct {
	Topic  string          `yaml:"topic"`
	Meta   content.TopicMeta `yaml:"meta"`
	HasPic bool            `yaml:"hasPic"`
}

func sectionFromMap(m map[string]any) Section {
	section := Section{RecordValid: true}
	var fields sectionFields
	decodeYAMLMap(m, &fields)
	section.Type = fields.Type
	section.Topics = topicsFromAny(m["topics"])

	if repos, ok := m["repo"].([]any); ok {
		section.Repos = make([]content.Repo, 0, len(repos))
		for _, item := range repos {
			if repoMap, ok := item.(map[string]any); ok {
				section.Repos = append(section.Repos, repoFromMap(repoMap))
			}
		}
	}

	if record, ok := m["record"]; ok {
		section.HasRecord = true
		section.Record, section.RecordValid = recordsFromAny(record)
	}

	return section
}

func repoFromMap(m map[string]any) content.Repo {
	repo := content.Repo{RecordValid: true}
	var fields repoFields
	decodeYAMLMap(m, &fields)
	repo.URL = fields.URL
	repo.Des = fields.Des
	repo.Zk = fields.Zk
	repo.NixURL = fields.Nix
	repo.Doc = fields.Doc
	repo.Topics = topicsFromAny(m["topics"])

	if record, ok := m["record"]; ok {
		repo.HasRecord = true
		repo.Record, repo.RecordValid = recordsFromAny(record)
	}

	return repo
}

func topicsFromAny(v any) []content.Topic {
	items, ok := v.([]any)
	if !ok {
		return nil
	}

	topics := make([]content.Topic, 0, len(items))
	for _, item := range items {
		if topicMap, ok := item.(map[string]any); ok {
			topics = append(topics, topicFromMap(topicMap))
		}
	}

	return topics
}

func topicFromMap(m map[string]any) content.Topic {
	topic := content.Topic{RecordValid: true}
	var fields topicFields
	decodeYAMLMap(m, &fields)
	topic.Topic = fields.Topic
	topic.Meta = &fields.Meta
	topic.HasPic = fields.HasPic
	topic.Sub = topicsFromAny(m["sub"])

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

	if record, ok := m["record"]; ok {
		topic.HasRecord = true
		topic.Record, topic.RecordValid = recordsFromAny(record)
	}

	return topic
}

func recordsFromAny(v any) ([]content.Record, bool) {
	items, ok := v.([]any)
	if !ok || items == nil {
		return nil, false
	}

	records := make([]content.Record, 0, len(items))
	for _, item := range items {
		if recordMap, ok := item.(map[string]any); ok {
			record := content.Record{}
			decodeYAMLMap(recordMap, &record)
			records = append(records, record)
		}
	}

	return records, true
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
