package schema

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/xbpk3t/docs-alfred/pkg/schemacheck"
)

func TestGhEmbeddedSchema_ValidJSON(t *testing.T) {
	var doc any
	require.NoError(t, json.Unmarshal(Gh, &doc))
}

func TestGhEmbeddedSchema_Compiles(t *testing.T) {
	_, err := schemacheck.CompileBytes(Gh)
	require.NoError(t, err)
}

func TestGoodsEmbeddedSchema_ValidJSON(t *testing.T) {
	var doc any
	require.NoError(t, json.Unmarshal(Goods, &doc))
}

func TestGoodsEmbeddedSchema_Compiles(t *testing.T) {
	_, err := schemacheck.CompileBytes(Goods)
	require.NoError(t, err)
}

func TestDiaryEmbeddedSchema_ValidJSON(t *testing.T) {
	var doc any
	require.NoError(t, json.Unmarshal(Diary, &doc))
}

func TestDiaryEmbeddedSchema_Compiles(t *testing.T) {
	_, err := schemacheck.CompileBytes(Diary)
	require.NoError(t, err)
}

func TestDiarySchema(t *testing.T) {
	sch, err := schemacheck.CompileBytes(Diary)
	require.NoError(t, err)

	tests := []struct {
		name    string
		doc     any
		wantErr bool
	}{
		{"date only", []any{map[string]any{"date": "2024-01-01"}}, false},
		{"full", []any{map[string]any{
			"date": "2024-01-01", "des": "hello", "score": 4, "url": "https://example.com",
		}}, false},
		{"null des", []any{map[string]any{"date": "2024-01-01", "des": nil}}, false},
		{"missing date", []any{map[string]any{"des": "hello"}}, true},
		{"bad date", []any{map[string]any{"date": "2024"}}, true},
		{"unknown field", []any{map[string]any{"date": "2024-01-01", "review": "x"}}, true},
		{"score out of range", []any{map[string]any{"date": "2024-01-01", "score": 6}}, true},
		{"bad url", []any{map[string]any{"date": "2024-01-01", "url": "ftp://example.com"}}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msgs := schemacheck.Validate(sch, tt.doc)
			if tt.wantErr {
				require.NotEmpty(t, msgs)
			} else {
				require.Empty(t, msgs)
			}
		})
	}
}
