package github_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	gogithub "github.com/google/go-github/v63/github"
	"go.uber.org/mock/gomock"

	svcgithub "github.com/Flashgap/marvin/internal/service/github"
	mock_github "github.com/Flashgap/marvin/pkg/github/mock"
	"github.com/Flashgap/marvin/pkg/utils"
)

var _ = Describe("HasAIReviewed", func() {
	const (
		prNumber = 94
		headSHA  = "3323bd58" // merge commit of master into the branch
		oldSHA   = "2d0f2707" // an earlier commit CodeRabbit reviewed
	)
	aiLogins := []string{"coderabbitai[bot]"}

	var (
		mockCtrl   *gomock.Controller
		mockClient *mock_github.MockClient
		svc        svcgithub.Service
		pr         *gogithub.PullRequest
		event      *gogithub.PullRequestEvent
	)

	review := func(login, commitID string) *gogithub.PullRequestReview {
		return &gogithub.PullRequestReview{
			User:     &gogithub.User{Login: utils.Ptr(login)},
			CommitID: utils.Ptr(commitID),
		}
	}

	BeforeEach(func() {
		mockCtrl = gomock.NewController(GinkgoT())
		mockClient = mock_github.NewMockClient(mockCtrl)
		svc = svcgithub.NewService(mockClient)
		pr = &gogithub.PullRequest{
			Number: utils.Ptr(prNumber),
			Head:   &gogithub.PullRequestBranch{SHA: utils.Ptr(headSHA)},
		}
		event = &gogithub.PullRequestEvent{
			Repo:        &gogithub.Repository{Name: utils.Ptr("infra"), Owner: &gogithub.User{Login: utils.Ptr("hector-finance")}},
			PullRequest: pr,
		}
	})

	AfterEach(func() { mockCtrl.Finish() })

	When("an AI review exists on HEAD", func() {
		It("passes", func(ctx SpecContext) {
			mockClient.EXPECT().ListReviews(gomock.Any(), event, prNumber, gomock.Any()).
				Return([]*gogithub.PullRequestReview{review("coderabbitai[bot]", headSHA)}, nil, nil)

			ok, err := svc.HasAIReviewed(ctx, event, pr, aiLogins)
			Expect(err).NotTo(HaveOccurred())
			Expect(ok).To(BeTrue())
		})
	})

	When("an AI review exists but only on an earlier commit", func() {
		It("still passes (we only require a review to have happened once)", func(ctx SpecContext) {
			mockClient.EXPECT().ListReviews(gomock.Any(), event, prNumber, gomock.Any()).
				Return([]*gogithub.PullRequestReview{review("coderabbitai[bot]", oldSHA)}, nil, nil)

			ok, err := svc.HasAIReviewed(ctx, event, pr, aiLogins)
			Expect(err).NotTo(HaveOccurred())
			Expect(ok).To(BeTrue())
		})
	})

	When("only humans have reviewed", func() {
		It("blocks", func(ctx SpecContext) {
			mockClient.EXPECT().ListReviews(gomock.Any(), event, prNumber, gomock.Any()).
				Return([]*gogithub.PullRequestReview{review("some-human", oldSHA)}, nil, nil)

			ok, err := svc.HasAIReviewed(ctx, event, pr, aiLogins)
			Expect(err).NotTo(HaveOccurred())
			Expect(ok).To(BeFalse())
		})
	})

	When("there are no reviews at all", func() {
		It("blocks", func(ctx SpecContext) {
			mockClient.EXPECT().ListReviews(gomock.Any(), event, prNumber, gomock.Any()).
				Return([]*gogithub.PullRequestReview{}, nil, nil)

			ok, err := svc.HasAIReviewed(ctx, event, pr, aiLogins)
			Expect(err).NotTo(HaveOccurred())
			Expect(ok).To(BeFalse())
		})
	})
})
