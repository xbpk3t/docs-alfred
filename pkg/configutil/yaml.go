package configutil

import (
	"errors"

	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/v2"
)

// LoadYAMLBytes parses YAML bytes into a koanf instance.
func LoadYAMLBytes(data []byte) (*koanf.Koanf, error) {
	k := koanf.New(".")
	if err := k.Load(bytesProvider(data), yaml.Parser()); err != nil {
		return nil, err
	}

	return k, nil
}

type bytesProvider []byte

func (p bytesProvider) ReadBytes() ([]byte, error) {
	return p, nil
}

func (p bytesProvider) Read() (map[string]interface{}, error) {
	return nil, errors.New("bytes provider requires a parser")
}
