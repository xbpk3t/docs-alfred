package compact

import (
	"context"
	"fmt"
	"strings"
	"time"

	carbon "github.com/dromara/carbon/v2"
	"github.com/xbpk3t/docs-alfred/internal/linear"
)

// Product defaults for compact Linear issues (yaml may override).
const (
	DefaultLinearTeamKey   = "LUC"
	DefaultLinearStateName = "In Review"
	DefaultLinearPriority  = 2 // High
	DefaultLinearAssignee  = "viewer"
)

// LinearConfig holds Linear create-issue parameters (API key from env only).
type LinearConfig struct {
	// NewClient is an optional factory for tests; nil uses linear.NewClient.
	NewClient func(apiKey string, teamKeys []string) LinearIssueCreator
	APIKey    string
	TeamKey   string
	// TeamID skips ResolveTeamID when set (tests / explicit config).
	TeamID string
	// StateName is resolved to stateId on the team (default In Review).
	StateName string
	// Assignee: "viewer" (default), "none", or a Linear user id.
	Assignee string
	// Priority is Linear priority 0–4 (default 2 High). Negative → omit on API.
	Priority int
	// SkipState leaves team default workflow state.
	SkipState bool
}

// LinearIssueCreator is the subset of linear.Client used by compact delivery.
type LinearIssueCreator interface {
	// CreateIssue creates a Linear issue from input.
	CreateIssue(ctx context.Context, in *linear.CreateIssueInput) (*linear.Issue, error)
	// ResolveTeamID maps a team key (e.g. LUC) to a team UUID.
	ResolveTeamID(ctx context.Context, teamKey string) (string, error)
	// ResolveStateID maps a workflow state name on a team to a state UUID.
	ResolveStateID(ctx context.Context, teamID, stateName string) (string, error)
	// ViewerID returns the authenticated API key user's id.
	ViewerID(ctx context.Context) (string, error)
}

// NormalizeLinearConfig fills empty product defaults in place.
// Single source for teamKey / stateName / assignee defaults.
// Priority is left as provided (ingest creasty default is 2; 0=None is valid).
func NormalizeLinearConfig(cfg *LinearConfig) {
	if cfg == nil {
		return
	}
	if strings.TrimSpace(cfg.TeamKey) == "" {
		cfg.TeamKey = DefaultLinearTeamKey
	}
	if !cfg.SkipState && strings.TrimSpace(cfg.StateName) == "" {
		cfg.StateName = DefaultLinearStateName
	}
	if cfg.Assignee == "" {
		cfg.Assignee = DefaultLinearAssignee
	}
}

// RenderCompactIssueTitle builds the Linear issue title for a run date.
// Format: {brand} [YYYY-MM-DD] (Asia/Shanghai via carbon). brand empty → DefaultBrand.
func RenderCompactIssueTitle(title string, date time.Time) string {
	day := carbon.CreateFromStdTime(date).ToDateString()
	return fmt.Sprintf("%s [%s]", CompactBrand(title), day)
}

// CreateCompactIssue always creates a new Linear issue with the compact TextBody.
// Defaults: priority High, state In Review, assignee = API key viewer.
func CreateCompactIssue(ctx context.Context, cfg *LinearConfig, title, description string) (*linear.Issue, error) {
	if cfg == nil {
		return nil, fmt.Errorf("linear config is required")
	}
	if strings.TrimSpace(cfg.APIKey) == "" {
		return nil, fmt.Errorf("linear api key is required")
	}
	if strings.TrimSpace(title) == "" {
		return nil, fmt.Errorf("linear issue title is required")
	}
	NormalizeLinearConfig(cfg)

	client := newLinearCreator(cfg)

	teamID := strings.TrimSpace(cfg.TeamID)
	if teamID == "" {
		id, err := client.ResolveTeamID(ctx, cfg.TeamKey)
		if err != nil {
			return nil, err
		}
		teamID = id
	}

	in := &linear.CreateIssueInput{
		TeamID:      teamID,
		Title:       title,
		Description: description,
		Priority:    cfg.Priority,
	}

	if !cfg.SkipState {
		stateID, err := client.ResolveStateID(ctx, teamID, cfg.StateName)
		if err != nil {
			return nil, err
		}
		in.StateID = stateID
	}

	switch {
	case strings.EqualFold(cfg.Assignee, DefaultLinearAssignee), strings.EqualFold(cfg.Assignee, "me"):
		viewerID, err := client.ViewerID(ctx)
		if err != nil {
			return nil, err
		}
		in.AssigneeID = viewerID
	case strings.EqualFold(cfg.Assignee, "none"), strings.EqualFold(cfg.Assignee, "unassigned"):
		// leave unassigned
	default:
		in.AssigneeID = strings.TrimSpace(cfg.Assignee)
	}

	return client.CreateIssue(ctx, in)
}

func newLinearCreator(cfg *LinearConfig) LinearIssueCreator {
	if cfg.NewClient != nil {
		return cfg.NewClient(cfg.APIKey, nil)
	}
	return linear.NewClient(cfg.APIKey, nil)
}
