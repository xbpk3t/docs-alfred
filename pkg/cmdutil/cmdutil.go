// Package cmdutil provides a thin wrapper around go-cmd for executing
// external commands with consistent API and error handling.
//
// It replaces raw os/exec usage across the codebase, providing:
//   - Unified stdout/stderr capture
//   - Context-aware cancellation
//   - Background (fire-and-forget) execution
//   - Binary existence checks
package cmdutil

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	gocmd "github.com/go-cmd/cmd"

	"github.com/xbpk3t/docs-alfred/pkg/fileutil"
)

// ExitError reports a command that finished with a non-zero exit code.
// Callers recover the code with errors.As instead of parsing the formatted
// message, which is presentation, not API.
type ExitError struct {
	Cmd  string
	Code int
}

func (e *ExitError) Error() string { return fmt.Sprintf("exit %d", e.Code) }

// RunWithOutput executes a command and returns combined stdout+stderr.
// Use this when you need all output in a single stream (like exec.CombinedOutput).
func RunWithOutput(ctx context.Context, dir, name string, args ...string) ([]byte, error) {
	c := gocmd.NewCmdOptions(gocmd.Options{Buffered: true, CombinedOutput: true}, name, args...)
	if dir != "" {
		c.Dir = dir
	}

	status := waitWithContext(ctx, c)
	if status.Error != nil {
		return nil, fmt.Errorf("exec %s: %w", name, status.Error)
	}
	if status.Exit != 0 {
		out := strings.Join(status.Stdout, "\n")

		return []byte(out), fmt.Errorf("exec %s: %w: %s", name, &ExitError{Cmd: name, Code: status.Exit}, out)
	}

	return []byte(strings.Join(status.Stdout, "\n")), nil
}

// RunStdout executes a command and returns only stdout.
// Use this when you only care about stdout (like exec.Output).
func RunStdout(ctx context.Context, name string, args ...string) ([]byte, error) {
	c := gocmd.NewCmdOptions(gocmd.Options{Buffered: true}, name, args...)

	status := waitWithContext(ctx, c)
	if status.Error != nil {
		return nil, fmt.Errorf("exec %s: %w", name, status.Error)
	}
	if status.Exit != 0 {
		errMsg := strings.Join(status.Stderr, "\n")

		return nil, fmt.Errorf("exec %s: %w: %s", name, &ExitError{Cmd: name, Code: status.Exit}, errMsg)
	}

	return []byte(strings.Join(status.Stdout, "\n")), nil
}

// RunSeparate executes a command and returns stdout and stderr separately.
// Use this when you need to inspect stderr independently.
func RunSeparate(ctx context.Context, name string, args ...string) ([]byte, []byte, error) {
	c := gocmd.NewCmdOptions(gocmd.Options{Buffered: true}, name, args...)

	status := waitWithContext(ctx, c)
	if status.Error != nil {
		return nil, nil, fmt.Errorf("exec %s: %w", name, status.Error)
	}
	if status.Exit != 0 {
		// Keep stdout even on failure: opencli 2.x writes its machine-readable
		// error envelope to stdout, so discarding it hides the error code/message.
		out := []byte(strings.Join(status.Stdout, "\n"))
		errMsg := strings.Join(status.Stderr, "\n")

		return out, []byte(errMsg), fmt.Errorf("exec %s: %w: %s", name, &ExitError{Cmd: name, Code: status.Exit}, errMsg)
	}

	return []byte(strings.Join(status.Stdout, "\n")),
		[]byte(strings.Join(status.Stderr, "\n")),
		nil
}

// RunSeparateLarge is RunSeparate for tools such as `yt-dlp --dump-json`
// whose output is one very long line. go-cmd's Buffered mode scans lines with
// bufio.Scanner's 64 KiB token limit, so it is bypassed here.
func RunSeparateLarge(ctx context.Context, name string, args ...string) ([]byte, []byte, error) {
	cmd := exec.CommandContext(ctx, name, args...) // #nosec G204 -- name is a fixed caller-provided binary.
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		code := -1
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			code = exitErr.ExitCode()
		}

		return stdout.Bytes(), stderr.Bytes(), fmt.Errorf("exec %s: %w: %s", name, &ExitError{Cmd: name, Code: code}, strings.TrimSpace(stderr.String()))
	}

	return stdout.Bytes(), stderr.Bytes(), nil
}

// ErrJobRunning is returned by RunBackground when a job with the same name
// is already running. Callers can errors.Is on it to treat "already syncing"
// as success instead of a failure.
var ErrJobRunning = errors.New("background job already running")

// executable is the binary spawned for background jobs: the current
// executable resolved absolutely, so spawned syncs work regardless of PATH
// (Alfred's runtime usually doesn't have the workflow binary on PATH).
// Overridable in tests.
var executable = func() string {
	if p, err := os.Executable(); err == nil {
		return p
	}

	return os.Args[0]
}

// RunBackground starts a named background job without waiting for
// completion. The process continues running after the caller returns
// (go-cmd sets Setpgid on darwin, so the child survives the parent).
//
// It spawns the current executable (executable()) with the given args.
// Deduped by jobName via a PID file, mirroring awgo's RunInBackground
// guard: calling twice with the same jobName while the first is still
// running returns ErrJobRunning instead of spawning a second process.
func RunBackground(jobName string, args ...string) error {
	pidPath := jobPIDPath(jobName)
	if pid, ok := readJobPID(pidPath); ok {
		if isProcessAlive(pid) {
			return fmt.Errorf("%w: %s with PID %d", ErrJobRunning, jobName, pid)
		}
		_ = os.Remove(pidPath) // stale PID file from a dead process
	}

	c := gocmd.NewCmdOptions(gocmd.Options{Buffered: false}, executable(), args...)
	c.Start() // non-blocking; fire and forget

	// gocmd's status channel only delivers its first value after the process
	// has *finished*, so we can't wait on it. Status().PID is set as soon as
	// exec.Cmd.Start() succeeds in run(); poll briefly for that race.
	pid, err := waitForStartedPID(c)
	if err != nil {
		return err
	}
	if err := writeJobPID(pidPath, pid); err != nil {
		return fmt.Errorf("write PID file %s: %w", pidPath, err)
	}

	return nil
}

// waitForStartedPID polls a started gocmd.Cmd until its child PID is known
// (bounded, ~10 ticks of 5ms). On failure the child is stopped so a started
// process never leaks.
func waitForStartedPID(c *gocmd.Cmd) (int, error) {
	var pid int
	for i := 0; i < 10; i++ {
		if pid = c.Status().PID; pid != 0 {
			return pid, nil
		}
		time.Sleep(5 * time.Millisecond)
	}
	_ = c.Stop()

	return 0, fmt.Errorf("background job did not report a PID within 50ms")
}

// pidDir is resolved once per process; XDG_CACHE_HOME is fixed for the
// process lifetime, so avoid a path join + env lookup on every keystroke.
var pidDir = fileutil.CachePath("jobs")

// jobPIDPath returns the PID file path for a named background job,
// stored under the shared docs-alfred cache directory (mode 0600).
func jobPIDPath(jobName string) string {
	return filepath.Join(pidDir, jobName+".pid")
}

func readJobPID(path string) (int, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, false
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	// Reject garbage and out-of-range values before the syscall: macOS
	// reports kill(maxInt, 0) as success, so a crash-residue PID file would
	// otherwise look like a live job forever.
	if err != nil || pid < 1 || pid > 10_000_000 {
		return 0, false
	}

	return pid, true
}

func isProcessAlive(pid int) bool {
	err := syscall.Kill(pid, 0)
	return err == nil || errors.Is(err, syscall.EPERM)
}

func writeJobPID(path string, pid int) error {
	return fileutil.AtomicWriteFile(path, []byte(strconv.Itoa(pid)), fileutil.FilePermPrivate)
}

// waitWithContext starts the command and waits for it to finish,
// canceling the process if the context is done first.
func waitWithContext(ctx context.Context, c *gocmd.Cmd) gocmd.Status {
	statusChan := c.Start()

	select {
	case status := <-statusChan:
		return status
	case <-ctx.Done():
		_ = c.Stop()

		return gocmd.Status{
			Error: ctx.Err(),
		}
	}
}
