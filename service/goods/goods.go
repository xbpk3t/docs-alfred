package goods

// Goods 定义商品配置结构
type Goods struct {
	Type  string `yaml:"type"`
	Tag   string `yaml:"tag"`
	Items []Item `yaml:"goods"`
	Des   string `yaml:"des,omitempty"`
	QA    []QA   `yaml:"qs,omitempty"`
}

// Item 定义单个商品项
type Item struct {
	Name  string   `yaml:"name"`
	Param string   `yaml:"param,omitempty"`
	Price string   `yaml:"price,omitempty"`
	Des   string   `yaml:"des,omitempty"`
	URL   string   `yaml:"url,omitempty"`
	Date  []string `yaml:"date,omitempty"`
	Use   bool     `yaml:"use,omitempty"`
}

// QA 定义问答结构
type QA struct {
	Question     string   `yaml:"q"`
	Answer       string   `yaml:"x"`
	SubQuestions []string `yaml:"s"`
}
