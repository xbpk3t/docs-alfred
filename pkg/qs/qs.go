package qs

import (
	"bytes"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/samber/lo"
	"gopkg.in/yaml.v3"
)

type Doc struct {
	Cate string `yaml:"cate"`
	Xxx  []Xxx  `yaml:"xxx"`
}

type Xxx struct {
	Name string   `yaml:"name"`
	Qs   []string `yaml:"qs"`
}

type Docs []Doc

func NewDocs(fp string) Docs {
	var docs Docs
	if PathExists(fp) {
		err := filepath.WalkDir(fp, func(path string, de fs.DirEntry, err error) error {
			if !de.IsDir() {
				f, err := Load(path)
				if err != nil {
					return err
				}
				d := yaml.NewDecoder(bytes.NewReader(f))
				for {
					spec := new(Doc)
					spec.Cate = de.Name()
					if err := d.Decode(&spec); err != nil {
						// break the loop in case of EOF
						if errors.Is(err, io.EOF) {
							break
						}
						panic(err)
					}
					if spec != nil {
						docs = append(docs, *spec)
					}
				}
			}
			return nil
		})
		if err != nil {
			return nil
		}
	}
	return docs
}

// GetNames Get All Names
func (d Docs) GetNames() (names []string) {
	for _, doc := range d {
		for _, xxx := range doc.Xxx {
			names = append(names, xxx.Name)
		}
	}
	return
}

func (d Docs) GetNameByCate(cate string) (names []string) {
	for _, doc := range d {
		if doc.Cate == cate {
			for _, xxx := range doc.Xxx {
				names = append(names, xxx.Name)
			}
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
		for _, xxx := range doc.Xxx {
			if strings.EqualFold(strings.ToLower(xxx.Name), strings.ToLower(name)) {
				return xxx.Qs
			}
		}
	}
	return nil
}

func (d Docs) SearchQs(query string) []string {
	var qs []string
	for _, doc := range d {
		for _, xxx := range doc.Xxx {
			for _, q := range xxx.Qs {
				qsLower := strings.ToLower(q)
				query = strings.ToLower(query)
				if strings.Contains(qsLower, query) {
					qs = append(qs, q)
				}
			}
		}
	}
	return qs
}

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
