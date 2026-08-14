// Package goods holds Go data models generated from the goods JSON Schema.
// goods_gen.go is generated — do not edit by hand; run `go generate ./internal/gh/model/goods`.
package goods

//go:generate go run github.com/atombender/go-jsonschema@v0.24.1 -p goods --only-models --capitalization URL -o goods_gen.go ../../schema/goods.schema.json
