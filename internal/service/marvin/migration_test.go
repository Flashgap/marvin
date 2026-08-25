package marvin_test

import (
	"context"
	"errors"
	"net/http"

	gogithub "github.com/google/go-github/v63/github"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"

	svcgithub "github.com/Flashgap/marvin/internal/service/github"
	mock_jira "github.com/Flashgap/marvin/internal/service/jira/mock"
	"github.com/Flashgap/marvin/internal/service/marvin"
	mock_slack "github.com/Flashgap/marvin/internal/service/slack/mock"
	pkggithub "github.com/Flashgap/marvin/pkg/github"
	mock_github "github.com/Flashgap/marvin/pkg/github/mock"
	mock_linear "github.com/Flashgap/marvin/pkg/linear/mock"
	"github.com/Flashgap/marvin/pkg/utils"
)

var _ = Describe("Config migration", func() {
	const defaultBranch = "main"

	var mockCtrl *gomock.Controller
	var mockGithub *mock_github.MockClient
	var githubService svcgithub.Service
	var mockSlack *mock_slack.MockService
	var mockLinear *mock_linear.MockClient
	var mockJira *mock_jira.MockService
	var prEvent *gogithub.PullRequestEvent

	BeforeEach(func() {
		mockCtrl = gomock.NewController(GinkgoT())
		mockGithub = mock_github.NewMockClient(mockCtrl)
		mockSlack = mock_slack.NewMockService(mockCtrl)
		mockLinear = mock_linear.NewMockClient(mockCtrl)
		mockJira = mock_jira.NewMockService(mockCtrl)
		githubService = svcgithub.NewService(mockGithub)

		prEvent = &gogithub.PullRequestEvent{
			Action: new(pkggithub.EventPullRequestActionOpened),
			Repo: &gogithub.Repository{
				Name:          new(repoName),
				DefaultBranch: new(defaultBranch),
			},
			Sender: &gogithub.User{Login: new("mx")},
			PullRequest: &gogithub.PullRequest{
				Number: new(1),
				State:  new("open"),
				User:   &gogithub.User{Login: new("dave")},
			},
		}
	})

	When("the repository has a legacy MARVIN_REPOSITORIES entry and no .marvin.yaml", func() {
		It("opens a migration PR carrying the legacy config forward as YAML", func(ctx SpecContext) {
			legacyConfig := marvin.LegacyConfig{
				Repositories:   map[string]string{repoName: "auto_merge;check_title"},
				ReviewersTeams: map[string]string{repoName: "backend-team"},
			}
			cfgs := marvin.StaticRepoConfigProvider{}

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

			svc := marvin.NewService(githubService, mockJira, mockLinear, mockSlack, cfgs, legacyConfig, testPRParserConfig)
			err := svc.OnPullRequest(ctx, prEvent)
			Expect(err).NotTo(HaveOccurred())
		})
	})

	When("the migration branch already exists (already proposed once)", func() {
		It("stops before creating the file or the PR", func(ctx SpecContext) {
			legacyConfig := marvin.LegacyConfig{
				Repositories: map[string]string{repoName: "auto_merge"},
			}
			cfgs := marvin.StaticRepoConfigProvider{}

			mockGithub.EXPECT().GetRef(gomock.Any(), gomock.Any(), "refs/heads/"+defaultBranch).
				Return(&gogithub.Reference{Object: &gogithub.GitObject{SHA: utils.Ptr("base-sha")}}, nil, nil)

			mockGithub.EXPECT().CreateRef(gomock.Any(), gomock.Any(), gomock.Any()).
				Return(nil, &gogithub.Response{Response: &http.Response{StatusCode: http.StatusUnprocessableEntity}}, errors.New("reference already exists"))

			// No CreateFile / CreatePullRequest expected: gomock fails the test if either is called.

			svc := marvin.NewService(githubService, mockJira, mockLinear, mockSlack, cfgs, legacyConfig, testPRParserConfig)
			err := svc.OnPullRequest(ctx, prEvent)
			Expect(err).NotTo(HaveOccurred())
		})
	})

	When("the repository has no legacy entry", func() {
		It("does nothing", func(ctx SpecContext) {
			cfgs := marvin.StaticRepoConfigProvider{}

			// No GetHub calls expected at all.

			svc := marvin.NewService(githubService, mockJira, mockLinear, mockSlack, cfgs, marvin.LegacyConfig{}, testPRParserConfig)
			err := svc.OnPullRequest(ctx, prEvent)
			Expect(err).NotTo(HaveOccurred())
		})
	})
})
