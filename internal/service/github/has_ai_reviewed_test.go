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
		baseRef  = "master"
		headSHA  = "3323bd58" // merge commit of master into the branch
		oldSHA   = "2d0f2707" // the commit CodeRabbit actually reviewed
	)
	aiLogins := []string{"coderabbitai[bot]"}

	var (
		mockCtrl   *gomock.Controller
		mockClient *mock_github.MockClient
		svc        svcgithub.Service
		pr         *gogithub.PullRequest
		event      *gogithub.PullRequestEvent
	)

	// files builds a CompareCommits result from filename->blobSHA pairs.
	files := func(pairs map[string]string) *gogithub.CommitsComparison {
		out := make([]*gogithub.CommitFile, 0, len(pairs))
		for name, sha := range pairs {
			out = append(out, &gogithub.CommitFile{
				Filename: utils.Ptr(name),
				Status:   utils.Ptr("modified"),
				SHA:      utils.Ptr(sha),
			})
		}
		return &gogithub.CommitsComparison{Files: out}
	}

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
			Base:   &gogithub.PullRequestBranch{Ref: utils.Ptr(baseRef)},
		}
		event = &gogithub.PullRequestEvent{
			Repo:        &gogithub.Repository{Name: utils.Ptr("infra"), Owner: &gogithub.User{Login: utils.Ptr("hector-finance")}},
			PullRequest: pr,
		}
	})

	AfterEach(func() { mockCtrl.Finish() })

	When("an AI review lands exactly on HEAD", func() {
		It("passes without any commit comparison", func(ctx SpecContext) {
			mockClient.EXPECT().ListReviews(gomock.Any(), event, prNumber, gomock.Any()).
				Return([]*gogithub.PullRequestReview{review("coderabbitai[bot]", headSHA)}, nil, nil)
			// No CompareCommits expected on the fast path.

			ok, err := svc.HasAIReviewed(ctx, event, pr, aiLogins)
			Expect(err).NotTo(HaveOccurred())
			Expect(ok).To(BeTrue())
		})
	})

	When("HEAD is a base-branch merge with no new branch changes", func() {
		It("passes because the branch diff is unchanged since the AI review", func(ctx SpecContext) {
			branchDiff := map[string]string{"main.go": "blobAAA", "go.mod": "blobBBB"}

			mockClient.EXPECT().ListReviews(gomock.Any(), event, prNumber, gomock.Any()).
				Return([]*gogithub.PullRequestReview{review("coderabbitai[bot]", oldSHA)}, nil, nil)
			// Same branch diff (identical file blobs) at HEAD and at the reviewed commit.
			mockClient.EXPECT().CompareCommits(gomock.Any(), event, baseRef, headSHA, gomock.Any()).
				Return(files(branchDiff), nil, nil)
			mockClient.EXPECT().CompareCommits(gomock.Any(), event, baseRef, oldSHA, gomock.Any()).
				Return(files(branchDiff), nil, nil)

			ok, err := svc.HasAIReviewed(ctx, event, pr, aiLogins)
			Expect(err).NotTo(HaveOccurred())
			Expect(ok).To(BeTrue())
		})
	})

	When("new branch code was pushed after the AI review", func() {
		It("blocks because the branch diff differs from the reviewed commit", func(ctx SpecContext) {
			mockClient.EXPECT().ListReviews(gomock.Any(), event, prNumber, gomock.Any()).
				Return([]*gogithub.PullRequestReview{review("coderabbitai[bot]", oldSHA)}, nil, nil)
			mockClient.EXPECT().CompareCommits(gomock.Any(), event, baseRef, headSHA, gomock.Any()).
				Return(files(map[string]string{"main.go": "blobNEW", "go.mod": "blobBBB"}), nil, nil)
			mockClient.EXPECT().CompareCommits(gomock.Any(), event, baseRef, oldSHA, gomock.Any()).
				Return(files(map[string]string{"main.go": "blobAAA", "go.mod": "blobBBB"}), nil, nil)

			ok, err := svc.HasAIReviewed(ctx, event, pr, aiLogins)
			Expect(err).NotTo(HaveOccurred())
			Expect(ok).To(BeFalse())
		})
	})

	When("no AI reviewer has reviewed at all", func() {
		It("blocks without comparing commits", func(ctx SpecContext) {
			mockClient.EXPECT().ListReviews(gomock.Any(), event, prNumber, gomock.Any()).
				Return([]*gogithub.PullRequestReview{review("some-human", oldSHA)}, nil, nil)

			ok, err := svc.HasAIReviewed(ctx, event, pr, aiLogins)
			Expect(err).NotTo(HaveOccurred())
			Expect(ok).To(BeFalse())
		})
	})
})
