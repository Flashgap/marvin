package github_test

import (
	"context"

	gogithub "github.com/google/go-github/v90/github"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"

	svcgithub "github.com/Flashgap/marvin/internal/service/github"
	pkggithub "github.com/Flashgap/marvin/pkg/github"
	mock_github "github.com/Flashgap/marvin/pkg/github/mock"
	"github.com/Flashgap/marvin/pkg/utils"
)

var _ = Describe("FindAndAssignReviewers", func() {
	const (
		prNumber       = 77
		defaultBranch  = "main"
		requiredReview = 3
	)

	var (
		mockCtrl   *gomock.Controller
		mockClient *mock_github.MockClient
		svc        svcgithub.Service
		pr         *gogithub.PullRequest
		event      *gogithub.PullRequestEvent
		openPRs    []pkggithub.OpenPRReviewLoad
	)

	member := func(login string) *gogithub.User {
		return &gogithub.User{Login: utils.Ptr(login)}
	}

	BeforeEach(func() {
		mockCtrl = gomock.NewController(GinkgoT())
		mockClient = mock_github.NewMockClient(mockCtrl)
		svc = svcgithub.NewService(mockClient)
		pr = &gogithub.PullRequest{
			Number: utils.Ptr(prNumber),
			User:   &gogithub.User{Login: utils.Ptr("dave")},
		}
		event = &gogithub.PullRequestEvent{
			Repo: &gogithub.Repository{
				Name:          utils.Ptr("infra"),
				Owner:         &gogithub.User{Login: utils.Ptr("hector-finance")},
				DefaultBranch: utils.Ptr(defaultBranch),
			},
			PullRequest: pr,
		}

		mockClient.EXPECT().ListReviews(gomock.Any(), event, prNumber, gomock.Any()).
			Return([]*gogithub.PullRequestReview{}, &gogithub.Response{}, nil)
		mockClient.EXPECT().ListReviewers(gomock.Any(), event, prNumber, gomock.Any()).
			Return(&gogithub.Reviewers{}, nil, nil)
		mockClient.EXPECT().GetBranchProtection(gomock.Any(), event, defaultBranch).
			Return(&gogithub.Protection{
				RequiredPullRequestReviews: &gogithub.PullRequestReviewsEnforcement{RequiredApprovingReviewCount: requiredReview},
			}, nil, nil)
		openPRs = []pkggithub.OpenPRReviewLoad{}
		mockClient.EXPECT().ListOpenPRsWithReviewers(gomock.Any(), event).
			DoAndReturn(func(context.Context, pkggithub.RepoSenderGetter) ([]pkggithub.OpenPRReviewLoad, error) {
				return openPRs, nil
			})
	})

	AfterEach(func() { mockCtrl.Finish() })

	When("multiple teams match a PR", func() {
		It("pools and dedupes members across every matched team before requesting reviewers", func(ctx SpecContext) {
			mockClient.EXPECT().ListTeamMembers(gomock.Any(), event, "backend-team", gomock.Any()).
				Return([]*gogithub.User{member("alice"), member("bob")}, nil, nil)
			mockClient.EXPECT().ListTeamMembers(gomock.Any(), event, "data-team", gomock.Any()).
				Return([]*gogithub.User{member("bob"), member("carol")}, nil, nil)

			mockClient.EXPECT().RequestReviewers(gomock.Any(), event, prNumber, gomock.Any()).
				DoAndReturn(func(_ context.Context, _ pkggithub.RepoSenderGetter, _ int, reviewers []string) (*gogithub.PullRequest, *gogithub.Response, error) {
					Expect(reviewers).To(ConsistOf("alice", "bob", "carol"))
					return pr, nil, nil
				})

			ok, err := svc.FindAndAssignReviewers(ctx, event, pr, []string{"backend-team", "data-team"})
			Expect(err).NotTo(HaveOccurred())
			Expect(ok).To(BeTrue())
		})
	})

	When("the PR is stacked on top of another open PR", func() {
		parentReviewers := func(logins ...string) map[string]struct{} {
			set := make(map[string]struct{}, len(logins))
			for _, login := range logins {
				set[login] = struct{}{}
			}
			return set
		}

		BeforeEach(func() {
			pr.Base = &gogithub.PullRequestBranch{Ref: utils.Ptr("alice/part-1")}
			mockClient.EXPECT().ListTeamMembers(gomock.Any(), event, "backend-team", gomock.Any()).
				Return([]*gogithub.User{member("alice"), member("bob"), member("carol"), member("dave"), member("frank")}, nil, nil)
		})

		It("requests the reviewers of the parent PR first, then fills the rest by review load", func(ctx SpecContext) {
			openPRs = []pkggithub.OpenPRReviewLoad{
				{Number: 70, HeadRef: "alice/part-1", Additions: 10, Reviewers: parentReviewers("bob", "erin")},
				{Number: 71, HeadRef: "unrelated", Additions: 500, Reviewers: parentReviewers("carol", "dave", "frank")},
			}

			mockClient.EXPECT().RequestReviewers(gomock.Any(), event, prNumber, gomock.Any()).
				DoAndReturn(func(_ context.Context, _ pkggithub.RepoSenderGetter, _ int, reviewers []string) (*gogithub.PullRequest, *gogithub.Response, error) {
					// bob and erin inherited (erin is not in the team), alice fills the last slot: lightest load
					Expect(reviewers).To(ConsistOf("bob", "erin", "alice"))
					return pr, nil, nil
				})

			ok, err := svc.FindAndAssignReviewers(ctx, event, pr, []string{"backend-team"})
			Expect(err).NotTo(HaveOccurred())
			Expect(ok).To(BeTrue())
		})

		It("never requests the PR author, even if they review the parent PR", func(ctx SpecContext) {
			openPRs = []pkggithub.OpenPRReviewLoad{
				{Number: 70, HeadRef: "alice/part-1", Additions: 1000, Reviewers: parentReviewers("dave", "bob")},
				{Number: 71, HeadRef: "unrelated", Additions: 500, Reviewers: parentReviewers("carol")},
				{Number: 72, HeadRef: "unrelated-2", Additions: 900, Reviewers: parentReviewers("frank")},
			}

			mockClient.EXPECT().RequestReviewers(gomock.Any(), event, prNumber, gomock.Any()).
				DoAndReturn(func(_ context.Context, _ pkggithub.RepoSenderGetter, _ int, reviewers []string) (*gogithub.PullRequest, *gogithub.Response, error) {
					Expect(reviewers).To(ConsistOf("bob", "alice", "carol"))
					return pr, nil, nil
				})

			ok, err := svc.FindAndAssignReviewers(ctx, event, pr, []string{"backend-team"})
			Expect(err).NotTo(HaveOccurred())
			Expect(ok).To(BeTrue())
		})

		It("behaves as usual when no open PR matches the base branch", func(ctx SpecContext) {
			openPRs = []pkggithub.OpenPRReviewLoad{
				{Number: 71, HeadRef: "unrelated", Additions: 500, Reviewers: parentReviewers("frank")},
			}

			mockClient.EXPECT().RequestReviewers(gomock.Any(), event, prNumber, gomock.Any()).
				DoAndReturn(func(_ context.Context, _ pkggithub.RepoSenderGetter, _ int, reviewers []string) (*gogithub.PullRequest, *gogithub.Response, error) {
					Expect(reviewers).To(ConsistOf("alice", "bob", "carol"))
					return pr, nil, nil
				})

			ok, err := svc.FindAndAssignReviewers(ctx, event, pr, []string{"backend-team"})
			Expect(err).NotTo(HaveOccurred())
			Expect(ok).To(BeTrue())
		})
	})
})
