package marvin_test

import (
	"context"
	"errors"
	"net/http"

	gogithub "github.com/google/go-github/v63/github"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"

	"github.com/Flashgap/marvin/internal/config"
	"github.com/Flashgap/marvin/internal/service/marvin"
	pkggithub "github.com/Flashgap/marvin/pkg/github"
	mock_github "github.com/Flashgap/marvin/pkg/github/mock"
	"github.com/Flashgap/marvin/pkg/utils"
)

var _ = Describe("Config migration", func() {
	const defaultBranch = "main"

	var mockCtrl *gomock.Controller
	var mockGithub *mock_github.MockClient
	var repo *gogithub.Repository

	BeforeEach(func() {
		mockCtrl = gomock.NewController(GinkgoT())
		mockGithub = mock_github.NewMockClient(mockCtrl)

		repo = &gogithub.Repository{
			Name:          new(repoName),
			FullName:      new("acme/" + repoName),
			DefaultBranch: new(defaultBranch),
		}
	})

	expectListAndNoFile := func() {
		mockGithub.EXPECT().ListInstalledRepos(gomock.Any(), gomock.Any()).
			Return(&gogithub.ListRepositories{Repositories: []*gogithub.Repository{repo}}, &gogithub.Response{NextPage: 0}, nil)
		mockGithub.EXPECT().GetFileContent(gomock.Any(), gomock.Any(), marvin.RepoConfigFileName, defaultBranch).
			Return("", &gogithub.Response{Response: &http.Response{StatusCode: http.StatusNotFound}}, errors.New("404 Not Found"))
	}

	When("the repository has a legacy MARVIN_REPOSITORIES entry and no .marvin.yaml", func() {
		It("opens a migration PR carrying the legacy config forward as YAML, and falls back to it in the meantime", func(ctx SpecContext) {
			orgConfig := config.Marvin{
				MarvinLegacyRepositories:   map[string]string{repoName: "auto_merge;check_title"},
				MarvinLegacyReviewersTeams: map[string]string{repoName: "backend-team"},
			}

			expectListAndNoFile()

			mockGithub.EXPECT().GetRef(gomock.Any(), gomock.Any(), "refs/heads/"+defaultBranch).
				Return(&gogithub.Reference{Object: &gogithub.GitObject{SHA: utils.Ptr("base-sha")}}, nil, nil)

			mockGithub.EXPECT().CreateRef(gomock.Any(), gomock.Any(), gomock.Any()).
				DoAndReturn(func(_ context.Context, _ pkggithub.RepoSenderGetter, ref *gogithub.Reference) (*gogithub.Reference, *gogithub.Response, error) {
					Expect(ref.GetRef()).To(Equal("refs/heads/marvin/add-marvin-yaml-config"))
					Expect(ref.GetObject().GetSHA()).To(Equal("base-sha"))
					return ref, &gogithub.Response{}, nil
				})

			expectedConfig := `# .marvin.yaml
#
# Auto-migrated by Marvin from the deprecated MARVIN_REPOSITORIES / MARVIN_REVIEWERS_TEAMS
# env vars. Review and adjust as needed, then merge this PR.

features:
  - auto_merge
  - check_title

reviewers:
  default_team: backend-team
`

			mockGithub.EXPECT().CreateFile(gomock.Any(), gomock.Any(), marvin.RepoConfigFileName, gomock.Any()).
				DoAndReturn(func(_ context.Context, _ pkggithub.RepoSenderGetter, path string, opts *gogithub.RepositoryContentFileOptions) (*gogithub.RepositoryContentResponse, *gogithub.Response, error) {
					Expect(opts.GetBranch()).To(Equal("marvin/add-marvin-yaml-config"))
					Expect(string(opts.Content)).To(Equal(expectedConfig))
					return &gogithub.RepositoryContentResponse{}, nil, nil
				})

			mockGithub.EXPECT().CreatePullRequest(gomock.Any(), gomock.Any(), gomock.Any()).
				DoAndReturn(func(_ context.Context, _ pkggithub.RepoSenderGetter, newPR *gogithub.NewPullRequest) (*gogithub.PullRequest, *gogithub.Response, error) {
					Expect(newPR.GetHead()).To(Equal("marvin/add-marvin-yaml-config"))
					Expect(newPR.GetBase()).To(Equal(defaultBranch))
					return &gogithub.PullRequest{Number: utils.Ptr(42)}, nil, nil
				})

			provider := marvin.NewRepoConfigProvider(mockGithub, orgConfig)
			provider.Start(ctx, 0)

			cfg, warning := provider.Get(&gogithub.PullRequestEvent{Repo: repo})
			Expect(warning).To(BeNil())
			Expect(cfg.AutoMerge).To(BeTrue())
		})
	})

	When("the migration branch already exists (already proposed once)", func() {
		It("stops before creating the file or the PR", func(ctx SpecContext) {
			orgConfig := config.Marvin{
				MarvinLegacyRepositories: map[string]string{repoName: "auto_merge"},
			}

			expectListAndNoFile()

			mockGithub.EXPECT().GetRef(gomock.Any(), gomock.Any(), "refs/heads/"+defaultBranch).
				Return(&gogithub.Reference{Object: &gogithub.GitObject{SHA: utils.Ptr("base-sha")}}, nil, nil)

			mockGithub.EXPECT().CreateRef(gomock.Any(), gomock.Any(), gomock.Any()).
				Return(nil, &gogithub.Response{Response: &http.Response{StatusCode: http.StatusUnprocessableEntity}}, errors.New("reference already exists"))

			// No CreateFile / CreatePullRequest expected: gomock fails the test if either is called.

			provider := marvin.NewRepoConfigProvider(mockGithub, orgConfig)
			provider.Start(ctx, 0)

			cfg, warning := provider.Get(&gogithub.PullRequestEvent{Repo: repo})
			Expect(warning).To(BeNil())
			Expect(cfg.AutoMerge).To(BeTrue())
		})
	})

	When("the repository has no legacy entry", func() {
		It("does nothing beyond the regular poll", func(ctx SpecContext) {
			expectListAndNoFile()

			// No GetRef / CreateRef / CreateFile / CreatePullRequest expected.

			provider := marvin.NewRepoConfigProvider(mockGithub, config.Marvin{})
			provider.Start(ctx, 0)

			cfg, warning := provider.Get(&gogithub.PullRequestEvent{Repo: repo})
			Expect(warning).To(BeNil())
			Expect(cfg).To(BeNil())
		})
	})
})
