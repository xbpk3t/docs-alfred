package feed

import (
	"github.com/xbpk3t/docs-alfred/pkg"
)

type Feed struct {
	Feed string `yaml:"feed"`
	Des  string `yaml:"des"`
	URL  string `yaml:"url"`
	Name string `yaml:"name"`
}

type Categories struct {
	Type  string        `yaml:"type"`
	Feeds []pkg.URLInfo `yaml:"feeds"`
}
