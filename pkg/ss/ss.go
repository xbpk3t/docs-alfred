package ss

import (
	"bytes"
	"errors"
	"io"

	"gopkg.in/yaml.v3"
)

type SS []struct {
	Type string `yaml:"type"`
	Tag  string `yaml:"tag"`
	Qs   Qs
}

type Qs []struct {
	Q string `yaml:"q,omitempty"`
	X string `yaml:"x,omitempty"`
	U string `yaml:"u,omitempty"`
}

func NewSS(f []byte) SS {
	var ghs SS

	d := yaml.NewDecoder(bytes.NewReader(f))
	for {
		// create new spec here
		spec := new(SS)
		// pass a reference to spec reference
		if err := d.Decode(&spec); err != nil {
			// break the loop in case of EOF
			if errors.Is(err, io.EOF) {
				break
			}
			panic(err)
		}
		ghs = append(ghs, *spec...)
	}

	return ghs
}
