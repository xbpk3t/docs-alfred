package data

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"

	"github.com/xbpk3t/docs-alfred/pkg/fileutil"
)

type topicEntry struct {
	Topic  string   `json:"topic"`
	Topics []string `json:"topics,omitempty"`
}

type backboneEntry struct {
	Tag    string       `json:"tag"`
	Type   string       `json:"type"`
	Topics []topicEntry `json:"topics"`
}

// ExtractTopics extracts topic backbone from docs/public/gh.json.
// Produces: [{tag, type, topics: [{topic, ...}]}].
func ExtractTopics(outPath string) error {
	// TODO: 改成可配置path
	renderedPath := "docs/public/gh.json"
	data, err := os.ReadFile(renderedPath)
	if err != nil {
		return fmt.Errorf("read %s: %w", renderedPath, err)
	}

	var entries []map[string]any
	if err2 := json.Unmarshal(data, &entries); err2 != nil {
		return fmt.Errorf("parse %s: %w", renderedPath, err2)
	}

	var backbone []backboneEntry
	for _, entry := range entries {
		tag, _ := entry["tag"].(string)
		typ, _ := entry["type"].(string)
		if tag == "" || typ == "" {
			continue
		}

		topics := extractTopicEntries(entry)

		if len(topics) > 0 {
			backbone = append(backbone, backboneEntry{
				Tag:    tag,
				Type:   typ,
				Topics: topics,
			})
		}
	}

	outData, err := json.MarshalIndent(backbone, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal backbone: %w", err)
	}

	if err := fileutil.AtomicWriteFile(outPath, outData, fileutil.FilePermPrivate); err != nil {
		return fmt.Errorf("write %s: %w", outPath, err)
	}

	slog.Info("Extracted topic groups", "count", len(backbone), "path", outPath)

	return nil
}

// extractTopicEntries extracts topic entries from a backbone entry.
//
//nolint:nestif // nested type assertions inherent to unstructured YAML traversal
func extractTopicEntries(entry map[string]any) []topicEntry {
	var topics []topicEntry
	if raw, ok := entry["topics"].([]any); ok {
		for _, r := range raw {
			if t, ok := r.(map[string]any); ok {
				te := topicEntry{
					Topic: fmt.Sprintf("%v", t["topic"]),
				}
				if subs, ok := t["sub"].([]any); ok {
					for _, s := range subs {
						if sub, ok := s.(map[string]any); ok {
							te.Topics = append(te.Topics, fmt.Sprintf("%v", sub["topic"]))
						}
					}
				}
				topics = append(topics, te)
			}
		}
	}

	return topics
}
