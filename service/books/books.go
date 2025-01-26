package books

type Books struct {
	Year      string
	BookTypes []BookType
}

type BookType struct {
	Tag   string `yaml:"tag"`
	Type  string `yaml:"type,omitempty"`
	Books []struct {
		Name   string     `yaml:"name"`
		Author string     `yaml:"author,omitempty"`
		URL    string     `yaml:"url,omitempty"`
		Des    string     `yaml:"des,omitempty"`
		Date   []struct{} `yaml:"date,omitempty"`
		Score  int        `yaml:"score,omitempty"`
	} `yaml:"books,omitempty"`
}
