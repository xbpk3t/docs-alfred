package wf

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetFormatter(t *testing.T) {
	tests := []struct {
		name   string
		format string
		want   string
	}{
		{name: "alfred", format: "alfred", want: "*wf.AlfredFormatter"},
		{name: "raw", format: "raw", want: "*wf.RawFormatter"},
		{name: "rofi", format: "rofi", want: "*wf.RofiFormatter"},
		{name: "plain default", format: "plain", want: "*wf.PlainFormatter"},
		{name: "empty falls back to plain", format: "", want: "*wf.PlainFormatter"},
		{name: "unknown falls back to plain", format: "unknown", want: "*wf.PlainFormatter"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := GetFormatter(tt.format)
			require.NotNil(t, f)
			assert.Equal(t, tt.want, typeName(f))
		})
	}
}

func typeName(f Formatter) string {
	switch f.(type) {
	case *AlfredFormatter:
		return "*wf.AlfredFormatter"
	case *PlainFormatter:
		return "*wf.PlainFormatter"
	case *RawFormatter:
		return "*wf.RawFormatter"
	case *RofiFormatter:
		return "*wf.RofiFormatter"
	default:
		return "unknown"
	}
}

// --- AlfredFormatter tests ---

func TestAlfredFormatterFormatString(t *testing.T) {
	f := &AlfredFormatter{}
	out, err := f.Format("hello")
	require.NoError(t, err)

	var result AlfredOutput
	require.NoError(t, json.Unmarshal([]byte(out), &result))
	require.Len(t, result.Items, 1)
	assert.Equal(t, "hello", result.Items[0].Title)
	assert.Equal(t, "hello", result.Items[0].Arg)
	assert.True(t, result.Items[0].Valid)
}

func TestAlfredFormatterFormatItems(t *testing.T) {
	f := &AlfredFormatter{}
	items := []AlfredItem{
		{Title: "a", Arg: "1", Valid: true},
		{Title: "b", Arg: "2", Valid: false},
	}
	out, err := f.Format(items)
	require.NoError(t, err)

	var result AlfredOutput
	require.NoError(t, json.Unmarshal([]byte(out), &result))
	require.Len(t, result.Items, 2)
	assert.Equal(t, "a", result.Items[0].Title)
	assert.Equal(t, "b", result.Items[1].Title)
}

func TestAlfredFormatterFormatOutput(t *testing.T) {
	f := &AlfredFormatter{}
	output := AlfredOutput{
		Variables: map[string]string{"key": "val"},
		Items:     []AlfredItem{{Title: "x", Valid: true}},
	}
	out, err := f.Format(output)
	require.NoError(t, err)

	var result AlfredOutput
	require.NoError(t, json.Unmarshal([]byte(out), &result))
	assert.Equal(t, "val", result.Variables["key"])
	require.Len(t, result.Items, 1)
	assert.Equal(t, "x", result.Items[0].Title)
}

func TestAlfredFormatterFormatDefaultMarshalable(t *testing.T) {
	f := &AlfredFormatter{}
	out, err := f.Format(42)
	require.NoError(t, err)

	var result AlfredOutput
	require.NoError(t, json.Unmarshal([]byte(out), &result))
	require.Len(t, result.Items, 1)
	assert.Equal(t, "42", result.Items[0].Title)
	assert.False(t, result.Items[0].Valid)
}

func TestAlfredFormatterFormatDefaultStruct(t *testing.T) {
	f := &AlfredFormatter{}
	type custom struct {
		Name string `json:"name"`
	}
	out, err := f.Format(custom{Name: "test"})
	require.NoError(t, err)

	var result AlfredOutput
	require.NoError(t, json.Unmarshal([]byte(out), &result))
	require.Len(t, result.Items, 1)
	assert.Contains(t, result.Items[0].Title, "test")
	assert.False(t, result.Items[0].Valid)
}

func TestAlfredFormatterFormatEmptyString(t *testing.T) {
	f := &AlfredFormatter{}
	out, err := f.Format("")
	require.NoError(t, err)

	var result AlfredOutput
	require.NoError(t, json.Unmarshal([]byte(out), &result))
	require.Len(t, result.Items, 1)
	assert.Equal(t, "", result.Items[0].Title)
}

func TestAlfredFormatterFormatEmptyItems(t *testing.T) {
	f := &AlfredFormatter{}
	out, err := f.Format([]AlfredItem{})
	require.NoError(t, err)

	var result AlfredOutput
	require.NoError(t, json.Unmarshal([]byte(out), &result))
	assert.Empty(t, result.Items)
}

func TestAlfredFormatterFormatSpecialChars(t *testing.T) {
	f := &AlfredFormatter{}
	out, err := f.Format(`hello "world" <tag>`)
	require.NoError(t, err)

	var result AlfredOutput
	require.NoError(t, json.Unmarshal([]byte(out), &result))
	require.Len(t, result.Items, 1)
	assert.Equal(t, `hello "world" <tag>`, result.Items[0].Title)
}

// --- PlainFormatter tests ---

func TestPlainFormatterFormatString(t *testing.T) {
	f := &PlainFormatter{}
	out, err := f.Format("hello")
	require.NoError(t, err)
	assert.Equal(t, "hello", out)
}

func TestPlainFormatterFormatStringSlice(t *testing.T) {
	f := &PlainFormatter{}
	out, err := f.Format([]string{"a", "b", "c"})
	require.NoError(t, err)
	assert.Equal(t, "a\nb\nc", out)
}

func TestPlainFormatterFormatMap(t *testing.T) {
	f := &PlainFormatter{}
	out, err := f.Format(map[string]any{"key": "value"})
	require.NoError(t, err)
	assert.Contains(t, out, "key: value\n")
}

func TestPlainFormatterFormatDefault(t *testing.T) {
	f := &PlainFormatter{}
	out, err := f.Format(42)
	require.NoError(t, err)
	assert.Equal(t, "42", out)
}

func TestPlainFormatterFormatBool(t *testing.T) {
	f := &PlainFormatter{}
	out, err := f.Format(true)
	require.NoError(t, err)
	assert.Equal(t, "true", out)
}

func TestPlainFormatterFormatEmptyString(t *testing.T) {
	f := &PlainFormatter{}
	out, err := f.Format("")
	require.NoError(t, err)
	assert.Equal(t, "", out)
}

func TestPlainFormatterFormatEmptySlice(t *testing.T) {
	f := &PlainFormatter{}
	out, err := f.Format([]string{})
	require.NoError(t, err)
	assert.Equal(t, "", out)
}

// --- RawFormatter tests ---

func TestRawFormatterFormatString(t *testing.T) {
	f := &RawFormatter{}
	out, err := f.Format("hello")
	require.NoError(t, err)
	assert.Equal(t, `"hello"`, out)
}

func TestRawFormatterFormatMap(t *testing.T) {
	f := &RawFormatter{}
	out, err := f.Format(map[string]string{"a": "b"})
	require.NoError(t, err)
	assert.Contains(t, out, `"a": "b"`)
}

func TestRawFormatterFormatSlice(t *testing.T) {
	f := &RawFormatter{}
	out, err := f.Format([]int{1, 2, 3})
	require.NoError(t, err)
	assert.Contains(t, out, "1")
	assert.Contains(t, out, "2")
	assert.Contains(t, out, "3")
}

func TestRawFormatterFormatNil(t *testing.T) {
	f := &RawFormatter{}
	out, err := f.Format(nil)
	require.NoError(t, err)
	assert.Equal(t, "null", out)
}

func TestRawFormatterFormatStruct(t *testing.T) {
	type s struct {
		Name string `json:"name"`
	}
	f := &RawFormatter{}
	out, err := f.Format(s{Name: "test"})
	require.NoError(t, err)
	assert.Contains(t, out, `"name": "test"`)
}

// --- RofiFormatter tests ---

func TestRofiFormatterFormatString(t *testing.T) {
	f := &RofiFormatter{}
	out, err := f.Format("hello")
	require.NoError(t, err)
	assert.Equal(t, "hello", out)
}

func TestRofiFormatterFormatStringSlice(t *testing.T) {
	f := &RofiFormatter{}
	out, err := f.Format([]string{"a", "b", "c"})
	require.NoError(t, err)
	assert.Equal(t, "a\nb\nc", out)
}

func TestRofiFormatterFormatAlfredItems(t *testing.T) {
	f := &RofiFormatter{}
	items := []AlfredItem{
		{Title: "first"},
		{Title: "second"},
	}
	out, err := f.Format(items)
	require.NoError(t, err)
	assert.Equal(t, "first\nsecond", out)
}

func TestRofiFormatterFormatDefault(t *testing.T) {
	f := &RofiFormatter{}
	out, err := f.Format(map[string]string{"key": "value"})
	require.NoError(t, err)
	assert.Contains(t, out, `"key"`)
}

func TestRofiFormatterFormatEmptyString(t *testing.T) {
	f := &RofiFormatter{}
	out, err := f.Format("")
	require.NoError(t, err)
	assert.Equal(t, "", out)
}

func TestRofiFormatterFormatEmptySlice(t *testing.T) {
	f := &RofiFormatter{}
	out, err := f.Format([]string{})
	require.NoError(t, err)
	assert.Equal(t, "", out)
}

func TestRofiFormatterFormatEmptyAlfredItems(t *testing.T) {
	f := &RofiFormatter{}
	out, err := f.Format([]AlfredItem{})
	require.NoError(t, err)
	assert.Equal(t, "", out)
}

// --- Error path tests ---

func TestAlfredFormatterFormatUnmarshalableType(t *testing.T) {
	f := &AlfredFormatter{}
	_, err := f.Format(make(chan int))
	require.Error(t, err)
}

func TestRawFormatterFormatUnmarshalableType(t *testing.T) {
	f := &RawFormatter{}
	_, err := f.Format(make(chan int))
	require.Error(t, err)
}

func TestRofiFormatterFormatUnmarshalableType(t *testing.T) {
	f := &RofiFormatter{}
	_, err := f.Format(make(chan int))
	require.Error(t, err)
}
