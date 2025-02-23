package okr

type OKR struct {
	Item []Item `yaml:"item"`
	Year int    `yaml:"year"`
}
type Item struct {
	ID   string `yaml:"id"`
	Goal string `yaml:"goal"`
	Kr   []Kr   `yaml:"kr"`
}

type Kr struct {
	ID       string     `yaml:"id"`
	Goal     string     `yaml:"goal"`
	Tracking []Tracking `yaml:"tracking"`
}

type Tracking struct {
	Target string `yaml:"target"`
	Actual string `yaml:"actual"`
	Month  int    `yaml:"month"`
}

type OKRs []struct{}
