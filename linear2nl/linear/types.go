package linear

// Issue is a Linear issue as returned by the API.
type Issue struct {
	ID          string
	Title       string
	Identifier  string
	StateName   string
	StateType   string
	TeamName    string
	TeamKey     string
	DueDate     string
	URL         string
	UpdatedAt   string
	CompletedAt string
	Priority    float64
}

// StateChange represents a workflow state transition.
type StateChange struct {
	IssueIdentifier string
	IssueTitle      string
	FromState       string
	ToState         string
	CreatedAt       string
	TeamName        string
	TeamKey         string
	URL             string
}

// IssueDetail carries full issue data (description + comments) for AI review.
type IssueDetail struct {
	Identifier  string
	Title       string
	Description string
	StateName   string
	StateType   string
	TeamName    string
	TeamKey     string
	URL         string
	CompletedAt string
	UpdatedAt   string
	Comments    []Comment
	Priority    float64
}

// Comment is a comment on a Linear issue.
type Comment struct {
	Body      string
	UserName  string
	CreatedAt string
}
