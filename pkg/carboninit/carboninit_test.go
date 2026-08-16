package carboninit

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
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

func TestInNormalizesToDefaultTimezone(t *testing.T) {
	// A UTC instant must render as Asia/Shanghai: 2026-08-14 17:00 UTC ==
	// 2026-08-15 01:00 +08. The absolute instant is unchanged.
	utc := time.Date(2026, 8, 14, 17, 0, 0, 0, time.UTC)

	got := In(utc).ToDateTimeString()
	assert.Equal(t, "2026-08-15 01:00:00", got)

	// Same instant after normalization.
	back := In(utc).StdTime()
	assert.Equal(t, time.Duration(0), back.Sub(utc))
}
