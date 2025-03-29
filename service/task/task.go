package task

import (
	"sort"
	"time"

	"github.com/xbpk3t/docs-alfred/service/gh"
)

type Task struct {
	Task   string    `yaml:"task" json:"task"`
	URL    string    `yaml:"url" json:"url,omitempty"`
	Date   string    `yaml:"date,omitempty" json:"date,omitempty"`
	Pid    string    `yaml:"pid,omitempty" json:"pid,omitempty"`       // 用来标明该task的所属pid
	Des    string    `yaml:"des,omitempty" json:"des,omitempty"`       // 用来说明Task
	Review string    `yaml:"review,omitempty" json:"review,omitempty"` // 类似上面的Item，但是是用来记录和复盘的 // 附加内容，类似tracking。用来标识该task的一些metrics之类的。比如说milestone类型的task（比如本月开销，购买了哪些东西，就可以写到item里）
	Sub    Tasks     `yaml:"sub,omitempty" json:"sub,omitempty"`
	Item   []string  `yaml:"item,omitempty" json:"item,omitempty"`
	Qs     []string  `yaml:"qs,omitempty" json:"qs,omitempty"`
	Topics gh.Topics `yaml:"topics,omitempty" json:"topics,omitempty"`
	IsX    bool      `yaml:"isX,omitempty" json:"isX,omitempty"` // 是否完成，默认true。如果false就是没完成
}

type Tasks []Task

// TaskOption defines a function type that modifies a Task
type TaskOption func(*Task)

// WithParentID sets the parent ID for a task and its sub-tasks
func WithParentID() TaskOption {
	return func(t *Task) {
		if len(t.Sub) > 0 {
			for i := range t.Sub {
				t.Sub[i].Pid = t.Pid
				t.Sub[i].ApplyOptions(WithParentID())
			}
		}
	}
}

// SortMainTasksByDate sorts main tasks by date
func SortMainTasksByDate(ascending bool) TaskOption {
	return func(t *Task) {
		if len(t.Sub) > 0 {
			sort.Slice(t.Sub, func(i, j int) bool {
				if t.Sub[i].Date == "" {
					return false
				}
				if t.Sub[j].Date == "" {
					return true
				}
				dateI, _ := time.Parse(time.DateOnly, t.Sub[i].Date)
				dateJ, _ := time.Parse(time.DateOnly, t.Sub[j].Date)
				if ascending {
					return dateI.Before(dateJ)
				}
				return dateI.After(dateJ)
			})
		}
	}
}

// SortSubTasksByDate sorts sub-tasks by date
func SortSubTasksByDate(ascending bool) TaskOption {
	return func(t *Task) {
		if len(t.Sub) > 0 {
			for i := range t.Sub {
				t.Sub[i].ApplyOptions(SortSubTasksByDate(ascending))
			}
		}
	}
}

// ApplyOptions applies the given options to the task
func (t *Task) ApplyOptions(opts ...TaskOption) {
	for _, opt := range opts {
		opt(t)
	}
}

// ApplyOptions applies the given options to all tasks in the slice
func (ts Tasks) ApplyOptions(opts ...TaskOption) {
	for i := range ts {
		ts[i].ApplyOptions(opts...)
	}
}
