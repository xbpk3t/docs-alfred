package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTargetOrDirResolution(t *testing.T) {
	t.Run("positional wins over --dir", func(t *testing.T) {
		flags := &rootFlags{dir: "/via/flag"}
		require.Equal(t, "/via/pos", targetOrDir(flags, []string{"/via/pos"}))
	})

	t.Run("--dir wins over default", func(t *testing.T) {
		flags := &rootFlags{dir: "/via/flag"}
		require.Equal(t, "/via/flag", targetOrDir(flags, nil))
	})

	t.Run("SKX_DIR overrides home default", func(t *testing.T) {
		t.Setenv("SKX_DIR", "/via/env")
		require.Equal(t, "/via/env", defaultDir())
		require.Equal(t, "/via/env", targetOrDir(&rootFlags{}, nil))
	})

	t.Run("default is deployed skill references dir", func(t *testing.T) {
		t.Setenv("SKX_DIR", "")
		home, err := os.UserHomeDir()
		require.NoError(t, err)
		require.Equal(t,
			filepath.Join(home, ".claude", "skills", "zzz", "references"),
			defaultDir())
	})
}
