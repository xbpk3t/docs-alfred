package usecase

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestResolveGhAppendDateKeepsExplicitDate(t *testing.T) {
	require.Equal(t, "2024-01-02", resolveGhAppendDate("2024-01-02"))
}

func TestResolveGhAppendDateDefaultsToToday(t *testing.T) {
	before := time.Now().Format(time.DateOnly)
	got := resolveGhAppendDate("")
	after := time.Now().Format(time.DateOnly)

	require.Contains(t, []string{before, after}, got)
}
