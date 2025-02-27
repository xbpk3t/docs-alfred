package video

type TV struct {
	Item []struct {
		Name  string   `yaml:"name"`
		Dict  string   `yaml:"dict,omitempty"`
		Des   string   `yaml:"des"`
		Tags  []string `yaml:"tags,omitempty"`
		Score int      `yaml:"score,omitempty"`
	} `yaml:"item"`
	Year int `yaml:"year"`
}

type TVs []TV
