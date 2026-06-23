package cmd

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveConfigDefaultsConfigValuesNotChanged(t *testing.T) {
	cmd := newRootCmd()
	// Don't call SetArgs or ParseFlags so flags are not "Changed"

	cfg := koanf.New(".")
	require.NoError(t, cfg.Load(bytesProvider([]byte("length: 32\noutput: alfred\nuppercase: false\nnumbers: false\npunctuation: true")), yaml.Parser()))

	length := 16
	uppercase := true
	numbers := true
	punctuation := false
	outputFormat := "plain"

	resolveConfigDefaults(cmd, cfg, &length, &uppercase, &numbers, &punctuation, &outputFormat)

	assert.Equal(t, 32, length, "length should be from config")
	assert.Equal(t, "alfred", outputFormat, "output should be from config")
	assert.True(t, punctuation, "punctuation should be true from config")
	// applyBoolCfg only sets to true, never false, so uppercase/numbers stay as-is
	assert.True(t, uppercase, "applyBoolCfg only sets true, does not override to false")
	assert.True(t, numbers, "applyBoolCfg only sets true, does not override to false")
}

func TestResolveConfigDefaultsConfigLengthZeroIgnored(t *testing.T) {
	cmd := newRootCmd()

	cfg := koanf.New(".")
	require.NoError(t, cfg.Load(bytesProvider([]byte("length: 0")), yaml.Parser()))

	length := 16
	uppercase := true
	numbers := true
	punctuation := false
	outputFormat := "plain"

	resolveConfigDefaults(cmd, cfg, &length, &uppercase, &numbers, &punctuation, &outputFormat)

	assert.Equal(t, 16, length, "zero config length should not override default")
}

func TestResolveConfigDefaultsConfigOutputEmptyIgnored(t *testing.T) {
	cmd := newRootCmd()

	cfg := koanf.New(".")
	require.NoError(t, cfg.Load(bytesProvider([]byte("output: \"\"")), yaml.Parser()))

	length := 16
	uppercase := true
	numbers := true
	punctuation := false
	outputFormat := "plain"

	resolveConfigDefaults(cmd, cfg, &length, &uppercase, &numbers, &punctuation, &outputFormat)

	assert.Equal(t, "plain", outputFormat, "empty config output should not override default")
}

func TestResolveConfigDefaultsFlagsChangedPreventOverride(t *testing.T) {
	cmd := newRootCmd()
	require.NoError(t, cmd.ParseFlags([]string{"--length", "24", "--output", "raw"}))

	cfg := koanf.New(".")
	require.NoError(t, cfg.Load(bytesProvider([]byte("length: 32\noutput: alfred")), yaml.Parser()))

	length := 24
	uppercase := true
	numbers := true
	punctuation := false
	outputFormat := "raw"

	resolveConfigDefaults(cmd, cfg, &length, &uppercase, &numbers, &punctuation, &outputFormat)

	assert.Equal(t, 24, length, "flag-changed length should not be overridden")
	assert.Equal(t, "raw", outputFormat, "flag-changed output should not be overridden")
}

func TestLoadConfigInvalidYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.yaml")
	require.NoError(t, os.WriteFile(path, []byte("secret: [invalid yaml: {{"), 0600))

	// Should not panic; prints warning and returns empty koanf
	stderr := captureStderr(t)
	k := loadConfig(path)
	_ = stderr() // consume the captured stderr
	assert.NotNil(t, k)
}

func TestLoadConfigDefaultPathMissing(t *testing.T) {
	// Explicitly use a path that does not exist
	k := loadConfig("/tmp/definitely-does-not-exist-pwgen-test.yaml")
	assert.NotNil(t, k)
	assert.Empty(t, k.Keys())
}

func TestNewRootCmdAllFlagsSet(t *testing.T) {
	stdout := captureStdout(t)
	root := newRootCmd()
	root.SetArgs([]string{
		"--secret", "my-secret",
		"--length", "20",
		"--uppercase=false",
		"--numbers=false",
		"--punctuation=true",
		"--output", "raw",
		"testsite.com",
	})
	err := root.Execute()
	require.NoError(t, err)
	out := stdout()
	assert.NotEmpty(t, out)
}

func TestNewRootCmdConfigFileDefaults(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "pwgen.yaml")
	require.NoError(t, os.WriteFile(cfgPath, []byte(
		"secret: config-secret\nlength: 24\noutput: raw\nuppercase: true\nnumbers: true\npunctuation: true\n",
	), 0600))

	stdout := captureStdout(t)
	root := newRootCmd()
	root.SetArgs([]string{"--config", cfgPath, "example.com"})
	err := root.Execute()
	require.NoError(t, err)
	out := stdout()
	assert.NotEmpty(t, out)
}

func TestExecuteSuccess(t *testing.T) {
	t.Setenv("DEFAULT_PWGEN", "test-secret-for-execute")
	// Capture stdout so output doesn't pollute test output.
	old := os.Stdout
	r, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stdout = w

	// We need to set args for the default command
	origArgs := os.Args
	os.Args = []string{"pwgen", "--output", "plain", "test.example.com"}

	// Execute should not panic in the success case
	assert.NotPanics(t, func() {
		Execute()
	})

	require.NoError(t, w.Close())
	os.Stdout = old
	os.Args = origArgs
	data, _ := io.ReadAll(r)
	require.NoError(t, r.Close())
	assert.NotEmpty(t, string(data))
}

// TestExecuteErrorPath uses a subprocess to test the os.Exit(1) path in Execute().
// This is the standard Go pattern for testing functions that call os.Exit.
func TestExecuteErrorPath(t *testing.T) {
	if os.Getenv("TEST_EXECUTE_ERROR") == "1" {
		// We're in the subprocess; set args to trigger error (no website arg).
		os.Args = []string{"pwgen"}
		Execute()
		return
	}

	// Run the test binary as a subprocess, requesting only this test.
	cmd := exec.Command(os.Args[0], "-test.run=TestExecuteErrorPath", "-test.count=1")
	cmd.Env = append(os.Environ(), "TEST_EXECUTE_ERROR=1")

	err := cmd.Run()
	require.Error(t, err, "subprocess should exit with error")

	var exitErr *exec.ExitError
	require.ErrorAs(t, err, &exitErr)
	assert.False(t, exitErr.Success(), "should exit with non-zero code")
}

// TestLoadConfigEmptyPathDefault tests loadConfig with empty path which attempts
// to load $HOME/.pwgen.yaml. We set HOME to a temp dir to control the outcome.
func TestLoadConfigEmptyPathDefault(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, ".pwgen.yaml")
	require.NoError(t, os.WriteFile(cfgPath, []byte("secret: home-secret\nlength: 20"), 0600))

	origHome := os.Getenv("HOME")
	t.Setenv("HOME", dir)
	t.Cleanup(func() { os.Setenv("HOME", origHome) })

	k := loadConfig("")
	assert.Equal(t, "home-secret", k.String("secret"))
	assert.Equal(t, 20, k.Int("length"))
}

// TestLoadConfigEmptyPathNoFile tests loadConfig with empty path when
// $HOME/.pwgen.yaml does not exist.
func TestLoadConfigEmptyPathNoFile(t *testing.T) {
	dir := t.TempDir()

	origHome := os.Getenv("HOME")
	t.Setenv("HOME", dir)
	t.Cleanup(func() { os.Setenv("HOME", origHome) })

	k := loadConfig("")
	assert.NotNil(t, k)
	assert.Empty(t, k.Keys())
}

// TestLoadConfigEmptyPathHomeDirError tests loadConfig with empty path
// when os.UserHomeDir() returns an error (HOME unset).
func TestLoadConfigEmptyPathHomeDirError(t *testing.T) {
	// On some systems, unsetting HOME makes UserHomeDir fail.
	// On others (macOS), it may fall back to a system call.
	// We test the code path by unsetting HOME.
	t.Setenv("HOME", "")
	// This should not panic regardless
	k := loadConfig("")
	assert.NotNil(t, k)
}

func captureStderr(t *testing.T) func() string {
	t.Helper()
	original := os.Stderr
	r, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stderr = w
	return func() string {
		require.NoError(t, w.Close())
		os.Stderr = original
		data, err := io.ReadAll(r)
		require.NoError(t, err)
		require.NoError(t, r.Close())
		return string(data)
	}
}

// Ensure fmt import is used to avoid compilation error.
var _ = fmt.Sprintf
