package yamlutil

import (
	"testing"

	"github.com/goccy/go-yaml/ast"
	"github.com/goccy/go-yaml/parser"
	"github.com/goccy/go-yaml/token"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- NodeLine edge cases ---

func TestNodeLineNilToken(t *testing.T) {
	// A node with nil token should return 0
	// StringNode has a token, but we can test with nil
	assert.Equal(t, 0, NodeLine(nil))
}

func TestNodeLineValidNode(t *testing.T) {
	file, err := parser.ParseBytes([]byte("key: value\n"), parser.ParseComments)
	require.NoError(t, err)
	mapping, ok := file.Docs[0].Body.(*ast.MappingNode)
	require.True(t, ok)
	assert.Equal(t, 1, NodeLine(mapping))
}

// --- KeyString edge cases ---

func TestKeyStringNil(t *testing.T) {
	assert.Empty(t, KeyString(nil))
}

func TestKeyStringStringNode(t *testing.T) {
	file, err := parser.ParseBytes([]byte("mykey: value\n"), parser.ParseComments)
	require.NoError(t, err)
	mapping, ok := file.Docs[0].Body.(*ast.MappingNode)
	require.True(t, ok)
	assert.Equal(t, "mykey", KeyString(mapping.Values[0].Key))
}

func TestKeyStringIntegerNodeInt64(t *testing.T) {
	file, err := parser.ParseBytes([]byte("42: value\n"), parser.ParseComments)
	require.NoError(t, err)
	mapping, ok := file.Docs[0].Body.(*ast.MappingNode)
	require.True(t, ok)
	assert.Equal(t, "42", KeyString(mapping.Values[0].Key))
}

func TestKeyStringDefaultCase(t *testing.T) {
	// A boolean key would fall through to the default case
	file, err := parser.ParseBytes([]byte("true: value\n"), parser.ParseComments)
	require.NoError(t, err)
	mapping, ok := file.Docs[0].Body.(*ast.MappingNode)
	require.True(t, ok)
	key := KeyString(mapping.Values[0].Key)
	assert.NotEmpty(t, key)
}

// --- MappingValue edge cases ---

func TestMappingValueNil(t *testing.T) {
	assert.Nil(t, MappingValue(nil, "key"))
}

func TestMappingValueFound(t *testing.T) {
	file, err := parser.ParseBytes([]byte("a: 1\nb: 2\n"), parser.ParseComments)
	require.NoError(t, err)
	mapping, ok := file.Docs[0].Body.(*ast.MappingNode)
	require.True(t, ok)
	val := MappingValue(mapping, "b")
	assert.NotNil(t, val)
}

func TestMappingValueNotFound(t *testing.T) {
	file, err := parser.ParseBytes([]byte("a: 1\n"), parser.ParseComments)
	require.NoError(t, err)
	mapping, ok := file.Docs[0].Body.(*ast.MappingNode)
	require.True(t, ok)
	assert.Nil(t, MappingValue(mapping, "missing"))
}

func TestMappingValueNilEntry(t *testing.T) {
	// This tests the nil check inside the loop by injecting a nil entry
	file, err := parser.ParseBytes([]byte("a: 1\nb: 2\n"), parser.ParseComments)
	require.NoError(t, err)
	mapping, ok := file.Docs[0].Body.(*ast.MappingNode)
	require.True(t, ok)
	// Insert a nil entry between the two values
	orig := mapping.Values
	newValues := make([]*ast.MappingValueNode, 0, len(orig)+1)
	newValues = append(newValues, orig[0])
	newValues = append(newValues, nil)
	newValues = append(newValues, orig[1:]...)
	mapping.Values = newValues
	val := MappingValue(mapping, "b")
	assert.NotNil(t, val)
}

// --- Mapping edge cases ---

func TestMappingValid(t *testing.T) {
	file, err := parser.ParseBytes([]byte("key: value\n"), parser.ParseComments)
	require.NoError(t, err)
	m, ok := Mapping(file.Docs[0].Body)
	assert.True(t, ok)
	assert.NotNil(t, m)
}

func TestMappingInvalid(t *testing.T) {
	file, err := parser.ParseBytes([]byte("- item\n"), parser.ParseComments)
	require.NoError(t, err)
	_, ok := Mapping(file.Docs[0].Body)
	assert.False(t, ok)
}

func TestMappingNil(t *testing.T) {
	_, ok := Mapping(nil)
	assert.False(t, ok)
}

// --- Sequence edge cases ---

func TestSequenceValid(t *testing.T) {
	file, err := parser.ParseBytes([]byte("- item\n"), parser.ParseComments)
	require.NoError(t, err)
	seq, ok := Sequence(file.Docs[0].Body)
	assert.True(t, ok)
	assert.NotNil(t, seq)
}

func TestSequenceInvalid(t *testing.T) {
	file, err := parser.ParseBytes([]byte("key: value\n"), parser.ParseComments)
	require.NoError(t, err)
	_, ok := Sequence(file.Docs[0].Body)
	assert.False(t, ok)
}

func TestSequenceNil(t *testing.T) {
	_, ok := Sequence(nil)
	assert.False(t, ok)
}

// --- String edge cases ---

func TestStringValid(t *testing.T) {
	file, err := parser.ParseBytes([]byte("key: hello\n"), parser.ParseComments)
	require.NoError(t, err)
	mapping, ok := file.Docs[0].Body.(*ast.MappingNode)
	require.True(t, ok)
	s, ok := String(mapping.Values[0].Value)
	assert.True(t, ok)
	assert.Equal(t, "hello", s)
}

func TestStringInvalid(t *testing.T) {
	file, err := parser.ParseBytes([]byte("key: 42\n"), parser.ParseComments)
	require.NoError(t, err)
	mapping, ok := file.Docs[0].Body.(*ast.MappingNode)
	require.True(t, ok)
	_, ok = String(mapping.Values[0].Value)
	assert.False(t, ok)
}

func TestStringNil(t *testing.T) {
	s, ok := String(nil)
	assert.False(t, ok)
	assert.Empty(t, s)
}

// --- IsNullOrEmptyString edge cases ---

func TestIsNullOrEmptyStringNil(t *testing.T) {
	assert.True(t, IsNullOrEmptyString(nil))
}

func TestIsNullOrEmptyStringNull(t *testing.T) {
	file, err := parser.ParseBytes([]byte("key: null\n"), parser.ParseComments)
	require.NoError(t, err)
	mapping, ok := file.Docs[0].Body.(*ast.MappingNode)
	require.True(t, ok)
	// null value should be a NullNode
	assert.True(t, IsNullOrEmptyString(mapping.Values[0].Value))
}

func TestIsNullOrEmptyStringEmpty(t *testing.T) {
	file, err := parser.ParseBytes([]byte("key: ''\n"), parser.ParseComments)
	require.NoError(t, err)
	mapping, ok := file.Docs[0].Body.(*ast.MappingNode)
	require.True(t, ok)
	assert.True(t, IsNullOrEmptyString(mapping.Values[0].Value))
}

func TestIsNullOrEmptyStringWhitespace(t *testing.T) {
	file, err := parser.ParseBytes([]byte("key: '  '\n"), parser.ParseComments)
	require.NoError(t, err)
	mapping, ok := file.Docs[0].Body.(*ast.MappingNode)
	require.True(t, ok)
	assert.True(t, IsNullOrEmptyString(mapping.Values[0].Value))
}

func TestIsNullOrEmptyStringNonEmpty(t *testing.T) {
	file, err := parser.ParseBytes([]byte("key: value\n"), parser.ParseComments)
	require.NoError(t, err)
	mapping, ok := file.Docs[0].Body.(*ast.MappingNode)
	require.True(t, ok)
	assert.False(t, IsNullOrEmptyString(mapping.Values[0].Value))
}

func TestIsNullOrEmptyStringNonString(t *testing.T) {
	file, err := parser.ParseBytes([]byte("key: 42\n"), parser.ParseComments)
	require.NoError(t, err)
	mapping, ok := file.Docs[0].Body.(*ast.MappingNode)
	require.True(t, ok)
	// Integer node is not null, not nil, not empty string
	assert.False(t, IsNullOrEmptyString(mapping.Values[0].Value))
}

func TestKeyStringLargeNumber(t *testing.T) {
	// Try to trigger uint64 case with a very large number
	file, err := parser.ParseBytes([]byte("18446744073709551615: value\n"), parser.ParseComments)
	require.NoError(t, err)
	mapping, ok := file.Docs[0].Body.(*ast.MappingNode)
	require.True(t, ok)
	key := KeyString(mapping.Values[0].Key)
	assert.NotEmpty(t, key)
}

func TestNodeLineWithToken(t *testing.T) {
	file, err := parser.ParseBytes([]byte("key: value\n"), parser.ParseComments)
	require.NoError(t, err)
	mapping, ok := file.Docs[0].Body.(*ast.MappingNode)
	require.True(t, ok)
	line := NodeLine(mapping)
	assert.Equal(t, 1, line)
}

func TestNodeLineNodeWithNilToken(t *testing.T) {
	// A StringNode with no token set has GetToken() == nil
	node := &ast.StringNode{}
	assert.Equal(t, 0, NodeLine(node))
}

func TestKeyStringIntegerNodeNonInt64(t *testing.T) {
	// Create an IntegerNode with a non-int64/uint64 value to cover the fallback
	tk := &token.Token{
		Value:    "42",
		Position: &token.Position{Line: 1},
	}
	node := ast.Integer(tk)
	node.Value = float64(3.14) // Override to non-int64/uint64
	result := KeyString(node)
	assert.Equal(t, "3.14", result)
}
