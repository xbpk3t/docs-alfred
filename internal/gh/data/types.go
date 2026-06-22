package ghdata

import "github.com/go-viper/mapstructure/v2"

// Section is a typed representation of a data/gh YAML section.
type Section struct {
	Using       *Repo    `yaml:"using"`
	Type        string   `yaml:"type"`
	Repos       []Repo   `yaml:"repo"`
	Topics      []Topic  `yaml:"topics"`
	Record      []Record `yaml:"record"`
	HasRecord   bool     `yaml:"-"`
	RecordValid bool     `yaml:"-"`
}

// Repo is a typed representation of a repository entry in data/gh YAML.
type Repo struct {
	URL         string   `yaml:"url"`
	Des         string   `yaml:"des"`
	Zk          string   `yaml:"zk"`
	Topics      []Topic  `yaml:"topics"`
	Record      []Record `yaml:"record"`
	HasRecord   bool     `yaml:"-"`
	RecordValid bool     `yaml:"-"`
}

// Topic is a typed representation of a topic entry in data/gh YAML.
type Topic struct {
	Meta        TopicMeta `yaml:"meta"`
	Topic       string    `yaml:"topic"`
	Sub         []Topic   `yaml:"sub"`
	Record      []Record  `yaml:"record"`
	HasPic      bool      `yaml:"hasPic"`
	HasRecord   bool      `yaml:"-"`
	RecordValid bool      `yaml:"-"`
}

// TopicMeta holds topic metadata used by images checks.
type TopicMeta struct {
	Slug   string `yaml:"slug"`
	HasPic bool   `yaml:"hasPic"`
}

// Record is a dated note attached to a repo or topic.
type Record struct {
	Date string `yaml:"date"`
	Des  string `yaml:"des"`
}

type sectionFields struct {
	Type string `yaml:"type"`
}

type repoFields struct {
	URL string `yaml:"url"`
	Des string `yaml:"des"`
	Zk  string `yaml:"zk"`
}

type topicFields struct {
	Topic  string    `yaml:"topic"`
	Meta   TopicMeta `yaml:"meta"`
	HasPic bool      `yaml:"hasPic"`
}

func sectionFromMap(m map[string]any) Section {
	section := Section{RecordValid: true}
	var fields sectionFields
	decodeYAMLMap(m, &fields)
	section.Type = fields.Type
	section.Topics = topicsFromAny(m["topics"])

	if using, ok := m["using"].(map[string]any); ok {
		repo := repoFromMap(using)
		section.Using = &repo
	}

	if repos, ok := m["repo"].([]any); ok {
		section.Repos = make([]Repo, 0, len(repos))
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

func repoFromMap(m map[string]any) Repo {
	repo := Repo{RecordValid: true}
	var fields repoFields
	decodeYAMLMap(m, &fields)
	repo.URL = fields.URL
	repo.Des = fields.Des
	repo.Zk = fields.Zk
	repo.Topics = topicsFromAny(m["topics"])

	if record, ok := m["record"]; ok {
		repo.HasRecord = true
		repo.Record, repo.RecordValid = recordsFromAny(record)
	}

	return repo
}

func topicsFromAny(v any) []Topic {
	items, ok := v.([]any)
	if !ok {
		return nil
	}

	topics := make([]Topic, 0, len(items))
	for _, item := range items {
		if topicMap, ok := item.(map[string]any); ok {
			topics = append(topics, topicFromMap(topicMap))
		}
	}

	return topics
}

func topicFromMap(m map[string]any) Topic {
	topic := Topic{RecordValid: true}
	var fields topicFields
	decodeYAMLMap(m, &fields)
	topic.Topic = fields.Topic
	topic.Meta = fields.Meta
	topic.HasPic = fields.HasPic
	topic.Sub = topicsFromAny(m["sub"])

	if record, ok := m["record"]; ok {
		topic.HasRecord = true
		topic.Record, topic.RecordValid = recordsFromAny(record)
	}

	return topic
}

func recordsFromAny(v any) ([]Record, bool) {
	items, ok := v.([]any)
	if !ok || items == nil {
		return nil, false
	}

	records := make([]Record, 0, len(items))
	for _, item := range items {
		if recordMap, ok := item.(map[string]any); ok {
			record := Record{}
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

// DirName returns the directory name implied by a topic.
func (t *Topic) DirName() string {
	if t.Meta.Slug != "" {
		return t.Meta.Slug
	}

	return t.Topic
}

// HasPicture reports whether a topic expects an image directory.
func (t *Topic) HasPicture() bool {
	return t.Meta.HasPic || t.HasPic
}
