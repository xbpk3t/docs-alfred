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
