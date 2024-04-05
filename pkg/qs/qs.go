package qs

import (
	"bytes"
	"errors"
	"io"
	"os"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/samber/lo"
)

type Doc struct {
	Type string   `yaml:"type"`
	Tag  string   `yaml:"tag"`
	Qs   []string `yaml:"qs"`
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

func (d Docs) GetQsByName(name string) []string {
	for _, doc := range d {
		if strings.EqualFold(strings.ToLower(doc.Type), strings.ToLower(name)) {
			return doc.Qs
		}
	}

	return nil
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
// 			Tag: doc.Tag,
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
// 		if types, ok := docsTempsMap[doc.Tag]; ok {
// 			// Append type to existing tag
// 			types = append(types, Type{Name: doc.Type, Qs: doc.Qs})
// 			docsTempsMap[doc.Tag] = types
// 		} else {
// 			// Create new tag entry
// 			docsTempsMap[doc.Tag] = Types{Type{Name: doc.Type, Qs: doc.Qs}}
// 		}
// 	}
//
// 	// Convert map to slice
// 	docsTemps := make(DocsTemps, 0, len(docsTempsMap))
// 	for tag, types := range docsTempsMap {
// 		docsTemp := DocsTemp{Tag: tag, Types: types}
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
// 		if types, ok := docsTempsMap[doc.Tag]; ok {
// 			// Append type to existing tag
// 			types = append(types, Type{Name: doc.Type, Qs: doc.Qs})
// 			docsTempsMap[doc.Tag] = types
// 		} else {
// 			// Create new tag entry
// 			docsTempsMap[doc.Tag] = Types{Type{Name: doc.Type, Qs: doc.Qs}}
// 		}
// 	}
//
// 	// Convert map to slice while preserving order
// 	docsTemps := make(DocsTemps, len(docsTempsMap))
// 	for i := 0; i < len(d); i++ {
// 		doc := d[i]
// 		if types, ok := docsTempsMap[doc.Tag]; ok {
// 			docsTemp := DocsTemp{Tag: doc.Tag, Types: types}
// 			docsTemps[i] = docsTemp
// 		}
// 	}
//
// 	return docsTemps
// }

func (d Docs) ConvertToDocsTemps() DocsTemps {
	// Create a slice to store the result
	result := make(DocsTemps, 0)

	// Iterate over the d
	for _, doc := range d {
		// Check if the current tag already exists in the result
		found := false
		for i := range result {
			if result[i].Tag == doc.Tag {
				// Add the type to the existing tag entry
				result[i].Types = append(result[i].Types, Type{Name: doc.Type, Qs: doc.Qs})
				found = true
				break
			}
		}

		// If the tag was not found, create a new tag entry
		if !found {
			result = append(result, DocsTemp{
				Tag:   doc.Tag,
				Types: Types{{Name: doc.Type, Qs: doc.Qs}},
			})
		}
	}

	return result
}

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

func PathExists(path string) bool {
	_, err := os.Stat(path)
	if err == nil {
		return true
	}
	if os.IsNotExist(err) {
		return false
	}
	panic(err)
}

// Load reads data saved under given name.
func Load(name string) ([]byte, error) {
	// p := c.path(name)
	if _, err := os.Stat(name); err != nil {
		return nil, err
	}
	return os.ReadFile(name)
}
