package work

import (
	"bytes"
	"errors"
	"io"
	"strings"

	"github.com/samber/lo"
	"gopkg.in/yaml.v3"
)

type Doc struct {
	Type string `yaml:"type"`
	Tag  string `yaml:"tag"`
	Qs   Qs     `yaml:"qs"`
}

type Qs []struct {
	Q string `yaml:"q"` // 问题
	X string `yaml:"x"` // 答案
	U string `yaml:"u"` // url
	P string `yaml:"p"` // 图片
}

type Docs []Doc

func NewConfigQs(f []byte) (gk Docs) {
	d := yaml.NewDecoder(bytes.NewReader(f))
	for {
		// create new spec here
		spec := new(Docs)
		// pass a reference to spec reference
		if err := d.Decode(&spec); err != nil {
			// break the loop in case of EOF
			if errors.Is(err, io.EOF) {
				break
			}
			panic(err)
		}
		gk = append(gk, *spec...)
	}

	return gk
}

// GetNames Get All Names
func (d Docs) GetNames() (names []string) {
	for _, doc := range d {
		// for _, xxx := range doc.Xxx {
		// 	names = append(names, xxx.Name)
		// }
		names = append(names, doc.Type)
	}
	return
}

func (d Docs) GetNameByTag(tag string) (names []string) {
	for _, doc := range d {
		if doc.Tag == tag {
			// for _, xxx := range doc.Xxx {
			// 	names = append(names, xxx.Name)
			// }
			names = append(names, doc.Type)
		}
	}
	return
}

func (d Docs) IsHitName(query string) bool {
	return lo.ContainsBy(d.GetNames(), func(name string) bool {
		return strings.EqualFold(name, query)
	})
}

type DocsTemp struct {
	Tag string
	Types
}

type Type struct {
	Name string
	Qs   []string
}

type Types []Type

type DocsTemps []DocsTemp

// func (d Docs) ConvertToDocsTemp()  {
// 	var dc DocsTemps
//
// 	for _, doc := range d {
//
// 		types :=
//
// 		dc = append(dc, DocsTemp{
// 			Type: doc.Type,
// 			Types: make(Types, len(doc.Type)),
// 		})
//
// 		for _, t := range doc.Type {
//
// 		}
// 	}
// }

// func (d Docs) ExtractDocsTemps() DocsTemps {
// 	docsTempsMap := make(map[string]Types)
//
// 	// Extract types for each tag
// 	for _, doc := range d {
// 		if types, ok := docsTempsMap[doc.Type]; ok {
// 			// Append type to existing tag
// 			types = append(types, Type{Name: doc.Type, Qs: doc.Qs})
// 			docsTempsMap[doc.Type] = types
// 		} else {
// 			// Create new tag entry
// 			docsTempsMap[doc.Type] = Types{Type{Name: doc.Type, Qs: doc.Qs}}
// 		}
// 	}
//
// 	// Convert map to slice
// 	docsTemps := make(DocsTemps, 0, len(docsTempsMap))
// 	for tag, types := range docsTempsMap {
// 		docsTemp := DocsTemp{Type: tag, Types: types}
// 		docsTemps = append(docsTemps, docsTemp)
// 	}
//
// 	return docsTemps
// }

// func (d Docs) ExtractDocsTemps() DocsTemps {
// 	docsTempsMap := make(map[string]Types)
//
// 	// Extract types for each tag
// 	for i := 0; i < len(d); i++ {
// 		doc := d[i]
// 		if types, ok := docsTempsMap[doc.Type]; ok {
// 			// Append type to existing tag
// 			types = append(types, Type{Name: doc.Type, Qs: doc.Qs})
// 			docsTempsMap[doc.Type] = types
// 		} else {
// 			// Create new tag entry
// 			docsTempsMap[doc.Type] = Types{Type{Name: doc.Type, Qs: doc.Qs}}
// 		}
// 	}
//
// 	// Convert map to slice while preserving order
// 	docsTemps := make(DocsTemps, len(docsTempsMap))
// 	for i := 0; i < len(d); i++ {
// 		doc := d[i]
// 		if types, ok := docsTempsMap[doc.Type]; ok {
// 			docsTemp := DocsTemp{Type: doc.Type, Types: types}
// 			docsTemps[i] = docsTemp
// 		}
// 	}
//
// 	return docsTemps
// }

// func (d Docs) SearchQs(query string) []string {
// 	var qs []string
// 	for _, doc := range d {
// 		for _, xxx := range doc.Xxx {
// 			for _, q := range xxx.Qs {
// 				qsLower := strings.ToLower(q)
// 				query = strings.ToLower(query)
// 				if strings.Contains(qsLower, query) {
// 					qs = append(qs, q)
// 				}
// 			}
// 		}
// 	}
// 	return qs
// }
