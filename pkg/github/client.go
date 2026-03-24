package github

import (
	"context"

	"github.com/google/go-github/v63/github"
)

// RepoSenderGetter is the missing interface from GitHub sdk. It allows us to get data on all webhook types.
type RepoSenderGetter interface {
	GetRepo() *github.Repository
	GetSender() *github.User
}

type client struct {
	*github.Client
}

func NewClient(ghClient *github.Client) Client {
	return &client{Client: ghClient}
}

func (h *client) ListLabels(ctx context.Context, webhook RepoSenderGetter, opts *github.ListOptions) ([]*github.Label, *github.Response, error) {
	return h.Issues.ListLabels(ctx, webhook.GetRepo().GetOwner().GetLogin(), webhook.GetRepo().GetName(), opts)
}

func (h *client) AddAssignees(ctx context.Context, webhook RepoSenderGetter, prNumber int, assignees []string) (*github.Issue, *github.Response, error) {
	return h.Issues.AddAssignees(ctx, webhook.GetRepo().GetOwner().GetLogin(), webhook.GetRepo().GetName(), prNumber, assignees)
}

func (h *client) ListTeamMembers(ctx context.Context, webhook RepoSenderGetter, teamSlug string, opts *github.TeamListTeamMembersOptions) ([]*github.User, *github.Response, error) {
	return h.Teams.ListTeamMembersBySlug(ctx, webhook.GetRepo().GetOwner().GetLogin(), teamSlug, opts)
}

func (h *client) GetBranchProtection(ctx context.Context, webhook RepoSenderGetter, branch string) (*github.Protection, *github.Response, error) {
	return h.Repositories.GetBranchProtection(ctx, webhook.GetRepo().GetOwner().GetLogin(), webhook.GetRepo().GetName(), branch)
}

func (h *client) ListReviewers(ctx context.Context, webhook RepoSenderGetter, prNumber int, opts *github.ListOptions) (*github.Reviewers, *github.Response, error) {
	return h.PullRequests.ListReviewers(ctx, webhook.GetRepo().GetOwner().GetLogin(), webhook.GetRepo().GetName(), prNumber, opts)
}

func (h *client) ListReviews(ctx context.Context, webhook RepoSenderGetter, prNumber int, opts *github.ListOptions) ([]*github.PullRequestReview, *github.Response, error) {
	return h.PullRequests.ListReviews(ctx, webhook.GetRepo().GetOwner().GetLogin(), webhook.GetRepo().GetName(), prNumber, opts)
}

func (h *client) ListCheckRunsForRef(ctx context.Context, webhook RepoSenderGetter, ref string, opts *github.ListCheckRunsOptions) (*github.ListCheckRunsResults, *github.Response, error) {
	return h.Checks.ListCheckRunsForRef(ctx, webhook.GetRepo().GetOwner().GetLogin(), webhook.GetRepo().GetName(), ref, opts)
}

func (h *client) PR(ctx context.Context, webhook RepoSenderGetter, number int) (*github.PullRequest, *github.Response, error) {
	return h.PullRequests.Get(ctx, webhook.GetRepo().GetOwner().GetLogin(), webhook.GetRepo().GetName(), number)
}

func (h *client) GetCommit(ctx context.Context, webhook RepoSenderGetter, sha string, opts *github.ListOptions) (*github.RepositoryCommit, *github.Response, error) {
	return h.Repositories.GetCommit(ctx, webhook.GetRepo().GetOwner().GetLogin(), webhook.GetRepo().GetName(), sha, opts)
}

func (h *client) CreateCheckRun(ctx context.Context, webhook RepoSenderGetter, opts github.CreateCheckRunOptions) (*github.CheckRun, *github.Response, error) {
	return h.Checks.CreateCheckRun(ctx, webhook.GetRepo().GetOwner().GetLogin(), webhook.GetRepo().GetName(), opts)
}

func (h *client) RequestReviewers(ctx context.Context, webhook RepoSenderGetter, prNumber int, reviewers []string) (*github.PullRequest, *github.Response, error) {
	opts := github.ReviewersRequest{
		Reviewers: reviewers,
	}

	return h.PullRequests.RequestReviewers(ctx, webhook.GetRepo().GetOwner().GetLogin(), webhook.GetRepo().GetName(), prNumber, opts)
}

func (h *client) ListPR(ctx context.Context, webhook RepoSenderGetter, opts *github.PullRequestListOptions) ([]*github.PullRequest, *github.Response, error) {
	return h.PullRequests.List(ctx, webhook.GetRepo().GetOwner().GetLogin(), webhook.GetRepo().GetName(), opts)
}

func (h *client) ListPRFiles(ctx context.Context, webhook RepoSenderGetter, prNumber int, opts *github.ListOptions) ([]*github.CommitFile, *github.Response, error) {
	return h.PullRequests.ListFiles(ctx, webhook.GetRepo().GetOwner().GetLogin(), webhook.GetRepo().GetName(), prNumber, opts)
}

func (h *client) ListPRCommits(ctx context.Context, webhook RepoSenderGetter, prNumber int, opts *github.ListOptions) ([]*github.RepositoryCommit, *github.Response, error) {
	return h.PullRequests.ListCommits(ctx, webhook.GetRepo().GetOwner().GetLogin(), webhook.GetRepo().GetName(), prNumber, opts)
}

func (h *client) ListCheckSuites(ctx context.Context, prEvent *github.PullRequestEvent, opts *github.ListCheckSuiteOptions) (*github.ListCheckSuiteResults, *github.Response, error) {
	return h.Checks.ListCheckSuitesForRef(ctx, prEvent.Organization.GetLogin(), prEvent.Repo.GetName(), prEvent.PullRequest.GetHead().GetSHA(), opts)
}

func (h *client) RemovePRLabel(ctx context.Context, webhook RepoSenderGetter, prNumber int, label string) (*github.Response, error) {
	return h.Issues.RemoveLabelForIssue(ctx, webhook.GetRepo().GetOwner().GetLogin(), webhook.GetRepo().GetName(), prNumber, label)
}

func (h *client) AddPRLabels(ctx context.Context, webhook RepoSenderGetter, prNumber int, labels []string) ([]*github.Label, *github.Response, error) {
	return h.Issues.AddLabelsToIssue(ctx, webhook.GetRepo().GetOwner().GetLogin(), webhook.GetRepo().GetName(), prNumber, labels)
}

func (h *client) CreatePRComment(ctx context.Context, webhook RepoSenderGetter, prNumber int, body *github.IssueComment) (*github.IssueComment, *github.Response, error) {
	return h.Issues.CreateComment(ctx, webhook.GetRepo().GetOwner().GetLogin(), webhook.GetRepo().GetName(), prNumber, body)
}

func (h *client) MergePR(ctx context.Context, webhook RepoSenderGetter, prNumber int, commitMsg string, opts *github.PullRequestOptions) (*github.PullRequestMergeResult, *github.Response, error) {
	return h.PullRequests.Merge(ctx, webhook.GetRepo().GetOwner().GetLogin(), webhook.GetRepo().GetName(), prNumber, commitMsg, opts)
}

func (h *client) EditPR(ctx context.Context, webhook RepoSenderGetter, prNumber int, body *github.PullRequest) (*github.PullRequest, *github.Response, error) {
	return h.PullRequests.Edit(ctx, webhook.GetRepo().GetOwner().GetLogin(), webhook.GetRepo().GetName(), prNumber, body)
}
