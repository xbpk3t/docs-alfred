package domrules

import (
	"testing"

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
	require.Empty(t, spec.RuleScope)
	require.False(t, spec.StructuredCheck)
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
	// goods is validated by its embedded JSON Schema, not the structured check;
	// it must not declare a rule scope or structured check.
	spec, ok := SpecForDomain(DomainGoods)
	require.True(t, ok)
	require.Equal(t, "data/goods", spec.DefaultPath)
	require.Empty(t, spec.RuleScope)
	require.False(t, spec.StructuredCheck)
	require.False(t, spec.YAMLParseOnly)
}
