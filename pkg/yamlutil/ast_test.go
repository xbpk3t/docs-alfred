package yamlutil

import (
	"testing"

	"github.com/goccy/go-yaml/ast"
	"github.com/goccy/go-yaml/parser"
	"github.com/stretchr/testify/require"
)

func TestASTHelpers(t *testing.T) {
	file, err := parser.ParseBytes([]byte("- name: demo\n  7: value\n  items:\n    - one\n  empty: ''\n"), parser.ParseComments)
	require.NoError(t, err)

	seq, ok := Sequence(file.Docs[0].Body)
	require.True(t, ok, "document body is not a sequence")
	mapping, ok := Mapping(seq.Values[0])
	require.True(t, ok, "sequence value is not a mapping")

	require.Equal(t, 1, NodeLine(mapping))

	got, ok := String(MappingValue(mapping, "name"))
	require.True(t, ok)
	require.Equal(t, "demo", got)

	require.Nil(t, MappingValue(mapping, "missing"))

	_, ok = Sequence(MappingValue(mapping, "items"))
	require.True(t, ok, "items is not a sequence")

	require.True(t, IsNullOrEmptyString(MappingValue(mapping, "empty")), "empty string should be treated as empty")
}

func TestKeyStringHandlesIntegerKeys(t *testing.T) {
	file, err := parser.ParseBytes([]byte("7: value\n"), parser.ParseComments)
	require.NoError(t, err)

	mapping, ok := file.Docs[0].Body.(*ast.MappingNode)
	require.True(t, ok, "document body is not a mapping")

	require.Equal(t, "7", KeyString(mapping.Values[0].Key))
}

func TestNilHelpers(t *testing.T) {
	require.Equal(t, 0, NodeLine(nil), "nil node line should be 0")
	require.Nil(t, MappingValue(nil, "x"), "nil mapping lookup should be nil")
	require.True(t, IsNullOrEmptyString(nil), "nil should be empty")
}
