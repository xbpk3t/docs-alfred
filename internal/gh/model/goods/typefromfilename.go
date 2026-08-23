// Hand-maintained (not go:generate'd): filename-derived helpers for the goods
// data model. The data/gh-style layout no longer carries a top-level "type",
// so it is inferred from the file stem via the shared helper.
package goods

import "github.com/xbpk3t/docs-alfred/pkg/fileutil"

// TypeFromFilename derives a data type tag from the file name (e.g.
// goods.EDC.yml → "EDC", coding.yml → "coding"), stripping the "goods."
// domain prefix in addition to the shared extension/dot rules.
func TypeFromFilename(name string) string {
	return fileutil.TypeFromFilename(name, "goods.")
}
