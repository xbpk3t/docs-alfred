package cmd

import (
	"io"
	"os"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadPwgenConfigFromFile(t *testing.T) {
	t.Setenv("DEFAULT_PWGEN", "")
	dir := t.TempDir()
	path := dir + "/test-pwgen.yaml"
	require.NoError(t, os.WriteFile(path, []byte("secret: my-secret\nlength: 24\nuppercase: false"), 0600))

	cfg, err := loadPwgenConfig(path)
	require.NoError(t, err)
	assert.Equal(t, "my-secret", cfg.Secret)
	assert.Equal(t, 24, cfg.Length)
	assert.False(t, cfg.Uppercase)
}

func TestLoadPwgenConfigEmptyPath(t *testing.T) {
	t.Setenv("DEFAULT_PWGEN", "")
	cfg, err := loadPwgenConfig("")
	require.NoError(t, err)
	assert.Equal(t, defaultPwgenConfig(), cfg)
}

func TestLoadPwgenConfigFromEnv(t *testing.T) {
	t.Setenv("DEFAULT_PWGEN", "env-secret")

	cfg, err := loadPwgenConfig("")
	require.NoError(t, err)
	assert.Equal(t, "env-secret", cfg.Secret)
}

func TestLoadPwgenConfigDefaults(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/minimal.yaml"
	require.NoError(t, os.WriteFile(path, []byte("secret: s"), 0600))

	cfg, err := loadPwgenConfig(path)
	require.NoError(t, err)
	assert.Equal(t, 16, cfg.Length)
	assert.True(t, cfg.Uppercase)
	assert.True(t, cfg.Numbers)
	assert.False(t, cfg.Punctuation)
	assert.Equal(t, "text", cfg.Format)
}

func TestResolveConfigPathExplicit(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/custom.yaml"
	require.NoError(t, os.WriteFile(path, []byte(""), 0600))

	got := resolveConfigPath(path)
	assert.Equal(t, path, got)
}

func TestResolveConfigPathNonexistent(t *testing.T) {
	got := resolveConfigPath("/definitely-does-not-exist-pwgen-test.yaml")
	assert.Empty(t, got)
}

func TestApplyFlagOverridesSecret(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.Flags().String("secret", "", "")
	cmd.Flags().Int("length", 16, "")
	cmd.Flags().Bool("uppercase", true, "")
	cmd.Flags().Bool("numbers", true, "")
	cmd.Flags().Bool("punctuation", false, "")
	cmd.Flags().String("format", "text", "")
	require.NoError(t, cmd.ParseFlags([]string{"--secret", "flag-secret"}))

	cfg := defaultPwgenConfig()
	cfg = applyFlagOverrides(cmd, cfg, "flag-secret", 16, true, true, false, "text")

	assert.Equal(t, "flag-secret", cfg.Secret)
}

func TestApplyFlagOverridesDoesNotOverrideDefault(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.Flags().String("secret", "", "")
	cmd.Flags().Int("length", 16, "")
	cmd.Flags().Bool("uppercase", true, "")
	cmd.Flags().Bool("numbers", true, "")
	cmd.Flags().Bool("punctuation", false, "")
	cmd.Flags().String("format", "text", "")
	// No flags parsed → nothing changed

	cfg := defaultPwgenConfig()
	cfg.Secret = "from-config"
	cfg.Length = 24
	cfg = applyFlagOverrides(cmd, cfg, "", 16, true, true, false, "text")

	assert.Equal(t, "from-config", cfg.Secret)
	assert.Equal(t, 24, cfg.Length)
}

func TestRootCommandGeneratesPassword(t *testing.T) {
	stdout := captureStdout(t)
	root := newRootCmd()
	root.SetArgs([]string{"--secret", "test-secret", "--format", "text", "example.com"})
	err := root.Execute()
	require.NoError(t, err)
	got := stdout()
	assert.NotEmpty(t, got)
}

func TestRootCommandMissingSecret(t *testing.T) {
	t.Setenv("DEFAULT_PWGEN", "")
	root := newRootCmd()
	root.SetArgs([]string{"example.com"})
	err := root.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "secret")
}

func captureStdout(t *testing.T) func() string {
	t.Helper()
	original := os.Stdout
	r, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stdout = w

	return func() string {
		require.NoError(t, w.Close())
		os.Stdout = original
		data, err := io.ReadAll(r)
		require.NoError(t, err)
		require.NoError(t, r.Close())

		return string(data)
	}
}
