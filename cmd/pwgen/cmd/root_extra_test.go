package cmd

import (
	"io"
	"os"
	"testing"

	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveSecretFromFlag(t *testing.T) {
	k := koanf.New(".")
	got := resolveSecret("flag-secret", k)
	assert.Equal(t, "flag-secret", got)
}

func TestResolveSecretFromConfig(t *testing.T) {
	k := koanf.New(".")
	require.NoError(t, k.Load(bytesProvider([]byte("secret: config-secret")), yaml.Parser()))
	got := resolveSecret("", k)
	assert.Equal(t, "config-secret", got)
}

func TestResolveSecretFromEnv(t *testing.T) {
	t.Setenv("DEFAULT_PWGEN", "env-secret")
	k := koanf.New(".")
	got := resolveSecret("", k)
	assert.Equal(t, "env-secret", got)
}

func TestResolveSecretEmpty(t *testing.T) {
	t.Setenv("DEFAULT_PWGEN", "")
	k := koanf.New(".")
	got := resolveSecret("", k)
	assert.Empty(t, got)
}

func TestResolveSecretFlagWinsOverConfig(t *testing.T) {
	k := koanf.New(".")
	require.NoError(t, k.Load(bytesProvider([]byte("secret: config-secret")), yaml.Parser()))
	got := resolveSecret("flag-secret", k)
	assert.Equal(t, "flag-secret", got)
}

type bytesProvider []byte

func (p bytesProvider) ReadBytes() ([]byte, error)    { return p, nil }
func (p bytesProvider) Read() (map[string]any, error) { return nil, nil }

func TestApplyBoolCfgWithChangedFlag(t *testing.T) {
	cmd := newRootCmd()
	cmd.SetArgs([]string{"--uppercase=false", "example.com"})
	// Parse flags
	require.NoError(t, cmd.ParseFlags([]string{"--uppercase=false"}))
	cfg := koanf.New(".")
	dest := true
	applyBoolCfg(cmd, cfg, "uppercase", &dest)
	// Flag was changed, so applyBoolCfg should not override
	assert.True(t, dest)
}

func TestApplyBoolCfgWithConfigValue(t *testing.T) {
	cmd := newRootCmd()
	// Don't change any flags
	cfg := koanf.New(".")
	require.NoError(t, cfg.Load(bytesProvider([]byte("punctuation: true")), yaml.Parser()))
	dest := false
	applyBoolCfg(cmd, cfg, "punctuation", &dest)
	assert.True(t, dest)
}

func TestLoadConfigNonexistentFile(t *testing.T) {
	k := loadConfig("/nonexistent/path/.pwgen.yaml")
	assert.NotNil(t, k)
	assert.Empty(t, k.Keys())
}

func TestLoadConfigWithExplicitFile(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/test-pwgen.yaml"
	require.NoError(t, os.WriteFile(path, []byte("secret: my-secret\nlength: 24"), 0600))
	k := loadConfig(path)
	assert.Equal(t, "my-secret", k.String("secret"))
	assert.Equal(t, 24, k.Int("length"))
}

func TestLoadConfigEmptyPath(t *testing.T) {
	// This will try to load $HOME/.pwgen.yaml which may not exist
	k := loadConfig("")
	assert.NotNil(t, k)
}

func TestRootCommandGeneratesPassword(t *testing.T) {
	stdout := captureStdout(t)
	root := newRootCmd()
	root.SetArgs([]string{"--secret", "test-secret", "--output", "plain", "example.com"})
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
