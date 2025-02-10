package poem

type Poems []Poem

type Poem struct {
	Author  string `yaml:"author"`
	Content string `yaml:"content"`
	Name    string `yaml:"name"`
}
