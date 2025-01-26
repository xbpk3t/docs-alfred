package movies

type Movie struct {
	Item []struct {
		Name  string   `yaml:"name"`
		Dict  string   `yaml:"dict"`
		Des   string   `yaml:"des,omitempty"`
		Tag   []string `yaml:"tag"`
		Score int      `yaml:"score"`
	} `yaml:"item"`
	Year int `yaml:"year"`
}

type Movies []Movie
