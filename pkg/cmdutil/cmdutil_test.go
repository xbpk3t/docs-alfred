package cmdutil

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- RunWithOutput tests ---

func TestRunWithOutputEcho(t *testing.T) {
	ctx := context.Background()
	out, err := RunWithOutput(ctx, "", "echo", "hello")
	require.NoError(t, err)
	assert.Contains(t, string(out), "hello")
}

func TestRunWithOutputWithDir(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	out, err := RunWithOutput(ctx, dir, "pwd")
	require.NoError(t, err)
	assert.Contains(t, string(out), dir)
}

func TestRunWithOutputNonZeroExit(t *testing.T) {
	ctx := context.Background()
	_, err := RunWithOutput(ctx, "", "false")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "exit 1")
}

func TestRunWithOutputCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := RunWithOutput(ctx, "", "sleep", "10")
	require.Error(t, err)
}

func TestRunWithOutputInvalidCommand(t *testing.T) {
	ctx := context.Background()
	_, err := RunWithOutput(ctx, "", "nonexistent_command_xyz")
	require.Error(t, err)
}

// --- RunStdout tests ---

func TestRunStdoutEcho(t *testing.T) {
	ctx := context.Background()
	out, err := RunStdout(ctx, "echo", "world")
	require.NoError(t, err)
	assert.Contains(t, string(out), "world")
}

func TestRunStdoutNonZeroExit(t *testing.T) {
	ctx := context.Background()
	_, err := RunStdout(ctx, "false")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "exit 1")
}

func TestRunStdoutCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := RunStdout(ctx, "sleep", "10")
	require.Error(t, err)
}

func TestRunStdoutInvalidCommand(t *testing.T) {
	ctx := context.Background()
	_, err := RunStdout(ctx, "nonexistent_command_xyz")
	require.Error(t, err)
}

// --- RunSeparate tests ---

func TestRunSeparateEcho(t *testing.T) {
	ctx := context.Background()
	stdout, stderr, err := RunSeparate(ctx, "echo", "test")
	require.NoError(t, err)
	assert.Contains(t, string(stdout), "test")
	assert.Empty(t, string(stderr))
}

func TestRunSeparateNonZeroExit(t *testing.T) {
	ctx := context.Background()
	_, _, err := RunSeparate(ctx, "sh", "-c", "echo err >&2; exit 1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "exit 1")
}

func TestRunSeparateCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, _, err := RunSeparate(ctx, "sleep", "10")
	require.Error(t, err)
}

func TestRunSeparateInvalidCommand(t *testing.T) {
	ctx := context.Background()
	_, _, err := RunSeparate(ctx, "nonexistent_command_xyz")
	require.Error(t, err)
}

// --- RunBackground tests ---

func TestRunBackground(t *testing.T) {
	err := RunBackground("echo", "bg")
	assert.NoError(t, err)
	// Give the background process a moment to start
	time.Sleep(50 * time.Millisecond)
}

// --- LookPath tests ---

func TestLookPathExists(t *testing.T) {
	path, found := LookPath("echo")
	assert.True(t, found)
	assert.NotEmpty(t, path)
}

func TestLookPathNotFound(t *testing.T) {
	path, found := LookPath("nonexistent_command_xyz")
	assert.False(t, found)
	assert.Empty(t, path)
}
