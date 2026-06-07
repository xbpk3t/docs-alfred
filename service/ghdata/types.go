package ghdata

// Section is a typed representation of a data/gh YAML section.
type Section struct {
	Using       *Repo
	Type        string
	Repos       []Repo
	Topics      []Topic
	Record      []Record
	HasRecord   bool
	RecordValid bool
}

// Repo is a typed representation of a repository entry in data/gh YAML.
type Repo struct {
	URL         string
	Des         string
	Zk          string
	Topics      []Topic
	Record      []Record
	HasRecord   bool
	RecordValid bool
}

// Topic is a typed representation of a topic entry in data/gh YAML.
type Topic struct {
	Meta        TopicMeta
	Topic       string
	Sub         []Topic
	Record      []Record
	HasPic      bool
	HasRecord   bool
	RecordValid bool
}

// TopicMeta holds topic metadata used by images checks.
type TopicMeta struct {
	Slug   string
	HasPic bool
}

// Record is a dated note attached to a repo or topic.
type Record struct {
	Date string
	Des  string
}

func sectionFromMap(m map[string]any) Section {
	section := Section{RecordValid: true}
	section.Type, _ = m["type"].(string)
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
	repo.URL, _ = m["url"].(string)
	repo.Des, _ = m["des"].(string)
	repo.Zk, _ = m["zk"].(string)
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
	topic.Topic, _ = m["topic"].(string)
	topic.HasPic, _ = m["hasPic"].(bool)
	topic.Sub = topicsFromAny(m["sub"])

	if metaMap, ok := m["meta"].(map[string]any); ok {
		topic.Meta.Slug, _ = metaMap["slug"].(string)
		topic.Meta.HasPic, _ = metaMap["hasPic"].(bool)
	}

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
			record.Date, _ = recordMap["date"].(string)
			record.Des, _ = recordMap["des"].(string)
			records = append(records, record)
		}
	}

	return records, true
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
