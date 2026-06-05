package data

import (
	"regexp"

	"github.com/xbpk3t/docs-alfred/pkg/checkutil"
)

// RuleScope defines which field set to use for validation.
type RuleScope string

const (
	ScopeBooks RuleScope = "books"
	ScopeMovie RuleScope = "movie"
	ScopeMusic RuleScope = "music"
	ScopeDiary RuleScope = "diary"
	ScopeJav   RuleScope = "jav"
	ScopeVG    RuleScope = "vg"
)

// DataDomain defines a data domain for validation.
type DataDomain string

const (
	DomainBooks DataDomain = "books"
	DomainMovie DataDomain = "movie"
	DomainTV    DataDomain = "tv"
	DomainMusic DataDomain = "music"
	DomainDiary DataDomain = "diary"
	DomainGH    DataDomain = "gh"
	DomainGoods DataDomain = "goods"
	DomainTask  DataDomain = "task"
	DomainNtl   DataDomain = "ntl"
)

var AllDataDomains = []DataDomain{
	DomainBooks, DomainMovie, DomainTV, DomainMusic, DomainDiary,
	DomainGH, DomainGoods, DomainTask, DomainNtl,
}

var DuplicateDomains = []DataDomain{DomainBooks, DomainMusic, DomainGH}

const (
	fieldAlias = "alias"
	fieldCast  = "cast"
	fieldLabel = "label"
	fieldAuthor = "author"
)

// ContentFields defines the allowed field set for content scope.
var ContentFields = map[string]bool{
	fieldName: true, fieldAlias: true, fieldAuthor: true, fieldScore: true,
	"readAt": true, "readTime": true, fieldPublishAt: true,
	fieldDes: true, fieldDate: true, fieldRecord: true, fieldSub: true,
	fieldTable: true, "recite": true, fieldItem: true, fieldURL: true,
	fieldTags: true, "tag": true, "qs": true, "source": true,
	"content": true, "topics": true, "dict": true, fieldCast: true,
}

var MusicFields = map[string]bool{
	fieldName: true, fieldAuthor: true, fieldScore: true,
	fieldPublishAt: true, fieldDes: true, fieldURL: true,
	fieldTags: true, fieldRecord: true, "perf": true,
	fieldLabel: true, "conductor": true,
}

var DiaryFields = map[string]bool{
	"date": true, "review": true, fieldDes: true, fieldScore: true,
	"week": true, fieldURL: true,
}

var JavFields = map[string]bool{
	fieldURL: true, "cast": true, fieldScore: true, fieldDes: true,
	fieldTags: true, fieldRecord: true, "rel": true, fieldSub: true,
	"label": true, fieldPublishAt: true, fieldName: true,
}

var VGFields = map[string]bool{
	fieldName: true, "developer": true, "price": true, fieldDes: true,
	"playAt": true, "score": true, fieldRecord: true, "tags": true,
	"url": true, "sub": true, fieldTable: true,
	"genre": true, "status": true, "platform": true, "publishAt": true,
	"alias": true,
}

var ForbiddenFields = map[string]bool{
	"category": true,
}

// date format patterns.
var (
	DateFull   = checkutil.DateFullPattern // alias for backward compatibility
	DateYear   = regexp.MustCompile(`^-?\d{1,4}$`)
	SeriesHint = regexp.MustCompile(`(系列|三部曲|四部曲|合集)`)
)
