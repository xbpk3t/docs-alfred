package parser

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

type TestConfig struct {
	Name  string `yaml:"name"`
	Value int    `yaml:"value"`
}

func readTestFile(t *testing.T, filename string) []byte {
	t.Helper()
	path := filepath.Join("testdata", filename)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read test file %s: %v", filename, err)
	}
	return data
}

func TestParser(t *testing.T) {
	t.Run("test single document", func(t *testing.T) {
		input := readTestFile(t, "single.yaml")
		want := TestConfig{Name: "test1", Value: 1}

		parser := NewParser[TestConfig](input)
		got, err := parser.ParseSingle()
		if err != nil {
			t.Errorf("ParseSingle() error = %v", err)
			return
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("ParseSingle() = %v, want %v", got, want)
		}
	})

	t.Run("test multiple documents", func(t *testing.T) {
		input := readTestFile(t, "multi.yaml")
		want := []TestConfig{
			{Name: "test1", Value: 1},
			{Name: "test2", Value: 2},
		}

		parser := NewParser[TestConfig](input)
		got, err := parser.ParseMulti()
		if err != nil {
			t.Errorf("ParseMulti() error = %v", err)
			return
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("ParseMulti() = %v, want %v", got, want)
		}
	})
}
