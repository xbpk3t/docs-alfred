package goods

import (
	"bytes"
	"errors"
	"gopkg.in/yaml.v3"
	"io"
)

type ConfigGoods []ConfigGoodsX

type ConfigGoodsX struct {
	Type  string   `yaml:"type"`
	Tag   string   `yaml:"tag"`
	Goods []GoodsX `yaml:"goods"`
	Des   string   `yaml:"des,omitempty"`
	Qs    []Qs     `yaml:"qs,omitempty"`
}

type GoodsX struct {
	Name  string   `yaml:"name"`
	Param string   `yaml:"param,omitempty"`
	Price string   `yaml:"price,omitempty"`
	Date  []string `yaml:"date,omitempty"`
	Des   string   `yaml:"des,omitempty"`
	URL   string   `yaml:"url,omitempty"`
	Use   bool     `yaml:"use,omitempty"`
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
