package cmdutil

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
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

// RunBackground spawns the test binary (via the executable() seam) with
// -test.run helper args, so spawned children run only the helper tests
// below instead of the full suite (which would recurse infinitely).

func TestHelperNoop(t *testing.T) {
	// Exits immediately; used to exercise RunBackground's spawn path.
}

func TestHelperSleep(t *testing.T) {
	// Stays alive briefly; used to hold a job open for dedup assertions.
	time.Sleep(2 * time.Second)
}

func isolateCache(t *testing.T) {
	t.Helper()
	t.Setenv("XDG_CACHE_HOME", t.TempDir()) // keep PID files out of the real cache
}

func TestRunBackground(t *testing.T) {
	isolateCache(t)
	err := RunBackground("test-job", "-test.run=TestHelperNoop")
	assert.NoError(t, err)
	// Give the background process a moment to start
	time.Sleep(50 * time.Millisecond)
}

func TestRunBackgroundDeduplicatesRunningJob(t *testing.T) {
	isolateCache(t)
	err := RunBackground("dedup-job", "-test.run=TestHelperSleep")
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.Remove(jobPIDPath("dedup-job")) })

	err = RunBackground("dedup-job", "-test.run=TestHelperSleep")
	require.ErrorIs(t, err, ErrJobRunning)
}

func TestRunBackgroundDifferentJobsAllowed(t *testing.T) {
	isolateCache(t)
	require.NoError(t, RunBackground("job-a", "-test.run=TestHelperNoop"))
	require.NoError(t, RunBackground("job-b", "-test.run=TestHelperNoop"))
	t.Cleanup(func() {
		_ = os.Remove(jobPIDPath("job-a"))
		_ = os.Remove(jobPIDPath("job-b"))
	})
}

func TestRunBackgroundWritesPrivatePIDFile(t *testing.T) {
	isolateCache(t)
	require.NoError(t, RunBackground("perm-job", "-test.run=TestHelperSleep"))
	t.Cleanup(func() { _ = os.Remove(jobPIDPath("perm-job")) })

	path := jobPIDPath("perm-job")
	info, err := os.Stat(path)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0600), info.Mode().Perm())

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	require.NoError(t, err)
	assert.True(t, isProcessAlive(pid))
}

func TestRunBackgroundStalePIDFileAllowsRestart(t *testing.T) {
	isolateCache(t)
	path := jobPIDPath("stale-job")
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0700))
	// A stale PID file with a garbage PID must not block a new run.
	require.NoError(t, os.WriteFile(path, []byte("9223372036854775807"), 0600))
	t.Cleanup(func() { _ = os.Remove(path) })

	require.NoError(t, RunBackground("stale-job", "-test.run=TestHelperNoop"))
}

func TestIsProcessAlive(t *testing.T) {
	assert.True(t, isProcessAlive(os.Getpid()))
	// isProcessAlive must not be fed raw input: readJobPID vets the range
	// (macOS reports kill(maxInt, 0) as success, so garbage would
	// false-positive as "alive").
	_, ok := readJobPID(mustWriteTestPID(t, "999999999"))
	assert.False(t, ok)
}

func TestReadJobPIDRejectsGarbage(t *testing.T) {
	isolateCache(t)
	path := jobPIDPath("garbage-job")
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0700))
	require.NoError(t, os.WriteFile(path, []byte("not-a-number"), 0600))
	t.Cleanup(func() { _ = os.Remove(path) })

	_, ok := readJobPID(path)
	assert.False(t, ok)
}

func mustWriteTestPID(t *testing.T, pid string) string {
	t.Helper()
	isolateCache(t)
	path := jobPIDPath("range-job")
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0700))
	require.NoError(t, os.WriteFile(path, []byte(pid), 0600))
	t.Cleanup(func() { _ = os.Remove(path) })

	return path
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
