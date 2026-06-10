package archtest

import (
	"fmt"
	"go/ast"
	"go/build"
	"go/parser"
	"go/token"
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

// TestSharedPackagesDoNotCallEnvOrExec verifies that shared internal/service
// packages do not directly call os.Getenv or exec.Command. These should be
// handled at the CLI layer and injected via Config or interfaces.
// TestUsecasePackagesDoNotCallEnvOrExec verifies that internal usecase packages
// do not directly call os.Getenv or exec.Command. These should be handled at the
// CLI layer and injected via Config or interfaces. Service/adapter packages are
// excluded since they legitimately wrap external commands and environment access.
func TestUsecasePackagesDoNotCallEnvOrExec(t *testing.T) {
	moduleRoot := findModuleRoot(t)
	for _, pkgDir := range listGoPackages(t, filepath.Join(moduleRoot, "internal")) {
		// Skip archtest and non-usecase packages.
		base := filepath.Base(pkgDir)
		if base == "archtest" || base == "transcript" {
			continue
		}

		violations := findEnvExecCalls(t, pkgDir)
		for _, v := range violations {
			t.Errorf("usecase package %s", v)
		}
	}
}

// findEnvExecCalls scans .go files in dir for direct os.Getenv or exec.Command calls.
func findEnvExecCalls(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}

	var violations []string
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}

		path := filepath.Join(dir, entry.Name())
		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
		if err != nil {
			continue
		}

		for _, imp := range file.Imports {
			if imp.Path == nil {
				continue
			}
			pkg := imp.Path.Value
			if pkg != `"os"` && pkg != `"os/exec"` {
				continue
			}

			alias := pkgAlias(imp)
			astInspectCalls(file, fset, alias, func(callName string, pos token.Position) {
				violations = append(violations,
					fmt.Sprintf("%s:%d: direct %s call (move to CLI layer)", entry.Name(), pos.Line, callName))
			})
		}
	}

	return violations
}

func pkgAlias(imp *ast.ImportSpec) string {
	if imp.Name != nil {
		return imp.Name.Name
	}
	p := strings.Trim(imp.Path.Value, `"`)
	if i := strings.LastIndex(p, "/"); i >= 0 {
		return p[i+1:]
	}
	return p
}

func astInspectCalls(file *ast.File, fset *token.FileSet, targetAlias string, onCall func(string, token.Position)) {
	ast.Inspect(file, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		ident, ok := sel.X.(*ast.Ident)
		if !ok {
			return true
		}
		if ident.Name != targetAlias {
			return true
		}

		method := sel.Sel.Name
		fullCall := targetAlias + "." + method

		switch fullCall {
		case "os.Getenv", "os.LookupEnv", "os.Setenv", "os.Unsetenv",
			"exec.Command", "exec.CommandContext":
			onCall(fullCall, fset.Position(call.Lparen))
		}

		return true
	})
}
