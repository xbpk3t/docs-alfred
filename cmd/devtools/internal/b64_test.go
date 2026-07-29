package internal

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEncodeBase64(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{name: "simple", input: "hello", expected: "aGVsbG8="},
		{name: "url", input: "https://example.com", expected: "aHR0cHM6Ly9leGFtcGxlLmNvbQ=="},
		{name: "empty", input: "", expected: ""},
		{name: "chinese", input: "你好", expected: "5L2g5aW9"},
		{name: "json", input: `{"key":"value"}`, expected: "eyJrZXkiOiJ2YWx1ZSJ9"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := EncodeBase64(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestDecodeBase64(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
		err      bool
	}{
		{name: "simple", input: "aGVsbG8=", expected: "hello", err: false},
		{name: "url", input: "aHR0cHM6Ly9leGFtcGxlLmNvbQ==", expected: "https://example.com", err: false},
		{name: "chinese", input: "5L2g5aW9", expected: "你好", err: false},
		{name: "json", input: "eyJrZXkiOiJ2YWx1ZSJ9", expected: `{"key":"value"}`, err: false},
		{name: "invalid", input: "not-valid-base64!!!", expected: "", err: true},
		{name: "empty", input: "", expected: "", err: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := DecodeBase64(tt.input)
			if tt.err {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestRoundtrip(t *testing.T) {
	inputs := []string{
		"hello world",
		"https://example.com/path?q=1&r=2",
		"你好世界",
		"a\nb\nc",
		"    spaces   ",
		"special: ~!@#$%^&*()_+",
	}

	for _, input := range inputs {
		t.Run(input, func(t *testing.T) {
			encoded := EncodeBase64(input)
			decoded, err := DecodeBase64(encoded)
			require.NoError(t, err)
			assert.Equal(t, input, decoded)
		})
	}
}
