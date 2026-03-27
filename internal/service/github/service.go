package github

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"

	gogithub "github.com/google/go-github/v63/github"

	"github.com/Flashgap/marvin/internal/middlewares"
	"github.com/Flashgap/marvin/pkg/github"
	"github.com/Flashgap/marvin/pkg/utils"
)

const (
	CommitTitleSizeLimit = 72
	MinTimeSpent         = 0.25

	mainBranchName = "main"
)

var (
	ErrSectionNotFound      = errors.New("cannot find section")
	ErrInvalidDescription   = errors.New("wrong description section")
	ErrLinearLink           = errors.New("wrong issue link")
	ErrTimeSpent            = errors.New("wrong time spent")
	ErrIssueNotFoundInTitle = errors.New("issue not in title")
	ErrBranchFormat         = errors.New("wrong branch format")
	ErrInconsistentIssueID  = errors.New("inconsistent issue ID between PR, title, git branch")
	ErrMerge                = errors.New("error during merge")
	ErrLabelNotFound        = errors.New("label not found in repository")
)

type Service interface {
	github.Client // This could be removed by defining the interface on the requesters side
	FindAndAssignReviewers(ctx context.Context, webhook github.RepoSenderGetter, pr *gogithub.PullRequest, fromTeam string) (bool, error)
	AddLabel(ctx context.Context, webhook github.RepoSenderGetter, prNumber int, label string) error
	RemoveLabel(ctx context.Context, webhook github.RepoSenderGetter, prNumber int, label string) error
	UpdateAndMergePR(ctx context.Context, webhook github.RepoSenderGetter, pr *gogithub.PullRequest) error
	AddCheckRun(ctx context.Context, webhook github.RepoSenderGetter, name string, headSHA string, success bool, output *gogithub.CheckRunOutput) error
	HasCheckRunSucceeded(ctx context.Context, webhook github.RepoSenderGetter, prNumber int, checkName string) (bool, error)
	AreAllCheckRunsDone(ctx context.Context, webhook github.RepoSenderGetter, prNumber int) (bool, error)
	HasEnoughApprovals(ctx context.Context, webhook github.RepoSenderGetter, pr *gogithub.PullRequest) (bool, error)
}

type service struct {
	github.Client
}

// NewService returns a new GitHub service
func NewService(githubClient github.Client) Service {
	return &service{
		Client: githubClient,
	}
}

func (s *service) labelByString(ctx context.Context, webhook github.RepoSenderGetter, label string) (*gogithub.Label, error) {
	ghLabels, _, err := s.ListLabels(ctx, webhook, nil)
	if err != nil {
		return nil, fmt.Errorf("error listing labels: %w", err)
	}

	if ghLabel := github.GetLabelInList(ghLabels, label); ghLabel != nil {
		return ghLabel, nil
	}

	return nil, fmt.Errorf("%w: no label found matching %q", ErrLabelNotFound, label)
}

// ListAllReviewers returns a map where keys are the GitHub logins of all people who have either left a review or have a review pending on the PR
func (s *service) ListAllReviewers(ctx context.Context, webhook github.RepoSenderGetter, prNumber int) (map[string]struct{}, error) {
	log := middlewares.LoggerFromGHContext(ctx, "github.ListAllReviewers")

	allReviewers := make(map[string]struct{})

	// List finished reviews
	err := github.ConsumePaginatedResource(github.MaxPerPage, func(opts *gogithub.ListOptions) (*gogithub.Response, bool, error) {
		reviews, res, err := s.ListReviews(ctx, webhook, prNumber, opts)
		if err != nil {
			return nil, false, fmt.Errorf("error listing reviews: %w", err)
		}

		for _, reviewer := range reviews {
			allReviewers[reviewer.GetUser().GetLogin()] = struct{}{}
		}

		return res, true, nil
	})
	if err != nil {
		return nil, fmt.Errorf("error listing paginated reviews: %w", err)
	}

	log.Infof("%+v added a review to this PR", allReviewers)

	// List pending reviews
	reviewers, _, err := s.ListReviewers(ctx, webhook, prNumber, nil)
	if err != nil {
		return nil, fmt.Errorf("error listing reviewers: %w", err)
	}

	for _, reviewer := range reviewers.Users {
		reviewerLogin := reviewer.GetLogin()
		log.Infof("PR has requested reviewer: %s", reviewerLogin)
		allReviewers[reviewerLogin] = struct{}{}
	}

	return allReviewers, nil
}

// requiredReviewCount returns the number of required approving reviews for the given branch.
// It first tries the classic branch protection API and falls back to repository rulesets on 404.
func (s *service) requiredReviewCount(ctx context.Context, webhook github.RepoSenderGetter, branch string) (int, error) {
	protection, _, err := s.GetBranchProtection(ctx, webhook, branch)
	if err != nil {
		if !errors.Is(err, gogithub.ErrBranchNotProtected) {
			return 0, fmt.Errorf("error getting branch protection: %w", err)
		}
		// Branch uses rulesets instead of classic branch protection
		return s.requiredReviewCountFromRuleset(ctx, webhook, branch)
	}
	if rpr := protection.GetRequiredPullRequestReviews(); rpr != nil {
		return rpr.RequiredApprovingReviewCount, nil
	}
	return 1, nil
}

// requiredReviewCountFromRuleset looks for a pull_request rule in the branch's active rulesets
// and returns its required approving review count. Defaults to 1 if no rule is found.
func (s *service) requiredReviewCountFromRuleset(ctx context.Context, webhook github.RepoSenderGetter, branch string) (int, error) {
	rules, _, err := s.GetRulesForBranch(ctx, webhook, branch)
	if err != nil {
		return 0, fmt.Errorf("error getting rules for branch: %w", err)
	}
	for _, rule := range rules {
		if rule.Type == "pull_request" && rule.Parameters != nil {
			var params gogithub.PullRequestRuleParameters
			if err := json.Unmarshal(*rule.Parameters, &params); err != nil {
				return 0, fmt.Errorf("error parsing pull_request rule parameters: %w", err)
			}
			return params.RequiredApprovingReviewCount, nil
		}
	}
	return 1, nil
}

// FindAndAssignReviewers attempts to assign reviewers to the given PR. It returns true if it succeeded
// All members of the given team are considered after being ranked by current review load.
// Succeeding in assigning reviewers means that we found and assigned at least enough reviewers to satisfy the main branch protection
func (s *service) FindAndAssignReviewers(ctx context.Context, webhook github.RepoSenderGetter, pr *gogithub.PullRequest, fromTeam string) (bool, error) {
	prNumber := pr.GetNumber()
	prOwner := pr.GetUser().GetLogin()

	log := middlewares.LoggerFromGHContext(ctx, "github.FindAndAssignReviewers")

	allReviewers, err := s.ListAllReviewers(ctx, webhook, pr.GetNumber())
	if err != nil {
		return false, fmt.Errorf("error listing current reviewers: %w", err)
	}

	log.Infof("PR has total reviewers: %v", allReviewers)

	teamMembers, _, err := s.ListTeamMembers(ctx, webhook, fromTeam, nil)
	if err != nil {
		return false, fmt.Errorf("error listing team members: %w", err)
	}

	log.Infof("found %d developers in team %s", len(teamMembers), fromTeam)

	// Prune reviewer list, considering only team members and ignoring PR opener
	consideredReviewers := make(map[string]struct{})
	teamMembersLogins := make([]string, len(teamMembers))
	for i, teamMember := range teamMembers {
		login := teamMember.GetLogin()
		if _, ok := allReviewers[login]; ok && login != prOwner {
			consideredReviewers[login] = struct{}{}
		}

		// Also take the opportunity to fill this slice that is needed below
		teamMembersLogins[i] = login
	}

	log.Infof("PR has reviewers in team %s: %v", fromTeam, consideredReviewers)

	requiredReviewers, err := s.requiredReviewCount(ctx, webhook, mainBranchName)
	if err != nil {
		return false, fmt.Errorf("error getting required reviewer count: %w", err)
	}
	if len(consideredReviewers) >= requiredReviewers {
		log.Infof("PR has enough reviewers: %d / %d", len(consideredReviewers), requiredReviewers)
		return true, nil
	}

	nbReviewersToFind := requiredReviewers - len(consideredReviewers)
	log.Infof("PR is ready to be reviewed but doesn't have enough reviewers: %d needs to request: %d reviewers", len(consideredReviewers), requiredReviewers)

	rankedDevs, err := s.RankUsersByReviewLoad(ctx, webhook, prNumber, teamMembersLogins)

	if err != nil {
		return false, fmt.Errorf("error ranking devs by review load: %w", err)
	}

	addedReviewers := make([]string, 0, nbReviewersToFind)
	for _, dev := range rankedDevs {
		if _, ok := consideredReviewers[dev]; !ok && dev != prOwner {
			addedReviewers = append(addedReviewers, dev)
		}

		if len(addedReviewers) >= nbReviewersToFind {
			break
		}
	}

	if len(addedReviewers) > 0 {
		if _, _, err = s.RequestReviewers(ctx, webhook, prNumber, addedReviewers); err != nil {
			return false, fmt.Errorf("error requesting reviewers: %w", err)
		}
		log.Infof("reviewers requested")
	}

	return len(addedReviewers) >= nbReviewersToFind, nil
}

// RankUsersByReviewLoad ranks all members of the given team by current review load.
// To do so, we calculate scores by looking at every open PR where a dev has either reviewed or is requested to review,
// and tally the number of additions in these PRs.
func (s *service) RankUsersByReviewLoad(ctx context.Context, webhook github.RepoSenderGetter, prNumber int, usersLogin []string) ([]string, error) {
	log := middlewares.LoggerFromGHContext(ctx, "github.RankUsersByReviewLoad")

	scores := make(map[string]int, len(usersLogin))
	for _, userLogin := range usersLogin {
		scores[userLogin] = 0
	}

	prs, _, err := s.ListPR(ctx, webhook, &gogithub.PullRequestListOptions{
		State:       "open",
		ListOptions: gogithub.ListOptions{PerPage: github.MaxPerPage},
	})
	if err != nil {
		return nil, fmt.Errorf("error listing pull requests: %w", err)
	}

	log.Infof("got %d opened PR's", len(prs))

	for _, pr := range prs {
		prNumber := pr.GetNumber()
		// Fetch extra information for the PR
		pr, _, err = s.PR(ctx, webhook, prNumber)
		if err != nil {
			return nil, fmt.Errorf("error getting PR: %w", err)
		}

		reviewers, err := s.ListAllReviewers(ctx, webhook, prNumber)
		if err != nil {
			return nil, fmt.Errorf("error listing reviewers: %w", err)
		}

		for reviewer := range reviewers {
			log.Infof("%s is reviewing PR #%d", reviewer, prNumber)
			if _, ok := scores[reviewer]; ok {
				scores[reviewer] += pr.GetAdditions()
			}
		}
	}

	// Arrange our dev and scores in a sortable struct
	type devScore struct {
		dev   string
		score int
	}
	devScores := make([]devScore, 0, len(scores))
	for dev, score := range scores {
		devScores = append(devScores, devScore{dev, score})
	}
	sort.Slice(devScores, func(i, j int) bool {
		return devScores[i].score < devScores[j].score
	})

	log.Infof("here's the score of our developers: %+v", devScores)

	rankedDevs := make([]string, len(devScores))
	for i := 0; i < len(devScores); i++ {
		rankedDevs[i] = devScores[i].dev
	}

	return rankedDevs, nil
}

// RemoveLabel attempts to match the label given with existing labels in the repository by case-insensitive prefix.
// If found, it removes the label from the PR
func (s *service) RemoveLabel(ctx context.Context, webhook github.RepoSenderGetter, prNumber int, label string) error {
	ghLabel, err := s.labelByString(ctx, webhook, label)
	if err != nil {
		return fmt.Errorf("error getting label %q: %w", label, err)
	}

	if _, err = s.RemovePRLabel(ctx, webhook, prNumber, ghLabel.GetName()); err != nil {
		var apiErr *gogithub.ErrorResponse
		if errors.As(err, &apiErr) && apiErr.Response.StatusCode == http.StatusNotFound { //nolint:revive // Check is done in order
			// Ignore not found errors, the label is already gone
			return nil
		}

		return fmt.Errorf("error removing label %q: %w", ghLabel.GetName(), err)
	}

	return nil
}

// AddLabel attempts to match the label given with existing labels in the repository by case-insensitive prefix.
// If found, it adds the label to the PR
func (s *service) AddLabel(ctx context.Context, webhook github.RepoSenderGetter, prNumber int, label string) error {
	ghLabel, err := s.labelByString(ctx, webhook, label)
	if err != nil {
		return fmt.Errorf("error getting label %q: %w", label, err)
	}

	if _, _, err = s.AddPRLabels(ctx, webhook, prNumber, []string{ghLabel.GetName()}); err != nil {
		var apiErr *gogithub.ErrorResponse
		if errors.As(err, &apiErr) && apiErr.Response.StatusCode == http.StatusUnprocessableEntity { //nolint:revive // Check is done in order
			// Ignore unprocessable entity errors, the label is already there
			return nil
		}

		return fmt.Errorf("error adding label %q: %w", ghLabel.GetName(), err)
	}

	return nil
}

// UpdateAndMergePR updates the title (or description if the title is too long) with the PR number and merges the PR
// It errors with ErrMerge if the GitHub merge request failed (most probably due to unsatisfied main branch protection constraints)
func (s *service) UpdateAndMergePR(ctx context.Context, webhook github.RepoSenderGetter, pr *gogithub.PullRequest) error {
	log := middlewares.LoggerFromGHContext(ctx, "github.UpdateAndMergePR")

	body := RemoveHTMLComments(pr.GetBody())
	prDescription, err := ExtractDescription(body)
	if err != nil {
		return fmt.Errorf("error parsing PR description: %w", err)
	}

	log.Infof("got this description: %q", prDescription)
	commitTitle, commitDescription := TitleOrDescriptionWithPRNumber(pr.GetTitle(), prDescription, pr.GetNumber())
	log.Infof("merging PR, title: %q description: %q", commitTitle, commitDescription)
	if _, _, err = s.MergePR(ctx, webhook, pr.GetNumber(), commitDescription, &gogithub.PullRequestOptions{
		CommitTitle: commitTitle,
		MergeMethod: "squash",
	}); err != nil {
		return fmt.Errorf("%w: %w", ErrMerge, err)
	}

	return nil
}

// AddCheckRun adds a check run to the given commit.
func (s *service) AddCheckRun(
	ctx context.Context,
	webhook github.RepoSenderGetter,
	name string,
	headSHA string,
	success bool,
	output *gogithub.CheckRunOutput,
) error {
	conclusion := github.CheckRunConclusionActionRequired
	if success {
		conclusion = github.CheckRunConclusionSuccess
	}

	_, _, err := s.CreateCheckRun(
		ctx,
		webhook,
		gogithub.CreateCheckRunOptions{
			Name:       name,
			HeadSHA:    headSHA,
			Status:     utils.Ptr(github.CheckRunStatusCompleted),
			Conclusion: &conclusion,
			Output:     output,
		},
	)

	return err
}

// HasCheckRunSucceeded returns true if the check run under the given checkName has succeeded on the HEAD commit of the given PR
func (s *service) HasCheckRunSucceeded(ctx context.Context, webhook github.RepoSenderGetter, prNumber int, checkName string) (bool, error) {
	log := middlewares.LoggerFromGHContext(ctx, "github.HasCheckRunSucceeded")

	checkRuns, _, err := s.ListCheckRunsForRef(
		ctx,
		webhook,
		fmt.Sprintf("pull/%d/head", prNumber),
		nil)
	if err != nil {
		return false, fmt.Errorf("error listing check runs: %w", err)
	}

	for _, cr := range checkRuns.CheckRuns {
		log.Infof("checkRun %q status: %q conclusion: %q", cr.GetName(), cr.GetStatus(), cr.GetConclusion())
		if cr.GetName() == checkName &&
			cr.GetStatus() == github.CheckRunStatusCompleted &&
			cr.GetConclusion() == github.CheckRunConclusionSuccess {
			return true, nil
		}
	}

	return false, nil
}

// AreAllCheckRunsDone returns true if all check runs on the HEAD commit of the given PR are done
func (s *service) AreAllCheckRunsDone(ctx context.Context, webhook github.RepoSenderGetter, prNumber int) (bool, error) {
	log := middlewares.LoggerFromGHContext(ctx, "github.AreAllCheckRunsDone")

	checkRuns, _, err := s.ListCheckRunsForRef(
		ctx,
		webhook,
		fmt.Sprintf("pull/%d/head", prNumber),
		nil)
	if err != nil {
		return false, fmt.Errorf("cannot list check runs: %w", err)
	}

	// Check if all check runs are in a completed state
	for _, checkRun := range checkRuns.CheckRuns {
		if checkRun.GetStatus() != github.CheckRunStatusCompleted {
			log.Infof("%s check is not completed yet: %s", checkRun.GetName(), checkRun.GetStatus())
			return false, nil
		}
	}

	return true, nil
}

// HasEnoughApprovals returns true if the given PR has sufficient approved reviews to satisfy the main branch protection constraint
func (s *service) HasEnoughApprovals(ctx context.Context, webhook github.RepoSenderGetter, pr *gogithub.PullRequest) (bool, error) {
	log := middlewares.LoggerFromGHContext(ctx, "github.HasEnoughApprovals")

	reviews := make([]*gogithub.PullRequestReview, 0, github.MaxPerPage)
	err := github.ConsumePaginatedResource(github.MaxPerPage, func(opts *gogithub.ListOptions) (*gogithub.Response, bool, error) {
		r, res, err := s.ListReviews(ctx, webhook, pr.GetNumber(), opts)
		if err != nil {
			return nil, false, fmt.Errorf("error listing reviews: %w", err)
		}

		reviews = append(reviews, r...)
		return res, true, nil
	})
	if err != nil {
		return false, fmt.Errorf("error listing all reviews: %w", err)
	}

	log.Infof("found %d reviews", len(reviews))

	// The list of reviews returns in chronological order.
	// Here we take the last review state given by every developer.
	devReview := make(map[string]struct{})
	for _, review := range reviews {
		reviewState := review.GetState()
		reviewerLogin := review.GetUser().GetLogin()
		log.Infof("review from %s in state: %s at %q", reviewerLogin, reviewState, review.GetSubmittedAt())
		if strings.EqualFold(reviewState, github.PullRequestReviewStateApproved) {
			devReview[reviewerLogin] = struct{}{}
		} else if strings.EqualFold(reviewState, github.PullRequestReviewStateChangesRequested) {
			delete(devReview, reviewerLogin)
		}
	}

	log.Infof("found %d approvals: %+v", len(devReview), devReview)

	mergeBranch := pr.GetBase().GetRef()
	requiredReviewers, err := s.requiredReviewCount(ctx, webhook, mergeBranch)
	if err != nil {
		return false, fmt.Errorf("error getting required reviewer count: %w", err)
	}
	log.Infof("%q branch requires %d reviews", mergeBranch, requiredReviewers)

	if len(devReview) < requiredReviewers {
		log.Info("not enough required ACK to add the approved label")
		return false, nil
	}

	return true, nil
}
