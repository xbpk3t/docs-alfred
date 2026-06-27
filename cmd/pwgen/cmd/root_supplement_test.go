package cmd

import (
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadPwgenConfigInvalidYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.yaml")
	require.NoError(t, os.WriteFile(path, []byte("secret: [invalid yaml: {{"), 0600))

	_, err := loadPwgenConfig(path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "parse")
}

func TestApplyFlagOverridesLengthAndOutput(t *testing.T) {
	cmd := newRootCmd()
	require.NoError(t, cmd.ParseFlags([]string{"--length", "24", "--format", "json"}))

	cfg := defaultPwgenConfig()
	cfg.Length = 32
	cfg.Format = "text"

	cfg = applyFlagOverrides(cmd, cfg, "", 24, true, true, false, "json")

	assert.Equal(t, 24, cfg.Length, "flag-changed length should override config")
	assert.Equal(t, "json", cfg.Format, "flag-changed format should override config")
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
		"--format", "json",
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
	old := os.Stdout
	r, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stdout = w

	origArgs := os.Args
	os.Args = []string{"pwgen", "--format", "text", "test.example.com"}

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

func TestExecuteErrorPath(t *testing.T) {
	if os.Getenv("TEST_EXECUTE_ERROR") == "1" {
		os.Args = []string{"pwgen"}
		Execute()

		return
	}

	cmd := exec.Command(os.Args[0], "-test.run=TestExecuteErrorPath", "-test.count=1")
	cmd.Env = append(os.Environ(), "TEST_EXECUTE_ERROR=1")

	err := cmd.Run()
	require.Error(t, err, "subprocess should exit with error")

	var exitErr *exec.ExitError
	require.ErrorAs(t, err, &exitErr)
	assert.False(t, exitErr.Success(), "should exit with non-zero code")
}
