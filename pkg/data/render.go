package data

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"sync"

	yaml "github.com/goccy/go-yaml"
	docscli "github.com/xbpk3t/docs-alfred/docs-cli/pkg"
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

// ExtractTopics extracts topic backbone from data/rendered/gh.json.
// Produces: [{tag, type, topics: [{topic, ...}]}].
func ExtractTopics(outPath string) error {
	renderedPath := "data/rendered/gh.json"
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

	if err := os.WriteFile(outPath, outData, fileutil.FilePermPrivate); err != nil {
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

// LoadRenderConfigs reads and parses the render config file.
func LoadRenderConfigs(cfgFile string) ([]docscli.DocsConfig, error) {
	configData, err := os.ReadFile(cfgFile)
	if err != nil {
		return nil, err
	}

	var rawConfigs []docscli.DocsConfig
	if err := yaml.NewDecoder(bytes.NewReader(configData)).Decode(&rawConfigs); err != nil {
		return nil, err
	}

	configs := make([]docscli.DocsConfig, 0, len(rawConfigs))
	for _, raw := range rawConfigs {
		configs = append(configs, processRenderConfig(raw))
	}

	return configs, nil
}

// processRenderConfig copies and initializes a DocsConfig from a raw one.
func processRenderConfig(raw docscli.DocsConfig) docscli.DocsConfig {
	config := docscli.DocsConfig{
		Src: raw.Src,
		Cmd: raw.Cmd,
	}
	if raw.JSON != nil {
		config.JSON = docscli.NewDocProcessor(docscli.FileTypeJSON)
		config.JSON.Dst = raw.JSON.Dst
		config.JSON.MergeOutputFile = raw.JSON.MergeOutputFile
	}
	if raw.YAML != nil {
		config.YAML = docscli.NewDocProcessor(docscli.FileTypeYAML)
		config.YAML.Dst = raw.YAML.Dst
		config.YAML.MergeOutputFile = raw.YAML.MergeOutputFile
	}

	return config
}

// ProcessRenderConfigs processes multiple render configs concurrently.
func ProcessRenderConfigs(configs []docscli.DocsConfig) {
	var wg sync.WaitGroup
	wg.Add(len(configs))

	for _, config := range configs {
		go func(cfg docscli.DocsConfig) {
			defer wg.Done()
			_ = cfg.Process()
		}(config)
	}

	wg.Wait()
}
