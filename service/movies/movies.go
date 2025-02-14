package movies

type Movie struct {
	Item []MovieItem `yaml:"item"`
	Year int         `yaml:"year"`
}

type MovieItem struct {
	Name  string   `yaml:"name"`
	Dict  string   `yaml:"dict"`
	Des   string   `yaml:"des,omitempty"`
	Tag   []string `yaml:"tag"`
	Score int      `yaml:"score"`
}

type Movies []Movie

// MovieFlatJSON 返回的flat的JSON参数
type MovieFlatJSON struct {
	MovieItem
	Year int `yaml:"year"`
}
