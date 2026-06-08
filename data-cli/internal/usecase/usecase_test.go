package usecase

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/xbpk3t/docs-alfred/pkg/checkutil"
	"github.com/xbpk3t/docs-alfred/service/data"
)

func TestRunDomainCheckPassesGhMaxLinesOverride(t *testing.T) {
	tmpDir := t.TempDir()
	content := strings.Repeat("# filler\n", 1000) + "- type: go\n  record: []\n"
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "go.yml"), []byte(content), 0644))

	defaultResult, err := RunDomainCheck(DomainCheckInput{Domain: data.DomainGH, Path: tmpDir})
	require.NoError(t, err)
	require.True(t, checkutil.HasErrors(defaultResult.Issues))

	overrideResult, err := RunDomainCheck(DomainCheckInput{
		Domain:     data.DomainGH,
		Path:       tmpDir,
		GhMaxLines: 1500,
	})
	require.NoError(t, err)
	require.False(t, checkutil.HasErrors(overrideResult.Issues))
}

func TestResolveGhAppendDateKeepsExplicitDate(t *testing.T) {
	require.Equal(t, "2024-01-02", resolveGhAppendDate("2024-01-02"))
}

func TestResolveGhAppendDateDefaultsToToday(t *testing.T) {
	before := time.Now().Format(time.DateOnly)
	got := resolveGhAppendDate("")
	after := time.Now().Format(time.DateOnly)

	require.Contains(t, []string{before, after}, got)
}
