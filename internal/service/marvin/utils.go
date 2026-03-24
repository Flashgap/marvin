package marvin

import (
	"fmt"
	"strings"

	"github.com/Flashgap/marvin/internal/service/github"
	"github.com/Flashgap/marvin/internal/service/jira"
	pkggithub "github.com/Flashgap/marvin/pkg/github"
	"github.com/Flashgap/marvin/pkg/linear"
	gogithub "github.com/google/go-github/v63/github"
)

func isMarvinEvent(prEvent *gogithub.PullRequestEvent) bool {
	return strings.Contains(strings.ToLower(prEvent.GetSender().GetLogin()), GitHubAppName)
}

func IsMergeable(labels []*gogithub.Label) error {
	if !pkggithub.IsLabelInList(labels, github.LabelMerge) {
		return fmt.Errorf("PR has no merge label")
	}

	if pkggithub.IsLabelInList(labels, github.LabelDoNotMerge) {
		return fmt.Errorf("PR has Do Not Merge label")
	}

	if pkggithub.IsLabelInList(labels, github.LabelWorkInProgress) {
		return fmt.Errorf("PR has WIP label")
	}

	if pkggithub.IsLabelInList(labels, github.LabelChangesRequired) {
		return fmt.Errorf("PR has changes required label")
	}

	if pkggithub.IsLabelInList(labels, github.LabelReadyForReview) {
		return fmt.Errorf("PR has ready for review label")
	}

	return nil
}

func issueLinearToJira(issue *linear.Issue) *jira.Issue {
	return &jira.Issue{
		ID:                 issue.ID,
		Title:              issue.Title,
		Description:        issue.Description,
		Estimate:           issue.Estimate,
		CompletedAt:        issue.CompletedAt,
		ProjectName:        issue.ProjectName,
		ProjectID:          issue.ProjectID,
		ProjectDescription: issue.ProjectDescription,
		ProjectStartDate:   issue.ProjectStartDate,
		ProjectTargetDate:  issue.ProjectTargetDate,
		ProjectCompletedAt: issue.ProjectCompletedAt,
		URL:                issue.URL(),
	}
}
