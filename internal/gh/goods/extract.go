package goods

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	modelgoods "github.com/xbpk3t/docs-alfred/internal/gh/model/goods"
	"github.com/xbpk3t/docs-alfred/pkg/fileutil"
	"github.com/xbpk3t/docs-alfred/pkg/parser"
)

// UsingItem is a table item currently in use (isUsing: true).
type UsingItem struct {
	Extra  map[string]string `json:"extra,omitempty" yaml:"extra,omitempty"`
	Name   string            `json:"name"  yaml:"name"`
	Brand  string            `json:"brand,omitempty" yaml:"brand,omitempty"`
	Param  string            `json:"param,omitempty" yaml:"param,omitempty"`
	Price  string            `json:"price,omitempty" yaml:"price,omitempty"`
	Date   string            `json:"date,omitempty" yaml:"date,omitempty"`
	Des    string            `json:"des,omitempty" yaml:"des,omitempty"`
	URL    string            `json:"url,omitempty" yaml:"url,omitempty"`
	Source string            `json:"source,omitempty" yaml:"source,omitempty"`
}

// UsingTopic groups in-use items by topic.
type UsingTopic struct {
	Topic string      `json:"topic"`
	Items []UsingItem `json:"items"`
}

// UsingType groups in-use items by type.
type UsingType struct {
	Type   string       `json:"type"`
	Topics []UsingTopic `json:"topics"`
}

// UsingExtract holds extracted in-use goods grouped by type → topic.
type UsingExtract []UsingType

// ExtractUsing reads all goods YAML files under dir and returns items with
// isUsing: true, grouped by type → topic. Files are parsed as flattened
// multi-document YAML (same shape as data/gh). Only item maps that declare
// isUsing explicitly as true are kept.
func ExtractUsing(dir string) (UsingExtract, error) {
	files, err := fileutil.ListYAMLFiles(dir)
	if err != nil {
		return nil, fmt.Errorf("list goods files %s: %w", dir, err)
	}

	types := make(map[string]int)
	topics := make(map[string]int)
	var out UsingExtract

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
func extractFileUsing(file string, out *UsingExtract, types, topics map[string]int) error {
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
			*out = append(*out, UsingType{Type: g.Type})
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
				(*out)[ti].Topics = append((*out)[ti].Topics, UsingTopic{Topic: tp.Topic})
			}
			(*out)[ti].Topics[pi].Items = append((*out)[ti].Topics[pi].Items, items...)
		}
	}

	return nil
}

// usingItemsFromTable returns items with isUsing: true from a table row list,
// in original order. Non-map rows and missing/empty names are skipped.
func usingItemsFromTable(table []modelgoods.TableItem) []UsingItem {
	var items []UsingItem
	for _, row := range table {
		v, ok := row["isUsing"]
		if !ok || !boolTrue(v) {
			continue
		}

		item := UsingItem{
			Name:   scalarString(row, "name"),
			Brand:  stringField(row, "brand"),
			Param:  stringField(row, "param"),
			Price:  stringField(row, "price"),
			Date:   stringField(row, "date"),
			Des:    stringField(row, "des"),
			URL:    stringField(row, "url"),
			Source: stringField(row, "source"),
		}
		if item.Name == "" {
			continue
		}

		var extra map[string]string
		for key, val := range row {
			if !knownItemField(key) {
				if extra == nil {
					extra = make(map[string]string)
				}
				if s, ok := val.(string); ok {
					extra[key] = s
				}
			}
		}
		item.Extra = extra

		items = append(items, item)
	}

	return items
}

// knownItemField reports whether key is a canonical UsingItem field or an
// established goods item field (domrules requires these to stay as fields).
func knownItemField(key string) bool {
	switch key {
	case "name", "brand", "param", "price", "date", "des", "isUsing",
		"url", "source", "endDate", "endPrice", "use",
		"developer", "genre", "status", "platform", "playAt", "publishAt", "alias":
		return true
	default:
		return false
	}
}

// boolTrue reports whether v represents a true boolean.
// goccy/yaml decodes true as bool, but unquoted YAML 1.1 truthy values
// (on/yes/y/TRUE) come through as strings; all are accepted here so a
// data typo like "isUsing: on" is not silently dropped.
func boolTrue(v interface{}) bool {
	switch b := v.(type) {
	case bool:
		return b
	case string:
		switch strings.ToLower(strings.TrimSpace(b)) {
		case "true", "on", "yes", "y":
			return true
		default:
			return false
		}
	default:
		return false
	}
}

func stringField(row map[string]interface{}, key string) string {
	s, _ := row[key].(string)

	return s
}

// scalarString converts a YAML scalar value to its string form, so numeric
// names (e.g. phone numbers like 18616287252) are kept as strings.
// goccy/yaml decodes unquoted numerics as int; non-scalar values yield "".
func scalarString(row map[string]interface{}, key string) string {
	v, ok := row[key]
	if !ok {
		return ""
	}
	switch s := v.(type) {
	case string:
		return s
	case int:
		return strconv.Itoa(s)
	case int64:
		return strconv.FormatInt(s, 10)
	case uint64:
		return strconv.FormatUint(s, 10)
	case float64:
		return strconv.FormatFloat(s, 'f', -1, 64)
	default:
		return ""
	}
}
