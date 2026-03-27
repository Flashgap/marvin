//go:generate mockgen --source=$GOFILE --destination=mock/mock.go --package mock_github
package github

import (
	"context"

	"github.com/google/go-github/v63/github"
)

type Client interface {
	ListLabels(ctx context.Context, webhook RepoSenderGetter, opts *github.ListOptions) ([]*github.Label, *github.Response, error)
	AddAssignees(ctx context.Context, webhook RepoSenderGetter, prNumber int, assignees []string) (*github.Issue, *github.Response, error)
	ListTeamMembers(ctx context.Context, webhook RepoSenderGetter, teamSlug string, opts *github.TeamListTeamMembersOptions) ([]*github.User, *github.Response, error)
	GetBranchProtection(ctx context.Context, webhook RepoSenderGetter, branch string) (*github.Protection, *github.Response, error)
	GetRulesForBranch(ctx context.Context, webhook RepoSenderGetter, branch string) ([]*github.RepositoryRule, *github.Response, error)
	ListReviewers(ctx context.Context, webhook RepoSenderGetter, number int, opts *github.ListOptions) (*github.Reviewers, *github.Response, error)
	ListReviews(ctx context.Context, webhook RepoSenderGetter, prNumber int, opts *github.ListOptions) ([]*github.PullRequestReview, *github.Response, error)
	ListCheckRunsForRef(ctx context.Context, webhook RepoSenderGetter, ref string, opts *github.ListCheckRunsOptions) (*github.ListCheckRunsResults, *github.Response, error)
	PR(ctx context.Context, webhook RepoSenderGetter, number int) (*github.PullRequest, *github.Response, error)
	GetCommit(ctx context.Context, webhook RepoSenderGetter, sha string, opts *github.ListOptions) (*github.RepositoryCommit, *github.Response, error)
	CreateCheckRun(ctx context.Context, webhook RepoSenderGetter, opts github.CreateCheckRunOptions) (*github.CheckRun, *github.Response, error)
	RequestReviewers(ctx context.Context, webhook RepoSenderGetter, prNumber int, reviewers []string) (*github.PullRequest, *github.Response, error)
	ListPR(ctx context.Context, webhook RepoSenderGetter, opts *github.PullRequestListOptions) ([]*github.PullRequest, *github.Response, error)
	ListPRFiles(ctx context.Context, webhook RepoSenderGetter, prNumber int, opts *github.ListOptions) ([]*github.CommitFile, *github.Response, error)
	ListPRCommits(ctx context.Context, webhook RepoSenderGetter, prNumber int, opts *github.ListOptions) ([]*github.RepositoryCommit, *github.Response, error)
	ListCheckSuites(ctx context.Context, prEvent *github.PullRequestEvent, opts *github.ListCheckSuiteOptions) (*github.ListCheckSuiteResults, *github.Response, error)
	RemovePRLabel(ctx context.Context, webhook RepoSenderGetter, prNumber int, label string) (*github.Response, error)
	AddPRLabels(ctx context.Context, webhook RepoSenderGetter, prNumber int, labels []string) ([]*github.Label, *github.Response, error)
	CreatePRComment(ctx context.Context, webhook RepoSenderGetter, prNumber int, body *github.IssueComment) (*github.IssueComment, *github.Response, error)
	MergePR(ctx context.Context, webhook RepoSenderGetter, prNumber int, commitMsg string, opts *github.PullRequestOptions) (*github.PullRequestMergeResult, *github.Response, error)
	EditPR(ctx context.Context, webhook RepoSenderGetter, prNumber int, body *github.PullRequest) (*github.PullRequest, *github.Response, error)
}
