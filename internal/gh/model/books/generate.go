// Package books holds Go data models generated from the books JSON Schema.
// books_gen.go is generated — do not edit by hand; run `go generate ./internal/gh/model/books`.
package books

//go:generate go run github.com/atombender/go-jsonschema@v0.24.1 -p books --only-models --capitalization URL -o books_gen.go ../../schema/books.schema.json
