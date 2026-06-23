package domrules

type topicEntry struct {
	Topic  string   `json:"topic"`
	Topics []string `json:"topics,omitempty"`
}

// extractTopicEntries extracts topic entries from a backbone entry.
func extractTopicEntries(entry map[string]any) []topicEntry {
	raw, ok := entry["topics"].([]any)
	if !ok {
		return nil
	}

	var topics []topicEntry
	for _, r := range raw {
		if te, ok := parseTopicEntry(r); ok {
			topics = append(topics, te)
		}
	}

	return topics
}

func parseTopicEntry(r any) (topicEntry, bool) {
	t, ok := r.(map[string]any)
	if !ok {
		return topicEntry{}, false
	}

	topic, _ := t["topic"].(string)
	if topic == "" {
		return topicEntry{}, false
	}

	te := topicEntry{Topic: topic}
	te.Topics = parseSubTopics(t["sub"])

	return te, true
}

func parseSubTopics(sub any) []string {
	subs, ok := sub.([]any)
	if !ok {
		return nil
	}

	var topics []string
	for _, s := range subs {
		sub, ok := s.(map[string]any)
		if !ok {
			continue
		}
		st, ok := sub["topic"].(string)
		if ok && st != "" {
			topics = append(topics, st)
		}
	}

	return topics
}
