package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/xbpk3t/docs-alfred/cmd/skx/internal/skx"
)

func newRenderCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "render [file-or-dir]",
		Short: "Render zzz prompt YAML to markdown (default: whole references dir)",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRender(targetOrDir(flags, args), flags.dryRun)
		},
	}
}

func runRender(target string, dryRun bool) error {
	fi, statErr := os.Stat(target)
	if statErr != nil {
		return fmt.Errorf("stat %s: %w", target, statErr)
	}

	// Render is gated on schema conformance: refuse to emit output for data
	// that deviates from prpt.yml rather than silently rendering it.
	if gateErr := gateOnCheck(target); gateErr != nil {
		return gateErr
	}

	if !fi.IsDir() {
		return renderOne(target, dryRun)
	}

	results, err := skx.RenderDir(target)
	if err != nil {
		return err
	}
	for _, r := range results {
		if r.Err != nil {
			fmt.Fprintf(os.Stderr, "error: %s: %v\n", r.YMLPath, r.Err)
			continue
		}
		if dryRun {
			fmt.Fprintf(os.Stderr, "dry-run: would write %s\n", r.MDPath)
			continue
		}
		if writeErr := skx.WriteMD(r.YMLPath, r.Content); writeErr != nil {
			fmt.Fprintf(os.Stderr, "error: %s: %v\n", r.YMLPath, writeErr)
			continue
		}
		fmt.Fprintf(os.Stderr, "rendered %s\n", r.MDPath)
	}
	return nil
}

// renderOne renders a single yml file (and writes the sibling .md unless
// dry-run). Progress is written to stderr so stdout stays clean for piping.
func renderOne(target string, dryRun bool) error {
	content, err := skx.RenderFile(target)
	if err != nil {
		return err
	}
	mdPath := skx.MDNameFor(target)
	if dryRun {
		fmt.Fprintf(os.Stderr, "dry-run: would write %s\n", mdPath)
		return nil
	}
	if writeErr := skx.WriteMD(target, content); writeErr != nil {
		return writeErr
	}
	fmt.Fprintf(os.Stderr, "rendered %s\n", mdPath)
	return nil
}

// gateOnCheck refuses to render when the target fails schema check.
func gateOnCheck(target string) error {
	schema, err := skx.FindSchema(target)
	if err != nil {
		return err
	}

	var issues []skx.Issue
	if fi, err := os.Stat(target); err == nil && !fi.IsDir() {
		sch, cerr := skx.CompileSchema(schema)
		if cerr != nil {
			return cerr
		}
		issues = skx.CheckFile(target, sch)
	} else {
		res, cerr := skx.CheckDir(target, schema)
		if cerr != nil {
			return cerr
		}
		issues = res.Issues
	}
	if len(issues) > 0 {
		return fmt.Errorf("render refused: %d schema issue(s); run `skx check` first", len(issues))
	}
	return nil
}
