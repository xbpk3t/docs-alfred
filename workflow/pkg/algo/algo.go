package algo

import (
	"bytes"
	"errors"
	"gopkg.in/yaml.v3"
	"io"
)

type ConfigAlgo []struct {
	Type  string `yaml:"type"`
	Tag   string `yaml:"tag"`
	Repos Repo   `yaml:"repo"`
}

type Repo []struct {
	Qs  string `yaml:"qs"`  // 算法题
	URL string `yaml:"url"` // 该题目的题解
	// Des string `yaml:"des"`
	Sol string `yaml:"sol"` // 思路
	Doc string `yaml:"doc"` // 不同于url，doc通常是blog的url
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
