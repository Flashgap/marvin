package github_test

import (
	"context"
	"errors"

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
		prNumber      = 77
		defaultBranch = "main"
	)

	var (
		mockCtrl         *gomock.Controller
		mockClient       *mock_github.MockClient
		svc              svcgithub.Service
		pr               *gogithub.PullRequest
		event            *gogithub.PullRequestEvent
		requiredReview   int
		currentReviewers []*gogithub.User
		openPRs          []pkggithub.OpenPRReviewLoad
		rankingCall      *gomock.Call
	)

	member := func(login string) *gogithub.User {
		return &gogithub.User{Login: utils.Ptr(login)}
	}

	BeforeEach(func() {
		requiredReview = 3
		currentReviewers = nil
		openPRs = []pkggithub.OpenPRReviewLoad{}

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
			DoAndReturn(func(context.Context, pkggithub.RepoSenderGetter, int, *gogithub.ListOptions) (*gogithub.Reviewers, *gogithub.Response, error) {
				return &gogithub.Reviewers{Users: currentReviewers}, nil, nil
			})
		mockClient.EXPECT().GetBranchProtection(gomock.Any(), event, defaultBranch).
			DoAndReturn(func(context.Context, pkggithub.RepoSenderGetter, string) (*gogithub.Protection, *gogithub.Response, error) {
				return &gogithub.Protection{
					RequiredPullRequestReviews: &gogithub.PullRequestReviewsEnforcement{RequiredApprovingReviewCount: requiredReview},
				}, nil, nil
			})
		rankingCall = mockClient.EXPECT().ListOpenPRsWithReviewers(gomock.Any(), event).
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

			ok, err := svc.FindAndAssignReviewers(ctx, event, pr, []string{"backend-team", "data-team"}, false)
			Expect(err).NotTo(HaveOccurred())
			Expect(ok).To(BeTrue())
		})
	})

	When("the PR is stacked", func() {
		// Review load of each team member: the lower, the sooner they get picked by load
		loads := map[string]int{"alice": 0, "bob": 10, "carol": 20, "maxime": 30, "clem": 1000}

		requestedReviewers := func() *[]string {
			var requested []string
			mockClient.EXPECT().RequestReviewers(gomock.Any(), event, prNumber, gomock.Any()).
				DoAndReturn(func(_ context.Context, _ pkggithub.RepoSenderGetter, _ int, reviewers []string) (*gogithub.PullRequest, *gogithub.Response, error) {
					requested = reviewers
					return pr, nil, nil
				})
			return &requested
		}

		stackLayers := func(layers ...pkggithub.StackLayer) {
			mockClient.EXPECT().ListStackLayers(gomock.Any(), event, prNumber).Return(layers, nil)
		}

		BeforeEach(func() {
			requiredReview = 1
			pr.Stack = &gogithub.PullRequestStack{Number: utils.Ptr(15), Position: utils.Ptr(3), Size: utils.Ptr(3)}
			for login, load := range loads {
				openPRs = append(openPRs, pkggithub.OpenPRReviewLoad{Number: 1, Additions: load, Reviewers: map[string]struct{}{login: {}}})
			}
			mockClient.EXPECT().ListTeamMembers(gomock.Any(), event, "backend-team", gomock.Any()).
				Return([]*gogithub.User{member("alice"), member("bob"), member("carol"), member("maxime"), member("clem"), member("dave")}, nil, nil)
		})

		It("takes the reviewer of the nearest layer below, whatever their load, without ranking anybody", func(ctx SpecContext) {
			stackLayers(
				pkggithub.StackLayer{Number: 75, Position: 1, Reviewers: []string{"maxime"}},
				pkggithub.StackLayer{Number: 76, Position: 2, Reviewers: []string{"clem"}},
				pkggithub.StackLayer{Number: prNumber, Position: 3},
			)
			rankingCall.Times(0)
			requested := requestedReviewers()

			ok, err := svc.FindAndAssignReviewers(ctx, event, pr, []string{"backend-team"}, true)
			Expect(err).NotTo(HaveOccurred())
			Expect(ok).To(BeTrue())
			Expect(*requested).To(Equal([]string{"clem"}))
		})

		It("ignores AI bots, people outside the teams and the PR author on a layer", func(ctx SpecContext) {
			stackLayers(
				pkggithub.StackLayer{Number: 75, Position: 1, Reviewers: []string{"maxime"}},
				pkggithub.StackLayer{Number: 76, Position: 2, Reviewers: []string{"coderabbitai[bot]", "outsider", "dave"}},
				pkggithub.StackLayer{Number: prNumber, Position: 3},
			)
			rankingCall.Times(0)
			requested := requestedReviewers()

			ok, err := svc.FindAndAssignReviewers(ctx, event, pr, []string{"backend-team"}, true)
			Expect(err).NotTo(HaveOccurred())
			Expect(ok).To(BeTrue())
			Expect(*requested).To(Equal([]string{"maxime"}))
		})

		It("does not count a stack reviewer already reviewing the PR twice", func(ctx SpecContext) {
			requiredReview = 2
			currentReviewers = []*gogithub.User{member("clem")}
			stackLayers(
				pkggithub.StackLayer{Number: 75, Position: 1, Reviewers: []string{"maxime"}},
				pkggithub.StackLayer{Number: 76, Position: 2, Reviewers: []string{"clem"}},
				pkggithub.StackLayer{Number: prNumber, Position: 3},
			)
			rankingCall.Times(0)
			requested := requestedReviewers()

			ok, err := svc.FindAndAssignReviewers(ctx, event, pr, []string{"backend-team"}, true)
			Expect(err).NotTo(HaveOccurred())
			Expect(ok).To(BeTrue())
			Expect(*requested).To(Equal([]string{"maxime"}))
		})

		It("picks the least loaded stack reviewers when the layer has more than needed", func(ctx SpecContext) {
			stackLayers(
				pkggithub.StackLayer{Number: 76, Position: 2, Reviewers: []string{"clem", "maxime"}},
				pkggithub.StackLayer{Number: prNumber, Position: 3},
			)
			requested := requestedReviewers()

			ok, err := svc.FindAndAssignReviewers(ctx, event, pr, []string{"backend-team"}, true)
			Expect(err).NotTo(HaveOccurred())
			Expect(ok).To(BeTrue())
			Expect(*requested).To(Equal([]string{"maxime"}))
		})

		It("fills the slots stack reviewers leave open by review load", func(ctx SpecContext) {
			requiredReview = 3
			stackLayers(
				pkggithub.StackLayer{Number: 76, Position: 2, Reviewers: []string{"clem"}},
				pkggithub.StackLayer{Number: prNumber, Position: 3},
			)
			requested := requestedReviewers()

			ok, err := svc.FindAndAssignReviewers(ctx, event, pr, []string{"backend-team"}, true)
			Expect(err).NotTo(HaveOccurred())
			Expect(ok).To(BeTrue())
			Expect(*requested).To(Equal([]string{"clem", "alice", "bob"}))
		})

		It("falls back to review load when the stack cannot be read", func(ctx SpecContext) {
			mockClient.EXPECT().ListStackLayers(gomock.Any(), event, prNumber).Return(nil, errors.New("boom"))
			requested := requestedReviewers()

			ok, err := svc.FindAndAssignReviewers(ctx, event, pr, []string{"backend-team"}, true)
			Expect(err).NotTo(HaveOccurred())
			Expect(ok).To(BeTrue())
			Expect(*requested).To(Equal([]string{"alice"}))
		})

		It("ignores the stack when the feature is disabled", func(ctx SpecContext) {
			// No ListStackLayers expectation: the strict mock fails the spec if the stack is read
			requested := requestedReviewers()

			ok, err := svc.FindAndAssignReviewers(ctx, event, pr, []string{"backend-team"}, false)
			Expect(err).NotTo(HaveOccurred())
			Expect(ok).To(BeTrue())
			Expect(*requested).To(Equal([]string{"alice"}))
		})
	})

	It("does not read any stack for a PR outside of one", func(ctx SpecContext) {
		// No ListStackLayers expectation: the strict mock fails the spec if the stack is read
		requiredReview = 1
		openPRs = []pkggithub.OpenPRReviewLoad{{Number: 1, Additions: 10, Reviewers: map[string]struct{}{"bob": {}}}}
		mockClient.EXPECT().ListTeamMembers(gomock.Any(), event, "backend-team", gomock.Any()).
			Return([]*gogithub.User{member("alice"), member("bob")}, nil, nil)
		mockClient.EXPECT().RequestReviewers(gomock.Any(), event, prNumber, []string{"alice"}).Return(pr, nil, nil)

		ok, err := svc.FindAndAssignReviewers(ctx, event, pr, []string{"backend-team"}, true)
		Expect(err).NotTo(HaveOccurred())
		Expect(ok).To(BeTrue())
	})
})
