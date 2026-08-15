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

// extractFileUsing parses one goods YAML file and merges its in-use items into out.
// types/topics maps track index positions so items from separate files merge
// into the same type/topic buckets.
func extractFileUsing(file string, out *modelgoods.UsingSchemaJson, types, topics map[string]int) error {
	data, err := os.ReadFile(file)
	if err != nil {
		return fmt.Errorf("read goods file %s: %w", file, err)
	}

	goodsList, err := parser.NewParser[modelgoods.Section](data).ParseFlatten()
	if err != nil {
		return fmt.Errorf("parse goods file %s: %w", file, err)
	}

	for i := range goodsList {
		g := &goodsList[i]

		ti, ok := types[g.Type]
		if !ok {
			ti = len(*out)
			types[g.Type] = ti
			// Empty non-nil slice so `topics` marshals as [] (not null) and stays
			// valid against using.schema.json even for types with no in-use items.
			*out = append(*out, modelgoods.UsingType{Type: g.Type, Topics: []modelgoods.UsingTopic{}})
		}

		for j := range g.Topics {
			tp := &g.Topics[j]
			items := usingItemsFromTable(tp.Table)
			if len(items) == 0 {
				continue
			}

			pi, ok := topics[g.Type+"\x00"+tp.Topic]
			if !ok {
				pi = len((*out)[ti].Topics)
				topics[g.Type+"\x00"+tp.Topic] = pi
				(*out)[ti].Topics = append((*out)[ti].Topics, modelgoods.UsingTopic{Topic: tp.Topic})
			}
			(*out)[ti].Topics[pi].Items = append((*out)[ti].Topics[pi].Items, items...)
		}
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
