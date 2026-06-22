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
