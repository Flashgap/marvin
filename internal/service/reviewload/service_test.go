package reviewload_test

import (
	"errors"
	"testing"

	gogithub "github.com/google/go-github/v90/github"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/slack-go/slack"
	"go.uber.org/mock/gomock"

	"github.com/Flashgap/marvin/internal/service/reviewload"
	pkggithub "github.com/Flashgap/marvin/pkg/github"
	mock_github "github.com/Flashgap/marvin/pkg/github/mock"
)

func Test(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Review load service test suite")
}

var _ = Describe("ReviewLoad", func() {
	var (
		mockClient *mock_github.MockClient
		svc        reviewload.Service
		backend    = &gogithub.Repository{
			Name:     gogithub.Ptr("backend"),
			FullName: gogithub.Ptr("hector-finance/backend"),
			HTMLURL:  gogithub.Ptr("https://github.com/hector-finance/backend"),
			Owner:    &gogithub.User{Login: gogithub.Ptr("hector-finance")},
		}
		marvinRepo = &gogithub.Repository{Name: gogithub.Ptr("marvin"), FullName: gogithub.Ptr("hector-finance/marvin")}
	)

	BeforeEach(func() {
		mockClient = mock_github.NewMockClient(gomock.NewController(GinkgoT()))
		svc = reviewload.NewService(mockClient)
		mockClient.EXPECT().ListInstalledRepos(gomock.Any(), gomock.Any()).
			Return(&gogithub.ListRepositories{Repositories: []*gogithub.Repository{marvinRepo, backend}}, &gogithub.Response{}, nil)
	})

	It("ranks reviewers lowest score first with links to their PRs", func(ctx SpecContext) {
		mockClient.EXPECT().ListOpenPRsWithReviewers(gomock.Any(), gomock.Any()).
			DoAndReturn(func(_ any, webhook pkggithub.RepoSenderGetter) ([]pkggithub.OpenPRReviewLoad, error) {
				Expect(webhook.GetRepo()).To(Equal(backend))
				return []pkggithub.OpenPRReviewLoad{
					{Number: 373, Additions: 153, Reviewers: map[string]struct{}{"lebascou": {}, "leoregino": {}}},
					{Number: 371, Additions: 1, Reviewers: map[string]struct{}{"lebascou": {}}},
					{Number: 320, Additions: 90, Reviewers: map[string]struct{}{"0rax": {}}},
					{Number: 286, Additions: 500, Reviewers: map[string]struct{}{}},
				}, nil
			})

		msg, err := svc.ReviewLoad(ctx, slack.SlashCommand{Text: " Backend "})
		Expect(err).NotTo(HaveOccurred())
		Expect(msg.ResponseType).To(Equal(slack.ResponseTypeEphemeral))
		Expect(msg.Text).To(Equal("*Review load of hector-finance/backend* (additions of the open PRs each person reviews, lowest is picked first)\n" +
			"• *0rax* — 90 — <https://github.com/hector-finance/backend/pull/320|#320>\n" +
			"• *leoregino* — 153 — <https://github.com/hector-finance/backend/pull/373|#373>\n" +
			"• *lebascou* — 154 — <https://github.com/hector-finance/backend/pull/371|#371>, <https://github.com/hector-finance/backend/pull/373|#373>\n"))
	})

	It("matches the repository's full name", func(ctx SpecContext) {
		mockClient.EXPECT().ListOpenPRsWithReviewers(gomock.Any(), gomock.Any()).Return(nil, nil)

		msg, err := svc.ReviewLoad(ctx, slack.SlashCommand{Text: "hector-finance/backend"})
		Expect(err).NotTo(HaveOccurred())
		Expect(msg.Text).To(Equal("Nobody is reviewing an open PR of *hector-finance/backend*."))
	})

	It("shows the usage with the installed repositories when no repository is given", func(ctx SpecContext) {
		msg, err := svc.ReviewLoad(ctx, slack.SlashCommand{})
		Expect(err).NotTo(HaveOccurred())
		Expect(msg.Text).To(Equal("Usage: `/review-load <repository>`. Repositories: `backend`, `marvin`"))
	})

	It("rejects an unknown repository", func(ctx SpecContext) {
		msg, err := svc.ReviewLoad(ctx, slack.SlashCommand{Text: "frontend"})
		Expect(err).NotTo(HaveOccurred())
		Expect(msg.Text).To(Equal("Unknown repository `frontend`. Repositories: `backend`, `marvin`"))
	})

	It("surfaces GitHub errors", func(ctx SpecContext) {
		mockClient.EXPECT().ListOpenPRsWithReviewers(gomock.Any(), gomock.Any()).Return(nil, errors.New("boom"))

		_, err := svc.ReviewLoad(ctx, slack.SlashCommand{Text: "backend"})
		Expect(err).To(MatchError(ContainSubstring("boom")))
	})
})

var _ = Describe("Rank", func() {
	var (
		mockClient *mock_github.MockClient
		svc        reviewload.Service
		webhook    = &gogithub.PullRequestEvent{Repo: &gogithub.Repository{Name: gogithub.Ptr("backend")}}
	)

	BeforeEach(func() {
		mockClient = mock_github.NewMockClient(gomock.NewController(GinkgoT()))
		svc = reviewload.NewService(mockClient)
		mockClient.EXPECT().ListOpenPRsWithReviewers(gomock.Any(), webhook).Return([]pkggithub.OpenPRReviewLoad{
			{Number: 2, Additions: 10, Reviewers: map[string]struct{}{"bob": {}, "outsider": {}}},
			{Number: 1, Additions: 5, Reviewers: map[string]struct{}{"bob": {}}},
		}, nil)
	})

	It("ranks everyone reviewing an open PR without members", func(ctx SpecContext) {
		Expect(svc.Rank(ctx, webhook, nil)).To(Equal([]reviewload.Reviewer{
			{Login: "outsider", Score: 10, PRs: []int{2}},
			{Login: "bob", Score: 15, PRs: []int{1, 2}},
		}))
	})

	It("ranks only members, including those reviewing nothing", func(ctx SpecContext) {
		Expect(svc.Rank(ctx, webhook, []string{"bob", "carol", "alice"})).To(Equal([]reviewload.Reviewer{
			{Login: "alice"},
			{Login: "carol"},
			{Login: "bob", Score: 15, PRs: []int{1, 2}},
		}))
	})
})
