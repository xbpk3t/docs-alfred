package yamlutil

import (
	"testing"

	"github.com/goccy/go-yaml/ast"
	"github.com/goccy/go-yaml/parser"
)

func TestASTHelpers(t *testing.T) {
	file, err := parser.ParseBytes([]byte("- name: demo\n  7: value\n  items:\n    - one\n  empty: ''\n"), parser.ParseComments)
	if err != nil {
		t.Fatalf("ParseBytes() error = %v", err)
	}

	seq, ok := Sequence(file.Docs[0].Body)
	if !ok {
		t.Fatal("document body is not a sequence")
	}
	mapping, ok := Mapping(seq.Values[0])
	if !ok {
		t.Fatal("sequence value is not a mapping")
	}

	if got := NodeLine(mapping); got != 1 {
		t.Fatalf("NodeLine() = %d, want 1", got)
	}

	if got, ok := String(MappingValue(mapping, "name")); !ok || got != "demo" {
		t.Fatalf("String(name) = %q, %v; want demo, true", got, ok)
	}

	if got := MappingValue(mapping, "missing"); got != nil {
		t.Fatalf("missing value = %v, want nil", got)
	}

	if _, ok := Sequence(MappingValue(mapping, "items")); !ok {
		t.Fatal("items is not a sequence")
	}

	if !IsNullOrEmptyString(MappingValue(mapping, "empty")) {
		t.Fatal("empty string should be treated as empty")
	}
}

func TestKeyStringHandlesIntegerKeys(t *testing.T) {
	file, err := parser.ParseBytes([]byte("7: value\n"), parser.ParseComments)
	if err != nil {
		t.Fatalf("ParseBytes() error = %v", err)
	}
	mapping, ok := file.Docs[0].Body.(*ast.MappingNode)
	if !ok {
		t.Fatal("document body is not a mapping")
	}

	if got := KeyString(mapping.Values[0].Key); got != "7" {
		t.Fatalf("KeyString() = %q, want 7", got)
	}
}

func TestNilHelpers(t *testing.T) {
	if NodeLine(nil) != 0 {
		t.Fatal("nil node line should be 0")
	}
	if MappingValue(nil, "x") != nil {
		t.Fatal("nil mapping lookup should be nil")
	}
	if !IsNullOrEmptyString(nil) {
		t.Fatal("nil should be empty")
	}
}
