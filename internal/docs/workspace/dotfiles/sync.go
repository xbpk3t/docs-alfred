package dotfiles

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/xbpk3t/docs-alfred/pkg/fileutil"
	"github.com/xbpk3t/docs-alfred/pkg/gitutil"
)

// SyncRecordOptions holds options for sync-record.
type SyncRecordOptions struct {
	DotfilesPath string
}

// ChangeFile represents a changed file in the sync record output.
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

// SyncRecordResult holds the sync record result.
type SyncRecordResult struct {
	DotfilesPath string       `json:"dotfilesPath"`
	Error        string       `json:"error,omitempty"`
	ChangedFiles []ChangeFile `json:"changedFiles"`
	OK           bool         `json:"ok"`
}

const (
	homeBasePrefix = "home/base/"
	ghDataPrefix   = "data/gh"
)

// RunSyncRecord inspects dotfiles changes for record synchronization.
func RunSyncRecord(opts SyncRecordOptions) *SyncRecordResult {
	dotfilesPath := opts.DotfilesPath

	if info, err := os.Stat(dotfilesPath); err != nil || !info.IsDir() {
		return &SyncRecordResult{
			DotfilesPath: dotfilesPath,
			OK:           false,
			Error:        "dotfiles path not found: " + dotfilesPath,
		}
	}

	gitDir := filepath.Join(dotfilesPath, ".git")
	if _, err := os.Stat(gitDir); os.IsNotExist(err) {
		return &SyncRecordResult{
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

	result := &SyncRecordResult{
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
	changed, err := gitutil.ChangedFiles(repoPath)
	if err != nil {
		return nil
	}

	files := make([]changedFile, 0, len(changed))
	for _, f := range changed {
		files = append(files, changedFile{Path: f.Path, Status: f.Status})
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

	var ghFiles []string
	if files, err := fileutil.ListYAMLFiles(ghDir); err == nil {
		ghFiles = files
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
