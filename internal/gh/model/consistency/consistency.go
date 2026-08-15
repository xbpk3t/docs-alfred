// Package consistency verifies that schema-generated model structs stay in
// lockstep with their JSON Schema. Each model package's *_gen.go is generated
// from a .schema.json via atombender/go-jsonschema; this package holds the
// shared test helpers used by the per-package consistency tests.
package consistency

import (
	"encoding/json"
	"reflect"
	"sort"
	"strings"
	"testing"
)

// Coarse type categories shared by schema- and Go-side mapping.
const (
	catAny      = "any"
	catString   = "string"
	catInt      = "int"
	catBool     = "bool"
	catObject   = "object"
	catArrayPre = "array:"
)

// CheckDefs verifies every $def in schemaBytes against a matching struct
// instance (def name → zero-value struct):
//
//  1. Field names: each schema property must have a matching yaml tag.
//  2. Coarse type category: each property's schema type (integer/string/
//     boolean/array/object) must match the struct field's Go type category.
//     Union types (oneOf/anyOf/interface{}) are treated as "any" and match
//     anything, since atombender degrades those to interface{}.
//
// The type check closes the gap where a schema type changes (e.g. score
// integer→string) but the field name stays the same — the old field-name-only
// test could not catch that silent drift.
func CheckDefs(t *testing.T, schemaBytes []byte, instances map[string]any) {
	t.Helper()

	defs := parseDefs(t, schemaBytes)
	for name, inst := range instances {
		def, ok := defs[name]
		if !ok {
			t.Errorf("def %s: not present in schema", name)
			continue
		}
		props := sortedKeys(def.Properties)
		fields := fieldByYAMLTags(reflect.TypeOf(inst))
		if !equal(props, sortedKeys(fields)) {
			t.Errorf("def %s: schema properties %v != model yaml tags %v", name, props, sortedKeys(fields))
		}
		for _, p := range props {
			schemaCat := schemaTypeCat(def.Properties[p])
			goCat := goTypeCat(fields[p])
			if schemaCat == catAny || goCat == catAny {
				continue
			}
			if schemaCat != goCat {
				t.Errorf("def %s: property %s: schema %s != model %s (run go generate)", name, p, schemaCat, goCat)
			}
		}
	}
}

type def struct {
	Properties map[string]any `json:"properties"`
}

func parseDefs(t *testing.T, raw []byte) map[string]def {
	t.Helper()
	var doc struct {
		Defs map[string]def `json:"$defs"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("parse schema: %v", err)
	}
	return doc.Defs
}

func sortedKeys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// fieldByYAMLTags maps each struct field's yaml tag name to its reflect.Type.
// Non-struct types (e.g. gh's TableItem which is map[string]interface{}) have
// no fields and return an empty map.
func fieldByYAMLTags(tp reflect.Type) map[string]reflect.Type {
	out := make(map[string]reflect.Type)
	if tp.Kind() != reflect.Struct {
		return out
	}
	for i := 0; i < tp.NumField(); i++ {
		f := tp.Field(i)
		name := strings.SplitN(f.Tag.Get("yaml"), ",", 2)[0]
		if name == "" || name == "-" {
			continue
		}
		out[name] = f.Type
	}
	return out
}

// schemaTypeCat maps a JSON Schema property spec to a coarse category:
// string / int / bool / array:<elem> / object / any.
func schemaTypeCat(spec any) string {
	m, ok := spec.(map[string]any)
	if !ok {
		return catAny
	}
	if _, hasRef := m["$ref"]; hasRef {
		return catObject
	}
	if _, hasOneOf := m["oneOf"]; hasOneOf {
		return catAny
	}
	if _, hasAnyOf := m["anyOf"]; hasAnyOf {
		return catAny
	}

	typ, hasType := m["type"]
	if !hasType {
		return catAny
	}

	switch t := typ.(type) {
	case string:
		if t == "array" {
			return arrayCat(m["items"])
		}
		return stringCat(t)
	case []any:
		// Union types: ["string","null"] → string; anything else is a
		// number-bearing union atombender degrades to interface{}.
		for _, v := range t {
			if s, ok := v.(string); ok && s != "null" && s != catString {
				return catAny
			}
		}
		return catString
	default:
		return catAny
	}
}

func arrayCat(items any) string {
	im, ok := items.(map[string]any)
	if !ok {
		return catAny
	}
	if _, hasRef := im["$ref"]; hasRef {
		return catArrayPre + catObject
	}
	if _, hasOneOf := im["oneOf"]; hasOneOf {
		return catAny
	}
	if _, hasAnyOf := im["anyOf"]; hasAnyOf {
		return catAny
	}
	switch t := im["type"].(type) {
	case string:
		if t == "object" {
			return catArrayPre + catObject
		}
		if cat := stringCat(t); cat != catAny {
			return catArrayPre + cat
		}
	case []any:
		// ["string","null"] items → array of strings.
		for _, v := range t {
			if s, ok := v.(string); ok && s != "null" && s != catString {
				return catAny
			}
		}
		return catArrayPre + catString
	}
	return catAny
}

func stringCat(t string) string {
	switch t {
	case catString:
		return catString
	case "integer":
		return catInt
	case "boolean":
		return catBool
	case "object":
		return catObject
	default:
		return catAny
	}
}

// goTypeCat maps a Go field type to the same coarse category, folding pointers,
// named types, and nil-slice forms so optional fields match their schema type.
func goTypeCat(t reflect.Type) string {
	switch t.Kind() {
	case reflect.Pointer:
		return goTypeCat(t.Elem())
	case reflect.Slice:
		elem := goTypeCat(t.Elem())
		if elem == catAny {
			return catAny
		}
		return catArrayPre + elem
	case reflect.String:
		return catString
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return catInt
	case reflect.Bool:
		return catBool
	case reflect.Struct:
		return catObject
	case reflect.Map, reflect.Interface:
		return catAny
	default:
		return catAny
	}
}
