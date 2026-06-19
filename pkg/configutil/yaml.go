package configutil

import (
	"errors"
	"fmt"
	"os"

	"github.com/knadh/koanf/parsers/yaml"
	env "github.com/knadh/koanf/providers/env/v2"
	"github.com/knadh/koanf/v2"
)

// EnvOverride maps a single environment variable to a config path.
type EnvOverride struct {
	Path string
	Name string
}

// LoadYAMLConfigOptions configures LoadYAMLConfig.
type LoadYAMLConfigOptions[T any] struct {
	Initial        T
	AfterUnmarshal func(*T) error
	Validate       func(*T) error
	Path           string
	Tag            string
	EnvOverrides   []EnvOverride
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

	if err := loadConfigFromPath(&cfg, opts.Path, tag); err != nil {
		return cfg, err
	}

	if err := applyEnvOverrides(&cfg, tag, opts.EnvOverrides); err != nil {
		return cfg, &LoadError{Stage: StageUnmarshal, Err: err}
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

func loadConfigFromPath[T any](cfg *T, path, tag string) error {
	if path == "" {
		return nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return &LoadError{Stage: StageRead, Err: err}
	}
	k, err := LoadYAMLBytes(data)
	if err != nil {
		return &LoadError{Stage: StageParse, Err: err}
	}
	if err := k.UnmarshalWithConf("", cfg, koanf.UnmarshalConf{Tag: tag}); err != nil {
		return &LoadError{Stage: StageUnmarshal, Err: err}
	}

	return nil
}

func applyEnvOverrides[T any](cfg *T, tag string, overrides []EnvOverride) error {
	if len(overrides) == 0 {
		return nil
	}

	k := koanf.New(".")
	if err := k.Load(env.Provider(".", env.Opt{TransformFunc: func(key, value string) (string, any) {
		if value == "" {
			return "", nil
		}
		for _, override := range overrides {
			if override.Name == key {
				return override.Path, value
			}
		}

		return "", nil
	}}), nil); err != nil {
		return err
	}
	if len(k.Keys()) == 0 {
		return nil
	}

	return k.UnmarshalWithConf("", cfg, koanf.UnmarshalConf{Tag: tag})
}

type bytesProvider []byte

func (p bytesProvider) ReadBytes() ([]byte, error) {
	return p, nil
}

func (p bytesProvider) Read() (map[string]any, error) {
	return nil, errors.New("bytes provider requires a parser")
}
