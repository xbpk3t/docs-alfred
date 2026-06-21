package parser

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

type TestConfig struct {
	Name  string `yaml:"name"`
	Value int    `yaml:"value"`
}

func readTestFile(t *testing.T, filename string) []byte {
	t.Helper()
	path := filepath.Join("testdata", filename)
	data, err := os.ReadFile(path)
	require.NoError(t, err, "failed to read test file %s", filename)

	return data
}

func TestParser(t *testing.T) {
	t.Run("single document", func(t *testing.T) {
		input := readTestFile(t, "single.yaml")
		want := TestConfig{Name: "test1", Value: 1}

		parser := NewParser[TestConfig](input)
		got, err := parser.ParseSingle()
		require.NoError(t, err)
		require.Equal(t, want, got)
	})

	t.Run("multiple documents", func(t *testing.T) {
		t.Run("multi document", func(t *testing.T) {
			input := readTestFile(t, "multi-1.yaml")
			want := []TestConfig{
				{Name: "test1", Value: 1},
				{Name: "test2", Value: 2},
			}

			parser := NewParser[TestConfig](input)
			got, err := parser.ParseMulti()
			require.NoError(t, err)
			require.Equal(t, want, got)
		})

		t.Run("blank document", func(t *testing.T) {
			input := readTestFile(t, "multi-2.yaml")
			want := []TestConfig{
				{Name: "test1", Value: 1},
			}

			parser := NewParser[TestConfig](input)
			got, err := parser.ParseMulti()
			require.NoError(t, err)
			require.Equal(t, want, got)
		})
	})
}
