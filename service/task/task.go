package task

type Task struct {
	Task   string   `yaml:"task" json:"task"`
	Date   string   `yaml:"date,omitempty" json:"date,omitempty"`
	Pid    string   `yaml:"pid,omitempty" json:"pid,omitempty"` // 用来标明该task的所属pid
	Des    string   `yaml:"des,omitempty" json:"des,omitempty"`
	Okr    string   `yaml:"okr,omitempty" json:"okr,omitempty"`       // 用来标明该task的所属okr
	Review []string `yaml:"review,omitempty" json:"review,omitempty"` // 类似上面的Item，但是是用来记录和复盘的 // 附加内容，类似tracking。用来标识该task的一些metrics之类的。比如说milestone类型的task（比如本月开销，购买了哪些东西，就可以写到item里）
	Sub    Tasks    `yaml:"sub,omitempty" json:"sub,omitempty"`
	IsX    bool     `yaml:"x,omitempty" json:"isX,omitempty"` // 是否完成，默认true。如果false就是没完成
}

type Tasks []Task
