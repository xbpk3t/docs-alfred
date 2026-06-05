package internal

import "html/template"

// IssueView is a display-ready issue for templates.
type IssueView struct {
	Identifier string
	Title      string
	Priority   string
	TeamName   string
	DueDate    string
	URL        string
	Review     template.HTML // per-issue AI review (HTML)
}

// StateChangeView is a display-ready state change for templates.
type StateChangeView struct {
	IssueIdentifier string
	IssueTitle      string
	FromState       string
	ToState         string
	TeamName        string
	URL              string
	Review           template.HTML // per-issue AI review (HTML)
}

// IssueDetail carries full issue data (description + comments) for AI review.
type IssueDetail struct {
	Identifier  string
	Title       string
	Description string
	StateName   string
	TeamName    string
	URL         string
	Comments    []Comment
}

// Comment is a single comment on a Linear issue.
type Comment struct {
	Body      string
	UserName  string
	CreatedAt string
}

// MorningData is the template data for the morning report.
type MorningData struct {
	Date       string
	DayOfWeek  string
	Theme      string
	AISummary  string
	InProgress []IssueView
	Todo       []IssueView
	Stats      MorningStats
}

// MorningStats holds aggregated counts for the morning report.
type MorningStats struct {
	InProgress int
	Todo       int
	DueToday   int
}

// EveningData is the template data for the evening report.
type EveningData struct {
	Date         string
	DayOfWeek    string
	Theme        string
	AIReview     string
	Completed    []IssueView
	StateChanges []StateChangeView
	Stats        EveningStats
}

// EveningStats holds aggregated counts for the evening report.
type EveningStats struct {
	Completed  int
	InProgress int
}
