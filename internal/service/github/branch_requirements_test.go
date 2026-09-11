package github_test

import (
	"errors"

	gogithub "github.com/google/go-github/v90/github"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"

	svcgithub "github.com/Flashgap/marvin/internal/service/github"
	mock_github "github.com/Flashgap/marvin/pkg/github/mock"
	"github.com/Flashgap/marvin/pkg/utils"
)

var _ = Describe("BranchRequirements", func() {
	const branch = "master"

	var (
		mockCtrl   *gomock.Controller
		mockClient *mock_github.MockClient
		svc        svcgithub.Service
		event      *gogithub.PullRequestEvent
	)

	BeforeEach(func() {
		mockCtrl = gomock.NewController(GinkgoT())
		mockClient = mock_github.NewMockClient(mockCtrl)
		svc = svcgithub.NewService(mockClient)
		event = &gogithub.PullRequestEvent{
			Repo: &gogithub.Repository{
				Name:  utils.Ptr("infra"),
				Owner: &gogithub.User{Login: utils.Ptr("hector-finance")},
			},
		}
	})

	AfterEach(func() { mockCtrl.Finish() })

	When("the branch uses classic branch protection", func() {
		It("renders every blocking rule", func(ctx SpecContext) {
			mockClient.EXPECT().GetBranchProtection(gomock.Any(), event, branch).Return(&gogithub.Protection{
				RequiredConversationResolution: &gogithub.RequiredConversationResolution{Enabled: true},
				RequiredPullRequestReviews: &gogithub.PullRequestReviewsEnforcement{
					RequiredApprovingReviewCount: 2,
					RequireCodeOwnerReviews:      true,
					RequireLastPushApproval:      true,
				},
				RequiredStatusChecks: &gogithub.RequiredStatusChecks{
					Contexts: &[]string{"build", "lint"},
				},
				RequiredSignatures:   &gogithub.SignaturesProtectedBranch{Enabled: utils.Ptr(true)},
				RequireLinearHistory: &gogithub.RequireLinearHistory{Enabled: true},
			}, nil, nil)

			requirements, err := svc.BranchRequirements(ctx, event, branch)
			Expect(err).NotTo(HaveOccurred())
			Expect(requirements).To(Equal([]string{
				"conversation resolution on all review threads",
				"2 approving reviews",
				"a review from a code owner",
				"an approval from someone other than the last pusher",
				"the status check `build` to pass",
				"the status check `lint` to pass",
				"signed commits",
				"a linear history (no merge commits)",
			}))
		})

		It("renders a single approval in the singular", func(ctx SpecContext) {
			mockClient.EXPECT().GetBranchProtection(gomock.Any(), event, branch).Return(&gogithub.Protection{
				RequiredPullRequestReviews: &gogithub.PullRequestReviewsEnforcement{
					RequiredApprovingReviewCount: 1,
				},
			}, nil, nil)

			requirements, err := svc.BranchRequirements(ctx, event, branch)
			Expect(err).NotTo(HaveOccurred())
			Expect(requirements).To(Equal([]string{"1 approving review"}))
		})
	})

	When("the branch uses rulesets", func() {
		BeforeEach(func() {
			mockClient.EXPECT().GetBranchProtection(gomock.Any(), event, branch).
				Return(nil, nil, gogithub.ErrBranchNotProtected)
		})

		It("renders every blocking rule", func(ctx SpecContext) {
			mockClient.EXPECT().GetRulesForBranch(gomock.Any(), event, branch).Return(&gogithub.BranchRules{
				PullRequest: []*gogithub.PullRequestBranchRule{
					{
						Parameters: gogithub.PullRequestRuleParameters{
							RequiredReviewThreadResolution: true,
							RequiredApprovingReviewCount:   1,
							RequireCodeOwnerReview:         true,
						},
					},
				},
				RequiredStatusChecks: []*gogithub.RequiredStatusChecksBranchRule{
					{
						Parameters: gogithub.RequiredStatusChecksRuleParameters{
							RequiredStatusChecks: []*gogithub.RuleStatusCheck{{Context: "build"}},
						},
					},
				},
				RequiredDeployments: []*gogithub.RequiredDeploymentsBranchRule{
					{
						Parameters: gogithub.RequiredDeploymentsRuleParameters{
							RequiredDeploymentEnvironments: []string{"staging"},
						},
					},
				},
			}, nil, nil)

			requirements, err := svc.BranchRequirements(ctx, event, branch)
			Expect(err).NotTo(HaveOccurred())
			Expect(requirements).To(Equal([]string{
				"conversation resolution on all review threads",
				"1 approving review",
				"a review from a code owner",
				"the status check `build` to pass",
				"a successful deployment to the `staging` environment",
			}))
		})

		It("returns an empty slice when no ruleset applies", func(ctx SpecContext) {
			mockClient.EXPECT().GetRulesForBranch(gomock.Any(), event, branch).
				Return(&gogithub.BranchRules{}, nil, nil)

			requirements, err := svc.BranchRequirements(ctx, event, branch)
			Expect(err).NotTo(HaveOccurred())
			Expect(requirements).To(BeEmpty())
		})
	})

	When("the protection lookup fails for another reason", func() {
		It("returns the error", func(ctx SpecContext) {
			mockClient.EXPECT().GetBranchProtection(gomock.Any(), event, branch).
				Return(nil, nil, errors.New("boom"))

			_, err := svc.BranchRequirements(ctx, event, branch)
			Expect(err).To(MatchError(ContainSubstring("boom")))
		})
	})
})
