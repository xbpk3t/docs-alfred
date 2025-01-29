package tv

type TV struct {
	Item []struct {
		Date  struct{} `yaml:"date,omitempty"`
		Name  string   `yaml:"name"`
		Dict  string   `yaml:"dict,omitempty"`
		Des   string   `yaml:"des"`
		Tag   []string `yaml:"tag,omitempty"`
		Score int      `yaml:"score,omitempty"`
	} `yaml:"item"`
	Year int `yaml:"year"`
}

type TVs []TV
