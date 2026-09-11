package github

import (
	"context"
	"errors"
	"fmt"

	gogithub "github.com/google/go-github/v90/github"

	"github.com/Flashgap/marvin/pkg/github"
)

const (
	requirementThreadResolution = "conversation resolution on all review threads"
	requirementCodeOwnerReview  = "a review from a code owner"
	requirementLastPushApproval = "an approval from someone other than the last pusher"
	requirementSignedCommits    = "signed commits"
	requirementLinearHistory    = "a linear history (no merge commits)"
)

// BranchRequirements returns human-readable descriptions of the rules that can block a merge on the
// given branch, so Marvin can tell a developer what to look at when GitHub refuses to merge.
//
// It follows the same lookup order as requiredReviewCount: classic branch protection first,
// repository rulesets when the branch is not protected the classic way. An empty slice means there
// was nothing worth reporting; callers are expected to fall back to a generic message rather than
// print an empty list.
func (s *service) BranchRequirements(ctx context.Context, webhook github.RepoSenderGetter, branch string) ([]string, error) {
	protection, _, err := s.GetBranchProtection(ctx, webhook, branch)
	if err != nil {
		if !errors.Is(err, gogithub.ErrBranchNotProtected) {
			return nil, fmt.Errorf("error getting branch protection: %w", err)
		}
		// Branch uses rulesets instead of classic branch protection
		return s.branchRequirementsFromRuleset(ctx, webhook, branch)
	}

	return protectionRequirements(protection), nil
}

// branchRequirementsFromRuleset renders the rules of every ruleset active on the branch.
func (s *service) branchRequirementsFromRuleset(ctx context.Context, webhook github.RepoSenderGetter, branch string) ([]string, error) {
	rules, _, err := s.GetRulesForBranch(ctx, webhook, branch)
	if err != nil {
		return nil, fmt.Errorf("error getting rules for branch: %w", err)
	}

	requirements := make([]string, 0, 8)

	for _, rule := range rules.PullRequest {
		params := rule.Parameters
		if params.RequiredReviewThreadResolution {
			requirements = append(requirements, requirementThreadResolution)
		}
		requirements = append(requirements, reviewRequirements(
			params.RequiredApprovingReviewCount,
			params.RequireCodeOwnerReview,
			params.RequireLastPushApproval,
		)...)
	}

	for _, rule := range rules.RequiredStatusChecks {
		for _, check := range rule.Parameters.RequiredStatusChecks {
			requirements = append(requirements, statusCheckRequirement(check.Context))
		}
	}

	for _, rule := range rules.RequiredDeployments {
		for _, env := range rule.Parameters.RequiredDeploymentEnvironments {
			requirements = append(requirements, fmt.Sprintf("a successful deployment to the `%s` environment", env))
		}
	}

	if len(rules.RequiredSignatures) > 0 {
		requirements = append(requirements, requirementSignedCommits)
	}

	if len(rules.RequiredLinearHistory) > 0 {
		requirements = append(requirements, requirementLinearHistory)
	}

	return requirements, nil
}

// protectionRequirements renders the rules of a classic branch protection.
func protectionRequirements(protection *gogithub.Protection) []string {
	requirements := make([]string, 0, 8)

	if protection.GetRequiredConversationResolution().GetEnabled() {
		requirements = append(requirements, requirementThreadResolution)
	}

	if reviews := protection.GetRequiredPullRequestReviews(); reviews != nil {
		requirements = append(requirements, reviewRequirements(
			reviews.RequiredApprovingReviewCount,
			reviews.RequireCodeOwnerReviews,
			reviews.RequireLastPushApproval,
		)...)
	}

	if checks := protection.GetRequiredStatusChecks(); checks != nil {
		// GitHub populates either Contexts or Checks, never both.
		if checks.Contexts != nil {
			for _, statusContext := range *checks.Contexts {
				requirements = append(requirements, statusCheckRequirement(statusContext))
			}
		}
		if checks.Checks != nil {
			for _, check := range *checks.Checks {
				requirements = append(requirements, statusCheckRequirement(check.Context))
			}
		}
	}

	if protection.GetRequiredSignatures().GetEnabled() {
		requirements = append(requirements, requirementSignedCommits)
	}

	if protection.GetRequireLinearHistory().GetEnabled() {
		requirements = append(requirements, requirementLinearHistory)
	}

	return requirements
}

// reviewRequirements renders the review constraints shared by classic branch protections and
// rulesets, which express them with the same semantics under different field names.
func reviewRequirements(approvals int, codeOwnerReview, lastPushApproval bool) []string {
	requirements := make([]string, 0, 3)

	switch {
	case approvals == 1:
		requirements = append(requirements, "1 approving review")
	case approvals > 1:
		requirements = append(requirements, fmt.Sprintf("%d approving reviews", approvals))
	}

	if codeOwnerReview {
		requirements = append(requirements, requirementCodeOwnerReview)
	}

	if lastPushApproval {
		requirements = append(requirements, requirementLastPushApproval)
	}

	return requirements
}

func statusCheckRequirement(statusContext string) string {
	return fmt.Sprintf("the status check `%s` to pass", statusContext)
}
