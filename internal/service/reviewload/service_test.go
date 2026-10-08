package reviewload_test

import (
	"errors"
	"fmt"
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

func pr(repo string, number int) reviewload.PullRequest {
	return reviewload.PullRequest{Number: number, URL: fmt.Sprintf("https://github.com/hector-finance/%s/pull/%d", repo, number)}
}

func repository(name string) *gogithub.Repository {
	return &gogithub.Repository{
		Name:     gogithub.Ptr(name),
		FullName: gogithub.Ptr("hector-finance/" + name),
		HTMLURL:  gogithub.Ptr("https://github.com/hector-finance/" + name),
		Owner:    &gogithub.User{Login: gogithub.Ptr("hector-finance")},
	}
}

var _ = Describe("RepoReviewLoad", func() {
	var (
		mockClient *mock_github.MockClient
		svc        reviewload.Service
		backend    = repository("backend")
		marvinRepo = repository("marvin")
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
						{Number: 373, Additions: 153, Reviewers: map[string]struct{}{"Jane": {}, "leoregino": {}}},
						{Number: 371, Additions: 1, Reviewers: map[string]struct{}{"Jane": {}}},
						{Number: 286, Additions: 500, Reviewers: map[string]struct{}{}},
					}, nil
				})

			repo, reviewers, err := svc.RepoReviewLoad(ctx, name)
			Expect(err).NotTo(HaveOccurred())
			Expect(repo).To(Equal(backend))
			Expect(reviewers).To(Equal([]reviewload.Reviewer{
				{Login: "leoregino", Score: 153, PRs: []reviewload.PullRequest{pr("backend", 373)}},
				{Login: "Jane", Score: 154, PRs: []reviewload.PullRequest{pr("backend", 371), pr("backend", 373)}},
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

var _ = Describe("ReviewLoad", func() {
	var (
		mockClient *mock_github.MockClient
		svc        reviewload.Service
		backend    = repository("backend")
		marvinRepo = repository("marvin")
		archived   = repository("legacy")
		quiet      = repository("quiet") // Nobody reviewing its open PRs
	)

	BeforeEach(func() {
		archived.Archived = gogithub.Ptr(true)
		mockClient = mock_github.NewMockClient(gomock.NewController(GinkgoT()))
		svc = reviewload.NewService(mockClient)
		mockClient.EXPECT().ListInstalledRepos(gomock.Any(), gomock.Any()).
			Return(&gogithub.ListRepositories{Repositories: []*gogithub.Repository{marvinRepo, quiet, archived, backend}}, &gogithub.Response{}, nil)
	})

	It("ranks reviewers per non-archived installed repository someone is reviewing", func(ctx SpecContext) {
		loads := map[string][]pkggithub.OpenPRReviewLoad{
			"backend": {
				{Number: 373, Additions: 100, Reviewers: map[string]struct{}{"Jane": {}, "leoregino": {}}},
			},
			"marvin": {
				{Number: 20, Additions: 50, Reviewers: map[string]struct{}{"Jane": {}}},
				{Number: 18, Additions: 5, Reviewers: map[string]struct{}{"Jane": {}}},
			},
		}
		mockClient.EXPECT().ListOpenPRsWithReviewers(gomock.Any(), gomock.Any()).Times(3).
			DoAndReturn(func(_ any, webhook pkggithub.RepoSenderGetter) ([]pkggithub.OpenPRReviewLoad, error) {
				return loads[webhook.GetRepo().GetName()], nil
			})

		Expect(svc.ReviewLoad(ctx)).To(Equal([]reviewload.RepositoryReviewers{
			{Repo: backend, Reviewers: []reviewload.Reviewer{
				{Login: "Jane", Score: 100, PRs: []reviewload.PullRequest{pr("backend", 373)}},
				{Login: "leoregino", Score: 100, PRs: []reviewload.PullRequest{pr("backend", 373)}},
			}},
			{Repo: marvinRepo, Reviewers: []reviewload.Reviewer{
				{Login: "Jane", Score: 55, PRs: []reviewload.PullRequest{pr("marvin", 18), pr("marvin", 20)}},
			}},
		}))
	})

	It("surfaces GitHub errors", func(ctx SpecContext) {
		mockClient.EXPECT().ListOpenPRsWithReviewers(gomock.Any(), gomock.Any()).Return(nil, errors.New("boom")).MinTimes(1)

		_, err := svc.ReviewLoad(ctx)
		Expect(err).To(MatchError(ContainSubstring("boom")))
	})
})

var _ = Describe("Rank", func() {
	var (
		mockClient *mock_github.MockClient
		svc        reviewload.Service
		webhook    = &gogithub.PullRequestEvent{Repo: repository("backend")}
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
			{Login: "outsider", Score: 10, PRs: []reviewload.PullRequest{pr("backend", 2)}},
			{Login: "bob", Score: 15, PRs: []reviewload.PullRequest{pr("backend", 1), pr("backend", 2)}},
		}))
	})

	It("ranks only members, including those reviewing nothing", func(ctx SpecContext) {
		Expect(svc.Rank(ctx, webhook, []string{"bob", "carol", "alice"})).To(Equal([]reviewload.Reviewer{
			{Login: "alice"},
			{Login: "carol"},
			{Login: "bob", Score: 15, PRs: []reviewload.PullRequest{pr("backend", 1), pr("backend", 2)}},
		}))
	})
})
