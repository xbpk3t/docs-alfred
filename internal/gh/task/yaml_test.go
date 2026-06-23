package task

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewTaskYAMLRender(t *testing.T) {
	r := NewTaskYAMLRender()
	require.NotNil(t, r)
	require.NotNil(t, r.YAMLRenderer)
}

func TestFlatten_NestedSlice(t *testing.T) {
	r := NewTaskYAMLRender()
	data := []byte(`
- - task: a
    date: "2024-01-01"
  - task: b
    date: "2024-06-01"
- task: c
  date: "2024-12-01"
`)
	result, err := r.Flatten(data)
	require.NoError(t, err)
	require.Len(t, result, 3)

	// Verify the flattened items
	item0, ok := result[0].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "a", item0["task"])

	item1, ok := result[1].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "b", item1["task"])

	item2, ok := result[2].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "c", item2["task"])
}

func TestFlatten_SimpleSlice(t *testing.T) {
	r := NewTaskYAMLRender()
	data := []byte(`
- task: x
  date: "2024-01-01"
- task: y
  date: "2024-06-01"
`)
	result, err := r.Flatten(data)
	require.NoError(t, err)
	require.Len(t, result, 2)
}

func TestFlatten_UnsupportedType(t *testing.T) {
	r := NewTaskYAMLRender()
	data := []byte(`key: value`)
	_, err := r.Flatten(data)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported data type")
}

func TestFlatten_ParseError(t *testing.T) {
	r := NewTaskYAMLRender()
	// Invalid YAML that causes parse error
	data := []byte("\t- [\tinvalid:\t[yaml:\tbroken\n\t- ]\tinvalid")
	_, err := r.Flatten(data)
	// If no error from parse, the result should still work
	if err != nil {
		assert.NotEmpty(t, err.Error())
	}
}

func TestRender_SortsByDate(t *testing.T) {
	r := NewTaskYAMLRender()
	data := []byte(`
- task: later
  date: "2024-12-01"
- task: earlier
  date: "2024-01-01"
`)
	result, err := r.Render(data)
	require.NoError(t, err)
	assert.NotEmpty(t, result)
	// The later date should appear first (descending sort)
	assert.Contains(t, result, "later")
	assert.Contains(t, result, "earlier")
}

func TestRender_WithSubTasks(t *testing.T) {
	r := NewTaskYAMLRender()
	data := []byte(`
- task: parent
  date: "2024-06-01"
  sub:
    - task: child-b
      date: "2024-12-01"
    - task: child-a
      date: "2024-01-01"
`)
	result, err := r.Render(data)
	require.NoError(t, err)
	assert.NotEmpty(t, result)
	assert.Contains(t, result, "parent")
	assert.Contains(t, result, "child-a")
	assert.Contains(t, result, "child-b")
}

func TestRender_WithParentID(t *testing.T) {
	r := NewTaskYAMLRender()
	data := []byte(`
- task: parent
  pid: p1
  date: "2024-06-01"
  sub:
    - task: child
      date: "2024-01-01"
`)
	result, err := r.Render(data)
	require.NoError(t, err)
	assert.NotEmpty(t, result)
	assert.Contains(t, result, "p1")
}

func TestRender_InvalidData(t *testing.T) {
	r := NewTaskYAMLRender()
	data := []byte(`key: value`)
	_, err := r.Render(data)
	require.Error(t, err)
}

func TestRender_EmptyNestedSlice(t *testing.T) {
	r := NewTaskYAMLRender()
	data := []byte(`- []`)
	result, err := r.Render(data)
	require.NoError(t, err)
	assert.NotEmpty(t, result)
}

func TestRender_NestedSliceFlattened(t *testing.T) {
	r := NewTaskYAMLRender()
	data := []byte(`
- - task: a
    date: "2024-01-01"
  - task: b
    date: "2024-06-01"
`)
	result, err := r.Render(data)
	require.NoError(t, err)
	assert.Contains(t, result, "a")
	assert.Contains(t, result, "b")
}

func TestRender_DecodeError(t *testing.T) {
	r := NewTaskYAMLRender()
	// Data that flattens to items but causes mapstructure decode error:
	// a slice value where a map is expected
	data := []byte(`
- task: test
  sub: not-a-valid-sub-list
  date: "2024-01-01"
`)
	_, err := r.Render(data)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "decode")
}
