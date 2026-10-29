package domrules

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSpecForDomain(t *testing.T) {
	spec, ok := SpecForDomain(DomainGH)
	require.True(t, ok)
	require.Equal(t, "data/gh", spec.DefaultPath)
	require.Equal(t, DupGH, spec.Duplicate)
	require.Equal(t, CheckWalker, spec.Check)

	spec, ok = SpecForDomain(DomainBooks)
	require.True(t, ok)
	require.Empty(t, spec.RuleScope)
	require.Equal(t, CheckBooksSchema, spec.Check)
	require.Equal(t, DupGeneric, spec.Duplicate)

	spec, ok = SpecForDomain(DomainTask)
	require.True(t, ok)
	require.Equal(t, "data", spec.DefaultPath)
	require.Equal(t, CheckYAMLParseOnly, spec.Check)
	require.Equal(t, DupNone, spec.Duplicate)
}

func TestSpecForDomainUnknown(t *testing.T) {
	_, ok := SpecForDomain(DataDomain("unknown"))
	require.False(t, ok)
}

func TestSpecForDomainGoods(t *testing.T) {
	// goods is validated by its embedded JSON Schema, not the structured check;
	// it must not declare a rule scope or structured check.
	spec, ok := SpecForDomain(DomainGoods)
	require.True(t, ok)
	require.Equal(t, "data/goods", spec.DefaultPath)
	require.Empty(t, spec.RuleScope)
	require.Equal(t, CheckGoodsSchema, spec.Check)
	require.Equal(t, DupNone, spec.Duplicate)
}

func TestSpecRenderAndFormat(t *testing.T) {
	tests := []struct {
		domain  DataDomain
		render  RenderStrategy
		format  string
		catalog bool
	}{
		{DomainGH, RenderGH, "json,yaml", false},
		{DomainGoods, RenderGoods, "json", true},
		{DomainBooks, RenderGeneric, "json", true},
		{DomainNtl, RenderGeneric, "json", true},
		{DomainTask, RenderGeneric, "yaml", false},
		{DomainDiary, RenderGeneric, "yaml", false},
	}
	for _, tt := range tests {
		spec, ok := SpecForDomain(tt.domain)
		require.True(t, ok, tt.domain)
		require.Equal(t, tt.render, spec.Render, tt.domain)
		require.Equal(t, tt.format, spec.DefaultFormat, tt.domain)
		require.Equal(t, tt.catalog, spec.Catalog, tt.domain)
	}
}
