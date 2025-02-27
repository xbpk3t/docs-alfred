package books

type BooksZ []struct {
	novel struct {
		webnovel []Book `yaml:"webnovel"`
		erotic   []Book `yaml:"erotic"`
		classic  []Book `yaml:"classic"`
	} `yaml:"novel"`
	coding struct {
		langs  []Book `yaml:"langs"`
		kernel []Book `yaml:"kernel"`
		arch   []Book `yaml:"arch"`
	} `yaml:"coding"`
	ss struct {
		xxx      []Book `yaml:"xxx"`
		politics []Book `yaml:"politics"`
	} `yaml:"ss"`
	year int `yaml:"year"`
}

// Book 定义通用的书籍结构体
type Book struct {
	Name    string   `yaml:"name"`
	Author  string   `yaml:"author,omitempty"`
	Des     string   `yaml:"des,omitempty"`
	URL     string   `yaml:"url,omitempty"`
	Tags    []string `yaml:"tags,omitempty"`
	Pq4R    []Pq4R   `yaml:"pq4r,omitempty"`
	Summary []string `yaml:"summary,omitempty"`
	Item    []string `yaml:"item,omitempty"`
	Score   int      `yaml:"score,omitempty"` // score: -1 就是之前的isOk: false
}

// Pq4R 定义 PQ4R 结构体
type Pq4R struct {
	Section string   `yaml:"section"`
	Summary string   `yaml:"summary,omitempty"`
	Qs      []string `yaml:"qs,omitempty"`
	Review  []string `yaml:"review,omitempty"`
}

// BookFlattenJSON JSON打平数据，方便admin使用
type BookFlattenJSON struct {
	Year string
	Type string `yaml:"type,omitempty"`
	Sub  string `yaml:"sub,omitempty"`
}
