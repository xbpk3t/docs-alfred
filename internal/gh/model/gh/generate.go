// Package gh holds Go data models generated from the gh JSON Schema.
// gh_gen.go is generated — do not edit by hand; run `go generate ./internal/gh/model/gh`.
package gh

//go:generate go run github.com/atombender/go-jsonschema@v0.24.1 -p gh --only-models --capitalization URL -o gh_gen.go ../../schema/gh.schema.json
