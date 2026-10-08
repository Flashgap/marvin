package reviewload_test

import (
	"errors"
	"testing"

	gogithub "github.com/google/go-github/v90/github"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"

	"github.com/Flashgap/marvin/internal/service/reviewload"
	pkggithub "github.com/Flashgap/marvin/pkg/github"
	mock_github "github.com/Flashgap/marvin/pkg/github/mock"
)

func Test(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Review load service test suite")
}

var _ = Describe("RepoReviewLoad", func() {
	var (
		mockClient *mock_github.MockClient
		svc        reviewload.Service
		backend    = &gogithub.Repository{
			Name:     gogithub.Ptr("backend"),
			FullName: gogithub.Ptr("hector-finance/backend"),
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

	DescribeTable("ranks everyone reviewing an open PR of the matching repository",
		func(ctx SpecContext, name string) {
			mockClient.EXPECT().ListOpenPRsWithReviewers(gomock.Any(), gomock.Any()).
				DoAndReturn(func(_ any, webhook pkggithub.RepoSenderGetter) ([]pkggithub.OpenPRReviewLoad, error) {
					Expect(webhook.GetRepo()).To(Equal(backend))
					return []pkggithub.OpenPRReviewLoad{
						{Number: 373, Additions: 153, Reviewers: map[string]struct{}{"lebascou": {}, "leoregino": {}}},
						{Number: 371, Additions: 1, Reviewers: map[string]struct{}{"lebascou": {}}},
						{Number: 286, Additions: 500, Reviewers: map[string]struct{}{}},
					}, nil
				})

			repo, reviewers, err := svc.RepoReviewLoad(ctx, name)
			Expect(err).NotTo(HaveOccurred())
			Expect(repo).To(Equal(backend))
			Expect(reviewers).To(Equal([]reviewload.Reviewer{
				{Login: "leoregino", Score: 153, PRs: []int{373}},
				{Login: "lebascou", Score: 154, PRs: []int{371, 373}},
			}))
		},
		Entry("by name, ignoring case", "Backend"),
		Entry("by full name", "hector-finance/backend"),
	)

	DescribeTable("returns the installed repositories when none matches",
		func(ctx SpecContext, name string) {
			_, _, err := svc.RepoReviewLoad(ctx, name)

			var unknownRepo *reviewload.UnknownRepositoryError
			Expect(errors.As(err, &unknownRepo)).To(BeTrue())
			Expect(unknownRepo).To(Equal(&reviewload.UnknownRepositoryError{Name: name, Repositories: []string{"backend", "marvin"}}))
		},
		Entry("no name", ""),
		Entry("unknown name", "frontend"),
	)

	It("surfaces GitHub errors", func(ctx SpecContext) {
		mockClient.EXPECT().ListOpenPRsWithReviewers(gomock.Any(), gomock.Any()).Return(nil, errors.New("boom"))

		_, _, err := svc.RepoReviewLoad(ctx, "backend")
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
