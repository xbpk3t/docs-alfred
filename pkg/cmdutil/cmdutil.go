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
	"context"
	"fmt"
	"os/exec"
	"strings"

	gocmd "github.com/go-cmd/cmd"
)

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

		return []byte(out), fmt.Errorf("exec %s: exit %d: %s", name, status.Exit, out)
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

		return nil, fmt.Errorf("exec %s: exit %d: %s", name, status.Exit, errMsg)
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
		errMsg := strings.Join(status.Stderr, "\n")

		return nil, nil, fmt.Errorf("exec %s: exit %d: %s", name, status.Exit, errMsg)
	}

	return []byte(strings.Join(status.Stdout, "\n")),
		[]byte(strings.Join(status.Stderr, "\n")),
		nil
}

// RunBackground starts a command in the background without waiting for
// completion. The process continues running after the caller returns.
// Use this for fire-and-forget scenarios like background syncs.
func RunBackground(name string, args ...string) error {
	c := gocmd.NewCmdOptions(gocmd.Options{Buffered: false}, name, args...)
	c.Start() // non-blocking; channel returned but not read

	return nil
}

// LookPath checks if a binary exists in PATH.
// Returns the full path and true if found.
func LookPath(name string) (string, bool) {
	path, err := exec.LookPath(name)
	if err != nil {
		return "", false
	}

	return path, true
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
