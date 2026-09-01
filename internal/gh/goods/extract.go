package goods

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	modelgoods "github.com/xbpk3t/docs-alfred/internal/gh/model/goods"
	"github.com/xbpk3t/docs-alfred/pkg/fileutil"
	"github.com/xbpk3t/docs-alfred/pkg/parser"
)

// AllSchemaJson is the full goods dump view: every topic in the goods domain,
// grouped by type (derived from file name). Unlike UsingSchemaJson, no row is
// dropped, so audits can read per-item fields (record/score/des/isUsing) and
// topic-level record/des/qs that the using projection omits.
type AllSchemaJSON []AllType

// AllType is the full content of one goods type (one or more goods.*.yml files).
type AllType struct {
	Type   string             `json:"type" yaml:"type" mapstructure:"type"`
	Topics []modelgoods.Topic `json:"topics" yaml:"topics" mapstructure:"topics"`
}

// ExtractAll reads all goods YAML files under dir and returns every topic with
// its full content, grouped by type → topic. Files are parsed as flattened
// multi-document YAML (same shape as data/gh); the type is derived from the
// file name. Rows are never dropped and no field is projected away.
func ExtractAll(dir string) (AllSchemaJSON, error) {
	files, err := fileutil.ListYAMLFiles(dir)
	if err != nil {
		return nil, fmt.Errorf("list goods files %s: %w", dir, err)
	}

	var out AllSchemaJSON
	typeIndex := make(map[string]int)
	for _, file := range files {
		if !isGoodsFileName(file) {
			continue
		}

		topics, typeName, err := parseGoodsFile(file)
		if err != nil {
			return nil, err
		}
		if len(topics) == 0 {
			continue
		}

		ti, ok := typeIndex[typeName]
		if !ok {
			ti = len(out)
			typeIndex[typeName] = ti
			out = append(out, AllType{Type: typeName})
		}
		out[ti].Topics = append(out[ti].Topics, topics...)
	}

	return out, nil
}

// ExtractUsing reads all goods YAML files under dir and returns items with
// isUsing: true, grouped by type → topic. Files are parsed as flattened
// multi-document YAML (same shape as data/gh). Only rows that declare isUsing
// explicitly as true are kept.
//
// The output shape (modelgoods.UsingSchemaJson) is generated from
// using.schema.json — the projection of goods.schema.json that the using view
// exposes. The mapping below is a pure field copy: goods.schema.json now types
// name/price as string, so no scalar normalization is needed.
func ExtractUsing(dir string) (modelgoods.UsingSchemaJson, error) {
	files, err := fileutil.ListYAMLFiles(dir)
	if err != nil {
		return nil, fmt.Errorf("list goods files %s: %w", dir, err)
	}

	types := make(map[string]int)
	topics := make(map[string]int)
	var out modelgoods.UsingSchemaJson

	for _, file := range files {
		if !isGoodsFileName(file) {
			continue
		}
		if err := extractFileUsing(file, &out, types, topics); err != nil {
			return nil, err
		}
	}

	return out, nil
}

// isGoodsFileName reports whether the file belongs to the goods domain,
// matching the goods.*.yml naming convention used by check (domrules).
func isGoodsFileName(file string) bool {
	name := filepath.Base(file)
	lower := strings.ToLower(name)

	return strings.HasPrefix(lower, "goods.") &&
		(strings.HasSuffix(lower, ".yml") || strings.HasSuffix(lower, ".yaml"))
}

// parseGoodsFile reads one goods YAML file and returns its flattened topics and
// the type derived from the file name. Shared by ExtractAll and ExtractUsing so
// the per-file parse pipeline lives in one place.
func parseGoodsFile(file string) ([]modelgoods.Topic, string, error) {
	data, err := os.ReadFile(file)
	if err != nil {
		return nil, "", fmt.Errorf("read goods file %s: %w", file, err)
	}

	topics, err := parser.NewParser[modelgoods.Topic](data).WithFileName(filepath.Base(file)).ParseFlatten()
	if err != nil {
		return nil, "", fmt.Errorf("parse goods file %s: %w", file, err)
	}

	return topics, modelgoods.TypeFromFilename(filepath.Base(file)), nil
}

// extractFileUsing parses one goods YAML file and merges its in-use items into out.
// types/topics maps track index positions so items from separate files merge
// into the same type/topic buckets.
func extractFileUsing(file string, out *modelgoods.UsingSchemaJson, types, topics map[string]int) error {
	// New layout: each file is a flat topic array and the whole file is one
	// type derived from the file name.
	topicsList, typeName, err := parseGoodsFile(file)
	if err != nil {
		return err
	}

	ti, ok := types[typeName]
	if !ok {
		ti = len(*out)
		types[typeName] = ti
		// Empty non-nil slice so `topics` marshals as [] (not null) and stays
		// valid against using.schema.json even for types with no in-use items.
		*out = append(*out, modelgoods.UsingType{Type: typeName, Topics: []modelgoods.UsingTopic{}})
	}

	for j := range topicsList {
		tp := &topicsList[j]
		items := usingItemsFromTable(tp.Table)
		if len(items) == 0 {
			continue
		}

		pi, ok := topics[typeName+"\x00"+tp.Topic]
		if !ok {
			pi = len((*out)[ti].Topics)
			topics[typeName+"\x00"+tp.Topic] = pi
			(*out)[ti].Topics = append((*out)[ti].Topics, modelgoods.UsingTopic{Topic: tp.Topic})
		}
		(*out)[ti].Topics[pi].Items = append((*out)[ti].Topics[pi].Items, items...)
	}

	return nil
}

// usingItemsFromTable returns items with isUsing: true from a table row list,
// in original order. Rows without isUsing, non-true isUsing, or missing/empty
// names are skipped. Unknown keys are rejected by goods.schema.json, so no
// extra-field collection is needed here.
func usingItemsFromTable(table []modelgoods.TableItem) []modelgoods.UsingItem {
	var items []modelgoods.UsingItem
	for _, row := range table {
		if row.IsUsing == nil || !*row.IsUsing {
			continue
		}

		item := modelgoods.UsingItem{
			Name:   row.Name,
			Brand:  row.Brand,
			Param:  row.Param,
			Price:  row.Price,
			Date:   row.Date,
			Des:    row.Des,
			URL:    row.URL,
			Source: row.Source,
		}
		if item.Name == "" {
			continue
		}

		items = append(items, item)
	}

	return items
}
