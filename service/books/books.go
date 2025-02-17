package books

type BooksZ []struct {
	Item []struct {
		Cate  string `yaml:"cate"`
		Sub   string `yaml:"sub,omitempty"`
		Books []struct {
			Name   string   `yaml:"name"`
			Author string   `yaml:"author,omitempty"`
			Des    string   `yaml:"des,omitempty"`
			URL    string   `yaml:"url,omitempty"`
			Tags   []string `yaml:"tags,omitempty"`
			Score  int      `yaml:"score,omitempty"`
			IsOk   bool     `yaml:"isOk,omitempty"`
		} `yaml:"books"`
	} `yaml:"item"`
	Year int `yaml:"year"`
}

// BookFlattenJSON JSON打平数据，方便admin使用
type BookFlattenJSON struct {
	Year string
	Type string `yaml:"type,omitempty"`
	Sub  string `yaml:"sub,omitempty"`
}
