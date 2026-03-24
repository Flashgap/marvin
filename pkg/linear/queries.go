package linear

import (
	"context"
	"fmt"

	"github.com/shurcooL/graphql"
)

type Issue struct {
	ID                 string
	Title              string
	Description        string
	Estimate           float64
	CompletedAt        string
	ProjectName        string
	ProjectID          string
	ProjectDescription string
	ProjectStartDate   string
	ProjectTargetDate  string
	ProjectCompletedAt string
	WorkspaceSlug      string
}

func (i *Issue) URL() string {
	return fmt.Sprintf("https://linear.app/%s/issue/%s", i.WorkspaceSlug, i.ID)
}

type issueQuery struct {
	Issue struct {
		Title       graphql.String
		Description graphql.String
		Estimate    graphql.Float
		CompletedAt graphql.String
		Project     struct {
			Name        graphql.String
			SlugID      graphql.String
			Description graphql.String
			StartDate   graphql.String
			TargetDate  graphql.String
			CompletedAt graphql.String
		}
	} `graphql:"issue(id: $id)"`
}

func (c *client) Issue(ctx context.Context, id string) (*Issue, error) {
	var query issueQuery
	if err := c.Query(ctx, &query, map[string]any{"id": graphql.String(id)}); err != nil {
		return nil, fmt.Errorf("error performing graphQL query: %w", err)
	}

	return &Issue{
		ID:                 id,
		Title:              string(query.Issue.Title),
		Description:        string(query.Issue.Description),
		Estimate:           float64(query.Issue.Estimate),
		CompletedAt:        string(query.Issue.CompletedAt),
		ProjectName:        string(query.Issue.Project.Name),
		ProjectID:          string(query.Issue.Project.SlugID),
		ProjectDescription: string(query.Issue.Project.Description),
		ProjectStartDate:   string(query.Issue.Project.StartDate),
		ProjectTargetDate:  string(query.Issue.Project.TargetDate),
		ProjectCompletedAt: string(query.Issue.Project.CompletedAt),
		WorkspaceSlug:      c.workspaceSlug,
	}, nil
}
