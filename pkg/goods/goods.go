package goods

import (
	"bytes"
	"errors"
	"io"

	"gopkg.in/yaml.v3"
)

type ConfigGoods []struct {
	Type  string `yaml:"type"`
	Tag   string `yaml:"tag"`
	Goods Goods  `yaml:"goods"`
	Des   string `yaml:"des,omitempty"`
	Qs    []Qs   `yaml:"qs,omitempty"`
}

type Goods []struct {
	Name  string `yaml:"name"`
	Param string `yaml:"param,omitempty"`
	Price string `yaml:"price,omitempty"`
	Des   string `yaml:"des,omitempty"`
	URL   string `yaml:"url,omitempty"`
	Use   bool   `yaml:"use,omitempty"`
}

type Qs struct {
	Q string `yaml:"q"`
	X string `yaml:"x"`
}

func NewConfigGoods(f []byte) (gk ConfigGoods) {
	d := yaml.NewDecoder(bytes.NewReader(f))
	for {
		// create new spec here
		spec := new(ConfigGoods)
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
