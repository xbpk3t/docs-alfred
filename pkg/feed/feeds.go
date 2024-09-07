package feed

import (
	"github.com/pkg/errors"
	"gopkg.in/yaml.v3"
)

type Categories struct {
	Type  string `yaml:"type"`
	Feeds []Feed `yaml:"feeds"`
}

type Feed struct {
	Feed string `yaml:"feed"`
	Des  string `yaml:"des"`
	URL  string `yaml:"url"`
	Name string `yaml:"name"`
}

func NewConfigFeeds(data []byte) ([]Categories, error) {
	var cates []Categories
	err := yaml.Unmarshal(data, &cates)
	if err != nil {
		return nil, errors.Wrap(err, "")
	}

	return cates, nil
}
