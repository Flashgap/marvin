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
	mock_reviewload "github.com/Flashgap/marvin/internal/service/reviewload/mock"
	pkggithub "github.com/Flashgap/marvin/pkg/github"
	mock_github "github.com/Flashgap/marvin/pkg/github/mock"
	"github.com/Flashgap/marvin/pkg/linguist"
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

// noGitattributes excludes no file of any repository.
func noGitattributes(ctrl *gomock.Controller) reviewload.Gitattributes {
	gitattributes := mock_reviewload.NewMockGitattributes(ctrl)
	gitattributes.EXPECT().Gitattributes(gomock.Any()).Return(nil).AnyTimes()
	return gitattributes
}

var _ = Describe("RepoReviewLoad", func() {
	var (
		mockClient *mock_github.MockClient
		svc        reviewload.Service
		backend    = repository("backend")
		marvinRepo = repository("marvin")
	)

	BeforeEach(func() {
		ctrl := gomock.NewController(GinkgoT())
		mockClient = mock_github.NewMockClient(ctrl)
		svc = reviewload.NewService(mockClient, noGitattributes(ctrl))
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
		ctrl := gomock.NewController(GinkgoT())
		mockClient = mock_github.NewMockClient(ctrl)
		svc = reviewload.NewService(mockClient, noGitattributes(ctrl))
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
		ctrl := gomock.NewController(GinkgoT())
		mockClient = mock_github.NewMockClient(ctrl)
		svc = reviewload.NewService(mockClient, noGitattributes(ctrl))
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

var _ = Describe("Rank without generated and vendored files", func() {
	var (
		mockClient    *mock_github.MockClient
		gitattributes *mock_reviewload.MockGitattributes
		svc           reviewload.Service
		webhook       = &gogithub.PullRequestEvent{Repo: repository("backend")}
		// bob's PR adds 30 lines of Go and 900 of protobuf codegen
		protoPR = pkggithub.OpenPRReviewLoad{
			Number: 1, Additions: 930,
			Files:     []pkggithub.FileAdditions{{Path: "go/api.go", Additions: 30}, {Path: "go/api.pb.go", Additions: 900}},
			Reviewers: map[string]struct{}{"bob": {}},
		}
		// alice's PR changes more files than the GraphQL query lists
		bigPR = pkggithub.OpenPRReviewLoad{
			Number: 2, Additions: 2000,
			Files:     []pkggithub.FileAdditions{{Path: "go/a.go", Additions: 1}},
			MoreFiles: true,
			Reviewers: map[string]struct{}{"alice": {}},
		}
	)

	// excludeGenerated makes the repository's .gitattributes mark protobuf codegen linguist-generated
	excludeGenerated := func() {
		gitattributes.EXPECT().Gitattributes(webhook.GetRepo()).
			Return(linguist.NewMatcher(map[string]string{"go/.gitattributes": "*.pb.go linguist-generated\n"}))
	}

	BeforeEach(func() {
		ctrl := gomock.NewController(GinkgoT())
		mockClient = mock_github.NewMockClient(ctrl)
		gitattributes = mock_reviewload.NewMockGitattributes(ctrl)
		svc = reviewload.NewService(mockClient, gitattributes)
	})

	It("leaves out the files the repository's .gitattributes exclude", func(ctx SpecContext) {
		excludeGenerated()
		mockClient.EXPECT().ListOpenPRsWithReviewers(gomock.Any(), webhook).Return([]pkggithub.OpenPRReviewLoad{protoPR}, nil)

		Expect(svc.Rank(ctx, webhook, nil)).To(Equal([]reviewload.Reviewer{
			{Login: "bob", Score: 30, PRs: []reviewload.PullRequest{pr("backend", 1)}},
		}))
	})

	DescribeTable("lists every file of a PR changing more than the query lists",
		func(ctx SpecContext, lastPageErr error, score int) {
			excludeGenerated()
			mockClient.EXPECT().ListOpenPRsWithReviewers(gomock.Any(), webhook).Return([]pkggithub.OpenPRReviewLoad{bigPR}, nil)
			mockClient.EXPECT().ListPRFiles(gomock.Any(), webhook, 2, gomock.Any()).
				DoAndReturn(func(_ any, _ any, _ int, opts *gogithub.ListOptions) ([]*gogithub.CommitFile, *gogithub.Response, error) {
					if opts.Page == 1 {
						return []*gogithub.CommitFile{
							{Filename: gogithub.Ptr("go/a.go"), Additions: gogithub.Ptr(1)},
							{Filename: gogithub.Ptr("go/a.pb.go"), Additions: gogithub.Ptr(1500)},
						}, &gogithub.Response{NextPage: 2}, nil
					}
					return []*gogithub.CommitFile{{Filename: gogithub.Ptr("go/b.go"), Additions: gogithub.Ptr(499)}}, &gogithub.Response{}, lastPageErr
				}).Times(2)

			Expect(svc.Rank(ctx, webhook, nil)).To(Equal([]reviewload.Reviewer{
				{Login: "alice", Score: score, PRs: []reviewload.PullRequest{pr("backend", 2)}},
			}))
		},
		Entry("leaving out its excluded files", nil, 500),
		Entry("counting all of them when they can't all be listed", errors.New("boom"), 2000),
	)

	It("counts every file, as before, when the .gitattributes exclude none or none were read", func(ctx SpecContext) {
		gitattributes.EXPECT().Gitattributes(webhook.GetRepo()).Return(nil)
		mockClient.EXPECT().ListOpenPRsWithReviewers(gomock.Any(), webhook).Return([]pkggithub.OpenPRReviewLoad{protoPR, bigPR}, nil)

		// The PRs' additions, even for the PR whose files aren't all listed
		Expect(svc.Rank(ctx, webhook, nil)).To(Equal([]reviewload.Reviewer{
			{Login: "bob", Score: 930, PRs: []reviewload.PullRequest{pr("backend", 1)}},
			{Login: "alice", Score: 2000, PRs: []reviewload.PullRequest{pr("backend", 2)}},
		}))
	})
})
