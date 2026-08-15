package domrules

import (
	"github.com/xbpk3t/docs-alfred/pkg/checkutil"
)

// RuleScope defines which field set to use for validation.
type RuleScope string

const (
	// ScopeDiary is the only remaining structured-check scope. books/ntl/goods
	// are validated against their JSON Schemas; gh uses the walker; task is
	// YAML-parse-only.
	ScopeDiary RuleScope = "diary"
)

// DataDomain defines a data domain for validation.
type DataDomain string

const (
	DomainBooks DataDomain = "books"
	DomainDiary DataDomain = "diary"
	DomainGH    DataDomain = "gh"
	DomainGoods DataDomain = "goods"
	DomainTask  DataDomain = "task"
	DomainNtl   DataDomain = "ntl"
)

var DiaryFields = map[string]bool{
	"date": true, "review": true, fieldDes: true, fieldScore: true,
	"week": true, fieldURL: true,
}

var ForbiddenFields = map[string]bool{
	"category": true,
}

// date format patterns. publishAt 年份校验已迁到 books.schema.json
// (type: integer, 1000-9999)，不再需要 DateYear。
var (
	DateFull = checkutil.DateFullPattern // alias for backward compatibility
)
