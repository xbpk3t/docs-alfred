package algo

import (
	"bytes"
	"errors"
	"io"

	"gopkg.in/yaml.v3"
)

type ConfigAlgo []struct {
	Type  string `yaml:"type"`
	Tag   string `yaml:"tag"`
	Repos Repo   `yaml:"repo"`
}

type Repo []struct {
	Qs  string `yaml:"qs"`            // 算法题
	URL string `yaml:"url,omitempty"` // 该题目的题解
	Sol string `yaml:"sol,omitempty"` // des & 思路
	// Des string `yaml:"des"`
	// Doc string `yaml:"doc,omitempty"` // 不同于url，doc通常是blog的url
}

func NewConfigAlgo(f []byte) (ga ConfigAlgo) {
	d := yaml.NewDecoder(bytes.NewReader(f))
	for {
		// create new spec here
		spec := new(ConfigAlgo)
		// pass a reference to spec reference
		if err := d.Decode(&spec); err != nil {
			// break the loop in case of EOF
			if errors.Is(err, io.EOF) {
				break
			}
			panic(err)
		}
		ga = append(ga, *spec...)
	}

	return ga
}
