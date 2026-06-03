package dotfiles

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// SyncPlanOptions holds options for the sync-plan.
type SyncPlanOptions struct {
	DotfilesPath string
	JSON         bool
}

// ChangeFile represents a changed file in the sync plan.
type ChangeFile struct {
	Gh     *GhMap `json:"gh,omitempty"`
	Path   string `json:"path"`
	Status string `json:"status"`
}

// GhMap maps a changed dotfiles file to its GH counterpart.
type GhMap struct {
	Category string   `json:"category"`
	GhDir    string   `json:"ghDir"`
	GhFiles  []string `json:"ghFiles"`
}

// SyncPlanResult holds the sync plan result.
type SyncPlanResult struct {
	DotfilesPath string       `json:"dotfilesPath"`
	Error        string       `json:"error,omitempty"`
	ChangedFiles []ChangeFile `json:"changedFiles"`
	OK           bool         `json:"ok"`
}

const (
	homeBasePrefix = "home/base/"
	ghDataPrefix   = "data/gh"
)

// RunSyncPlan plans dotfiles synchronization based on git changes.
func RunSyncPlan(opts SyncPlanOptions) *SyncPlanResult {
	dotfilesPath := opts.DotfilesPath

	if info, err := os.Stat(dotfilesPath); err != nil || !info.IsDir() {
		return &SyncPlanResult{
			DotfilesPath: dotfilesPath,
			OK:           false,
			Error:        "dotfiles path not found: " + dotfilesPath,
		}
	}

	gitDir := filepath.Join(dotfilesPath, ".git")
	if _, err := os.Stat(gitDir); os.IsNotExist(err) {
		return &SyncPlanResult{
			DotfilesPath: dotfilesPath,
			OK:           false,
			Error:        dotfilesPath + " exists but is not a git repository",
		}
	}

	changedFiles := getChangedFiles(dotfilesPath)

	var changes []ChangeFile
	for _, f := range changedFiles {
		change := ChangeFile{
			Path:   f.Path,
			Status: f.Status,
			Gh:     mapToGh(f.Path),
		}
		changes = append(changes, change)
	}

	result := &SyncPlanResult{
		DotfilesPath: dotfilesPath,
		OK:           true,
		ChangedFiles: changes,
	}

	return result
}

type changedFile struct {
	Path   string
	Status string
}

func getChangedFiles(repoPath string) []changedFile {
	cmd := exec.Command("git", "-C", repoPath, "status", "--porcelain")
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return nil
	}

	var files []changedFile
	for line := range strings.SplitSeq(out.String(), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if len(line) < 4 {
			continue
		}
		status := strings.TrimSpace(line[:2])
		path := strings.TrimSpace(line[3:])
		files = append(files, changedFile{Path: path, Status: status})
	}

	return files
}

func mapToGh(filePath string) *GhMap {
	if !strings.HasPrefix(filePath, homeBasePrefix) {
		return nil
	}

	relative := filePath[len(homeBasePrefix):]
	parts := strings.SplitN(relative, "/", 2)
	if len(parts) == 0 || parts[0] == "" {
		return nil
	}
	category := parts[0]
	ghDir := filepath.Join(ghDataPrefix, category)

	ghRoot := ghDir
	var ghFiles []string
	if entries, err := os.ReadDir(ghRoot); err == nil {
		for _, e := range entries {
			if !e.IsDir() && (strings.HasSuffix(e.Name(), ".yml") || strings.HasSuffix(e.Name(), ".yaml")) {
				ghFiles = append(ghFiles, filepath.Join(ghDir, e.Name()))
			}
		}
	}

	if len(ghFiles) == 0 {
		return nil
	}

	return &GhMap{
		Category: category,
		GhDir:    ghDir,
		GhFiles:  ghFiles,
	}
}

// PrintResult prints the sync plan result.
func (r *SyncPlanResult) PrintResult(jsonOutput bool) {
	if jsonOutput {
		data, err := json.MarshalIndent(r, "", "  ")
		if err != nil {
			slog.Error("Failed to marshal sync plan", "error", err)

			return
		}
		os.Stdout.Write(data) //nolint:errcheck // JSON delivery to stdout pipe

		return
	}

	if !r.OK {
		slog.Error("Sync plan failed", "error", r.Error)

		return
	}

	slog.Info("Dotfiles sync plan", "path", r.DotfilesPath, "changed", len(r.ChangedFiles))
	for _, f := range r.ChangedFiles {
		status := f.Status
		switch status {
		case "M":
			status = "modified"
		case "A":
			status = "added"
		case "D":
			status = "deleted"
		case "??":
			status = "untracked"
		}
		slog.Info("Changed file", "status", status, "path", f.Path)
		if f.Gh != nil {
			slog.Info("Gh mapping", "category", f.Gh.Category, "files", strings.Join(f.Gh.GhFiles, ", "))
		}
	}
}
