package diary

type Task struct {
	Task     string   `yaml:"task" json:"task"`
	Date     string   `yaml:"date,omitempty" json:"date,omitempty"`
	Pid      string   `yaml:"pid,omitempty" json:"pid,omitempty"`
	Des      string   `yaml:"des,omitempty" json:"des,omitempty"`
	Okr      string   `yaml:"okr,omitempty" json:"okr,omitempty"`
	Progress string   `yaml:"progress,omitempty" json:"progress,omitempty"`
	Item     []string `yaml:"item,omitempty" json:"item,omitempty"`
	Sub      Tasks    `yaml:"sub,omitempty" json:"sub,omitempty"`
	X        bool     `yaml:"x,omitempty" json:"x,omitempty"`
}

type Tasks []Task
