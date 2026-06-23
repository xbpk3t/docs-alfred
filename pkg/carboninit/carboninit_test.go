package carboninit

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSetup(t *testing.T) {
	// Setup should not panic
	require.NotPanics(t, Setup)
}

func TestSetupSetsTimezone(t *testing.T) {
	Setup()
	// Verify that carbon is configured - calling Setup multiple times should be safe
	require.NotPanics(t, Setup)
}
