package domrules

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSpecForDomain(t *testing.T) {
	spec, ok := SpecForDomain(DomainGH)
	require.True(t, ok)
	require.Equal(t, "data/gh", spec.DefaultPath)
	require.True(t, spec.DuplicateCheck)
	require.False(t, spec.StructuredCheck)

	spec, ok = SpecForDomain(DomainBooks)
	require.True(t, ok)
	require.Equal(t, ScopeBooks, spec.RuleScope)
	require.True(t, spec.StructuredCheck)
	require.True(t, spec.DuplicateCheck)

	spec, ok = SpecForDomain(DomainTask)
	require.True(t, ok)
	require.Equal(t, "data", spec.DefaultPath)
	require.True(t, spec.YAMLParseOnly)
	require.False(t, spec.DuplicateCheck)
}

func TestSpecForDomainUnknown(t *testing.T) {
	_, ok := SpecForDomain(DataDomain("unknown"))
	require.False(t, ok)
}

func TestSpecForDomainGoods(t *testing.T) {
	spec, ok := SpecForDomain(DomainGoods)
	require.True(t, ok)
	require.Equal(t, "data/goods", spec.DefaultPath)
	require.Equal(t, ScopeGoods, spec.RuleScope)
	require.True(t, spec.StructuredCheck)
	require.False(t, spec.YAMLParseOnly)
}

func TestResolveScopeGoods(t *testing.T) {
	require.Equal(t, ScopeGoods, ResolveScope("data/goods/goods.EDC.yml", ""))
	require.Equal(t, ScopeGoods, ResolveScope("data/goods/goods.food.yml", ""))
	require.Equal(t, ScopeGoods, ResolveScope("data/goods/goods.durs.yml", ""))
	require.Equal(t, ScopeGoods, ResolveScope("any.yml", "goods"))
	require.Equal(t, ScopeBooks, ResolveScope("data/goods/other.yml", ""), "non-goods-prefixed file stays books scope")
}

func TestAllowedFieldsForScopeGoods(t *testing.T) {
	fields := AllowedFieldsForScope(ScopeGoods)
	for _, f := range []string{"type", "tag", "topics", "topic", "table", "score", "name", "brand", "param", "price", "date", "endDate", "endPrice", "isUsing"} {
		assert.True(t, fields[f], "GoodsFields should allow %q", f)
	}
	assert.False(t, fields["author"], "GoodsFields should not allow author")
}

func TestDefaultPathForDomain(t *testing.T) {
	tests := []struct {
		domain DataDomain
		path   string
	}{
		{DomainBooks, "data/books"},
		{DomainMovie, "data/books"},
		{DomainTV, "data/books"},
		{DomainMusic, "data/music"},
		{DomainDiary, "data/diary"},
		{DomainGH, "data/gh"},
		{DomainGoods, "data/goods"},
		{DomainTask, "data"},
		{DomainNtl, "data/.archive/ntl"},
	}
	for _, tt := range tests {
		t.Run(string(tt.domain), func(t *testing.T) {
			assert.Equal(t, tt.path, DefaultPathForDomain(tt.domain))
		})
	}
}

func TestDefaultPathForDomain_Unknown(t *testing.T) {
	assert.Empty(t, DefaultPathForDomain(DataDomain("unknown")))
}
