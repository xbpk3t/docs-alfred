package configutil

import (
	"errors"
	"fmt"
	"os"

	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/v2"
)

// LoadYAMLConfigOptions configures LoadYAMLConfig.
type LoadYAMLConfigOptions[T any] struct {
	Initial        T
	AfterUnmarshal func(*T) error
	Validate       func(*T) error
	Path           string
	Tag            string
}

const (
	StageRead      = "read"
	StageParse     = "parse"
	StageUnmarshal = "unmarshal"
	StageValidate  = "validate"
)

// LoadError records which config loading stage failed.
type LoadError struct {
	Err   error
	Stage string
}

func (e *LoadError) Error() string {
	return fmt.Sprintf("%s config: %v", e.Stage, e.Err)
}

func (e *LoadError) Unwrap() error {
	return e.Err
}

// LoadYAMLBytes parses YAML bytes into a koanf instance.
func LoadYAMLBytes(data []byte) (*koanf.Koanf, error) {
	k := koanf.New(".")
	if err := k.Load(bytesProvider(data), yaml.Parser()); err != nil {
		return nil, err
	}

	return k, nil
}

// LoadYAMLConfig loads a YAML file into a typed config value.
// If Path is empty, Initial is returned after hooks are applied.
func LoadYAMLConfig[T any](opts LoadYAMLConfigOptions[T]) (T, error) {
	cfg := opts.Initial
	tag := opts.Tag
	if tag == "" {
		tag = "yaml"
	}

	if opts.Path != "" {
		data, err := os.ReadFile(opts.Path)
		if err != nil {
			return cfg, &LoadError{Stage: StageRead, Err: err}
		}
		k, err := LoadYAMLBytes(data)
		if err != nil {
			return cfg, &LoadError{Stage: StageParse, Err: err}
		}
		if err := k.UnmarshalWithConf("", &cfg, koanf.UnmarshalConf{Tag: tag}); err != nil {
			return cfg, &LoadError{Stage: StageUnmarshal, Err: err}
		}
	}

	if opts.AfterUnmarshal != nil {
		if err := opts.AfterUnmarshal(&cfg); err != nil {
			return cfg, err
		}
	}
	if opts.Validate != nil {
		if err := opts.Validate(&cfg); err != nil {
			return cfg, &LoadError{Stage: StageValidate, Err: err}
		}
	}

	return cfg, nil
}

type bytesProvider []byte

func (p bytesProvider) ReadBytes() ([]byte, error) {
	return p, nil
}

func (p bytesProvider) Read() (map[string]any, error) {
	return nil, errors.New("bytes provider requires a parser")
}
