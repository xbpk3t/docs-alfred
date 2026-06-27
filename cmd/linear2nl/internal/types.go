package internal

// IssueView is a display-ready issue for templates.
type IssueView struct {
	Identifier string
	Title      string
	Priority   string
	TeamName   string
	DueDate    string
	URL        string
	DoneAt     string // completion time in HH:mm (CST) — used by evening
	Review     string // per-issue AI review (Markdown) — used by evening
}

// StateChangeView is a display-ready state change for templates.
type StateChangeView struct {
	IssueIdentifier string
	IssueTitle      string
	FromState       string
	ToState         string
	TeamName        string
	URL             string
	Review          string // per-issue AI review (Markdown)
}

// IssueDetail carries full issue data (description + comments) for AI review.
type IssueDetail struct {
	Identifier      string
	Title           string
	Description     string
	StateName       string
	TeamName        string
	URL             string
	Priority        string
	DueDate         string
	LinearReference string // Linear issue identifier (e.g. "ENG-123"), populated by review command
	Comments        []Comment
}

// Comment is a single comment on a Linear issue.
type Comment struct {
	Body      string
	UserName  string
	CreatedAt string
}

// EveningData is the template data for the evening report.
type EveningData struct {
	Date         string
	DayOfWeek    string
	Completed    []IssueView
	StateChanges []StateChangeView
	Stats        EveningStats
}

// EveningStats holds aggregated counts for the evening report.
type EveningStats struct {
	Completed  int
	InProgress int
}
