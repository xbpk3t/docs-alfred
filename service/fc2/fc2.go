package fc2

type Fc2 struct {
	URL  string `yaml:"url"`
	Name string `yaml:"name,omitempty"`
	Des  string `yaml:"des,omitempty"`
	Cast []struct {
		URL   string   `yaml:"url"`
		Name  string   `yaml:"name,omitempty"`
		Des   string   `yaml:"des,omitempty"`
		Tag   []string `yaml:"tag,omitempty"`
		Score int      `yaml:"score,omitempty"`
	} `yaml:"cast,omitempty"`
}

type Fc2s []Fc2
