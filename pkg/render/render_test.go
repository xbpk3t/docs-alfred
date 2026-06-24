package render

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- YAMLRenderer tests ---

func TestNewYAMLRenderer(t *testing.T) {
	r := NewYAMLRenderer("test-cmd", true)
	assert.Equal(t, "test-cmd", r.Cmd)
	assert.True(t, r.PrettyPrint)
	assert.Equal(t, ParseSingle, r.ParseMode)
}

func TestNewYAMLRendererDefaults(t *testing.T) {
	r := NewYAMLRenderer("", false)
	assert.Empty(t, r.Cmd)
	assert.False(t, r.PrettyPrint)
	assert.Equal(t, ParseSingle, r.ParseMode)
}

func TestYAMLRendererWithParseMode(t *testing.T) {
	r := NewYAMLRenderer("", false)
	r.WithParseMode(ParseMulti)
	assert.Equal(t, ParseMulti, r.ParseMode)
}

func TestYAMLRendererWithParseModeFlatten(t *testing.T) {
	r := NewYAMLRenderer("", false)
	r.WithParseMode(ParseFlatten)
	assert.Equal(t, ParseFlatten, r.ParseMode)
}

func TestYAMLRendererRenderSingle(t *testing.T) {
	r := NewYAMLRenderer("", false)
	data := []byte(`name: test
value: 42
`)
	result, err := r.Render(data)
	require.NoError(t, err)
	assert.Contains(t, result, "name: test")
	assert.Contains(t, result, "value: 42")
}

func TestYAMLRendererRenderMulti(t *testing.T) {
	r := NewYAMLRenderer("", false)
	r.WithParseMode(ParseMulti)
	data := []byte(`name: first
---
name: second
`)
	result, err := r.Render(data)
	require.NoError(t, err)
	assert.Contains(t, result, "first")
	assert.Contains(t, result, "second")
}

func TestYAMLRendererRenderFlatten(t *testing.T) {
	r := NewYAMLRenderer("", false)
	r.WithParseMode(ParseFlatten)
	data := []byte(`- name: a
- name: b
---
- name: c
`)
	result, err := r.Render(data)
	require.NoError(t, err)
	assert.Contains(t, result, "a")
	assert.Contains(t, result, "b")
	assert.Contains(t, result, "c")
}

func TestYAMLRendererRenderInvalidYAML(t *testing.T) {
	r := NewYAMLRenderer("", false)
	data := []byte(`{{{invalid yaml`)
	_, err := r.Render(data)
	require.Error(t, err)
}

func TestYAMLRendererParseDataSingle(t *testing.T) {
	r := NewYAMLRenderer("", false)
	data := []byte(`key: value`)
	result, err := r.ParseData(data)
	require.NoError(t, err)
	m, ok := result.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "value", m["key"])
}

func TestYAMLRendererParseDataMulti(t *testing.T) {
	r := NewYAMLRenderer("", false)
	r.WithParseMode(ParseMulti)
	data := []byte(`a: 1
---
b: 2
`)
	result, err := r.ParseData(data)
	require.NoError(t, err)
	arr, ok := result.([]any)
	require.True(t, ok)
	require.Len(t, arr, 2)
	m0, ok := arr[0].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, uint64(1), m0["a"])
	m1, ok := arr[1].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, uint64(2), m1["b"])
}

func TestYAMLRendererParseDataFlatten(t *testing.T) {
	r := NewYAMLRenderer("", false)
	r.WithParseMode(ParseFlatten)
	data := []byte(`- x: 1
- x: 2
`)
	result, err := r.ParseData(data)
	require.NoError(t, err)
	arr, ok := result.([]any)
	require.True(t, ok)
	require.Len(t, arr, 2)
}

func TestYAMLRendererParseDataInvalid(t *testing.T) {
	r := NewYAMLRenderer("", false)
	_, err := r.ParseData([]byte(`{{{bad`))
	require.Error(t, err)
}

func TestYAMLRendererParseDataMultiInvalid(t *testing.T) {
	r := NewYAMLRenderer("", false)
	r.WithParseMode(ParseMulti)
	_, err := r.ParseData([]byte(`{{{bad`))
	require.Error(t, err)
}

func TestYAMLRendererParseDataFlattenInvalid(t *testing.T) {
	r := NewYAMLRenderer("", false)
	r.WithParseMode(ParseFlatten)
	_, err := r.ParseData([]byte(`{{{bad`))
	require.Error(t, err)
}

// Note: The yaml.Marshal error path in Render() is practically unreachable
// because goccy/go-yaml Marshal only fails on unserializable types (func, chan),
// but ParseData returns only map/slice primitives from parsed YAML.

func TestYAMLRendererRenderWithCmd(t *testing.T) {
	r := NewYAMLRenderer("my-cmd", false)
	assert.Equal(t, "my-cmd", r.Cmd)
	data := []byte(`key: value`)
	result, err := r.Render(data)
	require.NoError(t, err)
	assert.Contains(t, result, "key: value")
}

func TestYAMLRendererRenderPrettyPrint(t *testing.T) {
	r := NewYAMLRenderer("", true)
	assert.True(t, r.PrettyPrint)
	data := []byte(`a: 1`)
	result, err := r.Render(data)
	require.NoError(t, err)
	assert.Contains(t, result, "a: 1")
}

func TestYAMLRendererRenderEmptyData(t *testing.T) {
	r := NewYAMLRenderer("", false)
	data := []byte(``)
	_, err := r.Render(data)
	require.Error(t, err)
}

func TestYAMLRendererRenderSingleMap(t *testing.T) {
	r := NewYAMLRenderer("", false)
	data := []byte(`str: hello
num: 42
bool: true
list:
  - a
  - b
nested:
  key: val
`)
	result, err := r.Render(data)
	require.NoError(t, err)
	assert.Contains(t, result, "str: hello")
	assert.Contains(t, result, "num: 42")
	assert.Contains(t, result, "bool: true")
	assert.Contains(t, result, "a")
	assert.Contains(t, result, "b")
	assert.Contains(t, result, "key: val")
}

func TestYAMLRendererRenderMultiDocument(t *testing.T) {
	r := NewYAMLRenderer("", false)
	r.WithParseMode(ParseMulti)
	data := []byte(`a: 1
---
b: 2
---
c: 3
`)
	result, err := r.Render(data)
	require.NoError(t, err)
	assert.Contains(t, result, "a: 1")
	assert.Contains(t, result, "b: 2")
	assert.Contains(t, result, "c: 3")
}

// --- ParseMode constants ---

func TestParseModeConstants(t *testing.T) {
	assert.Equal(t, ParseModeInvalid, ParseMode(0))
	assert.Equal(t, ParseSingle, ParseMode(1))
	assert.Equal(t, ParseMulti, ParseMode(2))
	assert.Equal(t, ParseFlatten, ParseMode(3))
}

// --- Renderer interface ---

func TestYAMLRendererImplementsRenderer(t *testing.T) {
	var _ Renderer = &YAMLRenderer{}
}

// Note: The yaml.Marshal error path in Render() (yaml.go:48) is practically
// unreachable because goccy/go-yaml's Marshal only fails on unserializable
// types (func, chan), but ParseData returns only map/slice primitives from
// parsed YAML. Therefore the remaining ~7% uncovered is this single branch.
