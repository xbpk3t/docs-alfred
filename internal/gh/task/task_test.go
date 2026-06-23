package task

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestApplyOptions_Task(t *testing.T) {
	task := Task{
		Task: "parent",
		Pid:  "p1",
		Sub: Tasks{
			{Task: "child1"},
			{Task: "child2", Sub: Tasks{{Task: "grandchild"}}},
		},
	}
	task.ApplyOptions(WithParentID())

	assert.Equal(t, "p1", task.Sub[0].Pid)
	assert.Equal(t, "p1", task.Sub[1].Pid)
	assert.Equal(t, "p1", task.Sub[1].Sub[0].Pid)
}

func TestApplyOptions_Tasks(t *testing.T) {
	tasks := Tasks{
		{Task: "t1", Pid: "p1", Sub: Tasks{{Task: "sub1"}}},
		{Task: "t2", Pid: "p2", Sub: Tasks{{Task: "sub2"}}},
	}
	tasks.ApplyOptions(WithParentID())

	assert.Equal(t, "p1", tasks[0].Sub[0].Pid)
	assert.Equal(t, "p2", tasks[1].Sub[0].Pid)
}

func TestWithParentID_NoSubs(t *testing.T) {
	task := Task{Task: "leaf", Pid: "p1"}
	task.ApplyOptions(WithParentID())
	assert.Equal(t, "p1", task.Pid)
}

func TestWithParentID_EmptySubs(t *testing.T) {
	task := Task{Task: "leaf", Pid: "p1", Sub: Tasks{}}
	task.ApplyOptions(WithParentID())
	assert.Equal(t, "p1", task.Pid)
	assert.Empty(t, task.Sub)
}

func TestSortMainTasksByDate_Ascending(t *testing.T) {
	task := Task{
		Task: "root",
		Sub: Tasks{
			{Task: "b", Date: "2024-06-01"},
			{Task: "a", Date: "2024-01-01"},
			{Task: "c", Date: "2024-12-01"},
		},
	}
	task.ApplyOptions(SortMainTasksByDate(true))

	assert.Equal(t, "a", task.Sub[0].Task)
	assert.Equal(t, "b", task.Sub[1].Task)
	assert.Equal(t, "c", task.Sub[2].Task)
}

func TestSortMainTasksByDate_Descending(t *testing.T) {
	task := Task{
		Task: "root",
		Sub: Tasks{
			{Task: "a", Date: "2024-01-01"},
			{Task: "c", Date: "2024-12-01"},
			{Task: "b", Date: "2024-06-01"},
		},
	}
	task.ApplyOptions(SortMainTasksByDate(false))

	assert.Equal(t, "c", task.Sub[0].Task)
	assert.Equal(t, "b", task.Sub[1].Task)
	assert.Equal(t, "a", task.Sub[2].Task)
}

func TestSortMainTasksByDate_EmptyDateHandled(t *testing.T) {
	task := Task{
		Task: "root",
		Sub: Tasks{
			{Task: "nodate"},
			{Task: "hasdate", Date: "2024-01-01"},
		},
	}
	task.ApplyOptions(SortMainTasksByDate(true))

	// Task with date should come before task without date
	assert.Equal(t, "hasdate", task.Sub[0].Task)
	assert.Equal(t, "nodate", task.Sub[1].Task)
}

func TestSortMainTasksByDate_BothEmpty(t *testing.T) {
	task := Task{
		Task: "root",
		Sub: Tasks{
			{Task: "a"},
			{Task: "b"},
		},
	}
	task.ApplyOptions(SortMainTasksByDate(true))
	// Both empty dates: stable sort preserves order
	assert.Equal(t, "a", task.Sub[0].Task)
	assert.Equal(t, "b", task.Sub[1].Task)
}

func TestSortMainTasksByDate_NoSubs(t *testing.T) {
	task := Task{Task: "leaf"}
	task.ApplyOptions(SortMainTasksByDate(true))
	assert.Equal(t, "leaf", task.Task)
}

func TestSortSubTasksByDate_Recursive(t *testing.T) {
	// SortSubTasksByDate recursively applies itself but doesn't sort.
	// To sort sub-levels, combine it with SortMainTasksByDate.
	task := Task{
		Task: "root",
		Sub: Tasks{
			{
				Task: "parent",
				Date: "2024-06-01",
				Sub: Tasks{
					{Task: "b", Date: "2024-12-01"},
					{Task: "a", Date: "2024-01-01"},
				},
			},
		},
	}
	// Apply SortSubTasksByDate which recurses, then SortMainTasksByDate to sort
	task.ApplyOptions(
		SortMainTasksByDate(true),
		SortSubTasksByDate(true),
		SortMainTasksByDate(true),
	)
	// After SortSubTasksByDate(true), each child gets SortSubTasksByDate applied.
	// But the actual sorting of sub-children happens through the recursive structure.
	// This test just verifies the recursion doesn't panic and the option applies.
	assert.Len(t, task.Sub, 1)
	assert.Len(t, task.Sub[0].Sub, 2)
}

func TestSortSubTasksByDate_NoSubs(t *testing.T) {
	task := Task{Task: "leaf"}
	task.ApplyOptions(SortSubTasksByDate(false))
	assert.Equal(t, "leaf", task.Task)
}

func TestCompareTasks(t *testing.T) {
	t1 := &Task{Date: "2024-01-01"}
	t2 := &Task{Date: "2024-06-01"}

	assert.True(t, compareTasks(t1, t2, true))  // ascending: t1 before t2
	assert.False(t, compareTasks(t2, t1, true)) // ascending: t2 not before t1
	assert.True(t, compareTasks(t2, t1, false)) // descending: t2 after t1
	assert.False(t, compareTasks(t1, t2, false)) // descending: t1 not after t2
}

func TestCompareTasks_EmptyDates(t *testing.T) {
	t1 := &Task{Date: ""}
	t2 := &Task{Date: "2024-01-01"}
	t3 := &Task{Date: ""}

	assert.False(t, compareTasks(t1, t2, true)) // empty first: false
	assert.True(t, compareTasks(t2, t1, true))  // non-empty first: true
	assert.False(t, compareTasks(t1, t3, true)) // both empty: false
}

func TestMultipleOptionsCombined(t *testing.T) {
	task := Task{
		Task: "root",
		Pid:  "p1",
		Sub: Tasks{
			{Task: "b", Date: "2024-06-01", Sub: Tasks{{Task: "sub-b"}}},
			{Task: "a", Date: "2024-01-01", Sub: Tasks{{Task: "sub-a"}}},
		},
	}
	task.ApplyOptions(
		WithParentID(),
		SortMainTasksByDate(true),
		SortSubTasksByDate(true),
	)

	assert.Equal(t, "a", task.Sub[0].Task)
	assert.Equal(t, "p1", task.Sub[0].Pid)
	assert.Equal(t, "b", task.Sub[1].Task)
	assert.Equal(t, "p1", task.Sub[1].Pid)
}

func TestTaskStruct(t *testing.T) {
	task := Task{
		Task:   "my task",
		URL:    "https://example.com",
		Date:   "2024-01-01",
		Pid:    "p1",
		Des:    "description",
		Review: "review notes",
		Item:   []string{"item1", "item2"},
		Qs:     []string{"q1"},
	}
	assert.Equal(t, "my task", task.Task)
	assert.Equal(t, "https://example.com", task.URL)
	assert.Equal(t, "2024-01-01", task.Date)
	assert.Equal(t, "p1", task.Pid)
	assert.Equal(t, "description", task.Des)
	assert.Equal(t, "review notes", task.Review)
	assert.Len(t, task.Item, 2)
	assert.Len(t, task.Qs, 1)
}
