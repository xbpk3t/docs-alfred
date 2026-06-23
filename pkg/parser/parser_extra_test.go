package parser

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- ParseSingle tests ---

func TestParseSingleValid(t *testing.T) {
	data := []byte("name: test\nvalue: 42\n")
	p := NewParser[TestConfig](data)
	result, err := p.ParseSingle()
	require.NoError(t, err)
	assert.Equal(t, "test", result.Name)
	assert.Equal(t, 42, result.Value)
}

func TestParseSingleInvalid(t *testing.T) {
	data := []byte("{{{invalid yaml")
	p := NewParser[TestConfig](data)
	_, err := p.ParseSingle()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "解析配置失败")
}

func TestParseSingleWithFileName(t *testing.T) {
	data := []byte("{{{invalid yaml")
	p := NewParser[TestConfig](data).WithFileName("config.yaml")
	_, err := p.ParseSingle()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "config.yaml")
	assert.Contains(t, err.Error(), "解析配置失败")
}

func TestParseSingleEmpty(t *testing.T) {
	p := NewParser[TestConfig]([]byte(""))
	_, err := p.ParseSingle()
	require.Error(t, err)
}

// --- ParseMulti tests ---

func TestParseMultiValid(t *testing.T) {
	data := []byte("name: a\nvalue: 1\n---\nname: b\nvalue: 2\n")
	p := NewParser[TestConfig](data)
	results, err := p.ParseMulti()
	require.NoError(t, err)
	require.Len(t, results, 2)
	assert.Equal(t, "a", results[0].Name)
	assert.Equal(t, "b", results[1].Name)
}

func TestParseMultiSingleDoc(t *testing.T) {
	data := []byte("name: only\nvalue: 1\n")
	p := NewParser[TestConfig](data)
	results, err := p.ParseMulti()
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, "only", results[0].Name)
}

func TestParseMultiInvalid(t *testing.T) {
	data := []byte("name: a\n---\n{{{invalid")
	p := NewParser[TestConfig](data)
	_, err := p.ParseMulti()
	require.Error(t, err)
}

func TestParseMultiEmpty(t *testing.T) {
	p := NewParser[TestConfig]([]byte(""))
	results, err := p.ParseMulti()
	require.NoError(t, err)
	assert.Empty(t, results)
}

// --- ParseFlatten tests ---

func TestParseFlattenValid(t *testing.T) {
	data := []byte("- name: a\n  value: 1\n- name: b\n  value: 2\n")
	p := NewParser[TestConfig](data)
	results, err := p.ParseFlatten()
	require.NoError(t, err)
	require.Len(t, results, 2)
	assert.Equal(t, "a", results[0].Name)
	assert.Equal(t, "b", results[1].Name)
}

func TestParseFlattenMultiDoc(t *testing.T) {
	data := []byte("- name: a\n  value: 1\n---\n- name: b\n  value: 2\n")
	p := NewParser[TestConfig](data)
	results, err := p.ParseFlatten()
	require.NoError(t, err)
	require.Len(t, results, 2)
	assert.Equal(t, "a", results[0].Name)
	assert.Equal(t, "b", results[1].Name)
}

func TestParseFlattenInvalid(t *testing.T) {
	data := []byte("- name: a\n---\n{{{invalid")
	p := NewParser[TestConfig](data)
	_, err := p.ParseFlatten()
	require.Error(t, err)
}

func TestParseFlattenEmpty(t *testing.T) {
	p := NewParser[TestConfig]([]byte(""))
	results, err := p.ParseFlatten()
	require.NoError(t, err)
	assert.Empty(t, results)
}

// --- WithFileName tests ---

func TestWithFileNameReturnsSelf(t *testing.T) {
	p := NewParser[TestConfig]([]byte(""))
	ret := p.WithFileName("test.yaml")
	assert.Same(t, p, ret)
}

// --- NewParser tests ---

func TestNewParserNotNil(t *testing.T) {
	p := NewParser[TestConfig]([]byte("name: test\n"))
	assert.NotNil(t, p)
}
