// Package model holds Go data models generated from the gh JSON Schema.
// gh_gen.go is generated — do not edit by hand; run `go generate ./internal/gh/model`.
package model

//go:generate go run github.com/atombender/go-jsonschema@v0.24.1 -p model --only-models -o gh_gen.go ../schema/gh.schema.json
