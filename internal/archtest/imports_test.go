package archtest

import (
	"go/build"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSharedPackagesDoNotImportCLIOwnedPackages(t *testing.T) {
	moduleRoot := findModuleRoot(t)
	for _, root := range []string{"internal", "service"} {
		for _, pkgDir := range listGoPackages(t, filepath.Join(moduleRoot, root)) {
			pkg, err := build.ImportDir(pkgDir, 0)
			if err != nil {
				if _, ok := err.(*build.NoGoError); ok {
					continue
				}
				t.Fatalf("import %s: %v", pkgDir, err)
			}

			for _, imported := range packageImports(pkg) {
				if isCLIOwnedImport(imported) {
					t.Fatalf("shared package %s imports CLI-owned package %s", pkgDir, imported)
				}
			}
		}
	}
}

func packageImports(pkg *build.Package) []string {
	imports := make([]string, 0, len(pkg.Imports)+len(pkg.TestImports)+len(pkg.XTestImports))
	imports = append(imports, pkg.Imports...)
	imports = append(imports, pkg.TestImports...)
	imports = append(imports, pkg.XTestImports...)
	return imports
}

func isCLIOwnedImport(importPath string) bool {
	cliPrefixes := []string{
		"github.com/xbpk3t/docs-alfred/data-cli/",
		"github.com/xbpk3t/docs-alfred/docs-cli/",
		"github.com/xbpk3t/docs-alfred/gh-alfred/",
		"github.com/xbpk3t/docs-alfred/linear2nl/",
		"github.com/xbpk3t/docs-alfred/pwgen/",
		"github.com/xbpk3t/docs-alfred/rss2nl/",
		"github.com/xbpk3t/docs-alfred/xzb/",
	}
	for _, prefix := range cliPrefixes {
		if strings.HasPrefix(importPath, prefix) {
			return true
		}
	}

	return false
}

func findModuleRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found")
		}
		dir = parent
	}
}

func listGoPackages(t *testing.T, root string) []string {
	t.Helper()
	var packages []string
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() {
			return nil
		}
		base := entry.Name()
		if strings.HasPrefix(base, ".") || base == "testdata" {
			return filepath.SkipDir
		}
		if hasGoFiles(t, path) {
			packages = append(packages, path)
		}

		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	return packages
}

func hasGoFiles(t *testing.T, dir string) bool {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".go") {
			return true
		}
	}

	return false
}
