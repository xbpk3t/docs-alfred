package books

//type BooksZ []struct {
//	Item []struct {
//		Cate  string `yaml:"cate"`
//		Sc    string `yaml:"sc"`
//		Books []struct {
//			Name   string   `yaml:"name"`
//			Author string   `yaml:"author,omitempty"`
//			Des    string   `yaml:"des,omitempty"`
//			URL    string   `yaml:"url,omitempty"`
//			Tags   []string `yaml:"tags,omitempty"`
//			Sub    []string `yaml:"sub,omitempty"`
//			Score  int      `yaml:"score,omitempty"`
//			IsOk   bool     `yaml:"isOk,omitempty"`
//			PQ4R []struct {
//				Section string `yaml:"section"`
//				Summary string `yaml:"summary,omitempty"`
//				Qs []struct{
//					Q string `yaml:"q"` // 问题
//					X string `yaml:"x,omitempty"` // 书中对该问题的解答
//					R string `yaml:"r,omitempty"` // review 反思
//				} `yaml:"qs"`
//				Review []string `yaml:"review,omitempty"`
//			} `yaml:"pq4r,omitempty"`
//		} `yaml:"books"`
//	} `yaml:"item"`
//	Year int `yaml:"year"`
//}

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
	Section string `yaml:"section"`
	Summary string `yaml:"summary,omitempty"`
	Qs      []struct {
		Q string `yaml:"q"`
		X string `yaml:"x"`
		R string `yaml:"r"`
	} `yaml:"qs,omitempty"`
	Review []string `yaml:"review,omitempty"`
}

// BookFlattenJSON JSON打平数据，方便admin使用
type BookFlattenJSON struct {
	Year string
	Type string `yaml:"type,omitempty"`
	Sub  string `yaml:"sub,omitempty"`
}
