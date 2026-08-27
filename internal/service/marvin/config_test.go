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
	mock_github "github.com/Flashgap/marvin/pkg/github/mock"
	"github.com/Flashgap/marvin/pkg/utils"
)

var _ = Describe("DefaultAIReviewerLogins", func() {
	It("lists the known AI reviewer bot logins", func() {
		Expect(marvin.DefaultAIReviewerLogins).To(ConsistOf(
			"coderabbitai[bot]",
			"graphite-app[bot]",
			"copilot-pull-request-reviewer[bot]",
		))
	})
})

var _ = Describe("RepoConfigProvider", func() {
	var mockCtrl *gomock.Controller
	var mockGithub *mock_github.MockClient
	var webhook *gogithub.PullRequestEvent
	var repo *gogithub.Repository

	const defaultBranch = "main"

	BeforeEach(func() {
		mockCtrl = gomock.NewController(GinkgoT())
		mockGithub = mock_github.NewMockClient(mockCtrl)
		repo = &gogithub.Repository{
			Name:          utils.Ptr(repoName),
			FullName:      utils.Ptr("acme/" + repoName),
			DefaultBranch: utils.Ptr(defaultBranch),
		}
		webhook = &gogithub.PullRequestEvent{
			Repo:   repo,
			Sender: &gogithub.User{},
		}
	})

	// expectListInstalledRepos sets up a single-page installation repo listing containing just
	// repo, matching what Start's poll calls before fetching each repository's .marvin.yaml.
	expectListInstalledRepos := func() {
		mockGithub.EXPECT().
			ListInstalledRepos(gomock.Any(), gomock.Any()).
			Return(&gogithub.ListRepositories{Repositories: []*gogithub.Repository{repo}}, &gogithub.Response{NextPage: 0}, nil).
			Times(1)
	}

	It("never calls the GitHub API from Get — only Start polls, Get only reads the cache", func() {
		expectListInstalledRepos()
		mockGithub.EXPECT().
			GetFileContent(gomock.Any(), gomock.Any(), marvin.RepoConfigFileName, defaultBranch).
			Return("features: [auto_merge]", &gogithub.Response{}, nil).
			Times(1)

		provider := marvin.NewRepoConfigProvider(mockGithub, config.Marvin{})
		provider.Start(context.Background(), 0)

		for range 5 {
			cfg, warning := provider.Get(webhook)
			Expect(warning).To(BeNil())
			Expect(cfg.AutoMerge).To(BeTrue())
		}
	})

	It("returns a nil config for every repository until Start has run", func() {
		provider := marvin.NewRepoConfigProvider(mockGithub, config.Marvin{})

		cfg, warning := provider.Get(webhook)
		Expect(cfg).To(BeNil())
		Expect(warning).To(BeNil())
	})

	It("fetches .marvin.yaml off the repository's default branch, not the PR head", func() {
		expectListInstalledRepos()
		mockGithub.EXPECT().
			GetFileContent(gomock.Any(), gomock.Any(), marvin.RepoConfigFileName, defaultBranch).
			Return("features: [auto_merge]", &gogithub.Response{}, nil).
			Times(1)

		provider := marvin.NewRepoConfigProvider(mockGithub, config.Marvin{})
		provider.Start(context.Background(), 0)

		cfg, warning := provider.Get(webhook)
		Expect(warning).To(BeNil())
		Expect(cfg.AutoMerge).To(BeTrue())
	})

	It("enables RequireAIReview and carries the default AI reviewer logins plus the configured ones", func() {
		expectListInstalledRepos()
		mockGithub.EXPECT().
			GetFileContent(gomock.Any(), gomock.Any(), marvin.RepoConfigFileName, defaultBranch).
			Return("features: [require_ai_review]\nai_review:\n  reviewer_logins: [my-custom-bot]", &gogithub.Response{}, nil).
			Times(1)

		provider := marvin.NewRepoConfigProvider(mockGithub, config.Marvin{MarvinAIReviewerLogins: []string{"org-bot"}})
		provider.Start(context.Background(), 0)

		cfg, _ := provider.Get(webhook)
		Expect(cfg.RequireAIReview).To(BeTrue())
		Expect(cfg.AIReviewerLogins).To(Equal(append(append(append([]string{}, marvin.DefaultAIReviewerLogins...), "org-bot"), "my-custom-bot")))
	})

	It("leaves AIReviewerLogins empty when require_ai_review is not enabled", func() {
		expectListInstalledRepos()
		mockGithub.EXPECT().
			GetFileContent(gomock.Any(), gomock.Any(), marvin.RepoConfigFileName, defaultBranch).
			Return("features: [auto_merge]", &gogithub.Response{}, nil).
			Times(1)

		provider := marvin.NewRepoConfigProvider(mockGithub, config.Marvin{MarvinAIReviewerLogins: []string{"org-bot"}})
		provider.Start(context.Background(), 0)

		cfg, _ := provider.Get(webhook)
		Expect(cfg.RequireAIReview).To(BeFalse())
		Expect(cfg.AIReviewerLogins).To(BeEmpty())
	})

	It("parses reviewers.rules into ReviewRules and reviewers.default_team into DefaultTeam", func() {
		expectListInstalledRepos()
		mockGithub.EXPECT().
			GetFileContent(gomock.Any(), gomock.Any(), marvin.RepoConfigFileName, defaultBranch).
			Return(`
reviewers:
  default_team: platform
  rules:
    - path: "go/**"
      team: backend-team
    - path: "py/**"
      team: data-team
`, &gogithub.Response{}, nil).
			Times(1)

		provider := marvin.NewRepoConfigProvider(mockGithub, config.Marvin{})
		provider.Start(context.Background(), 0)

		cfg, _ := provider.Get(webhook)
		Expect(cfg.DefaultTeam).To(Equal("platform"))
		Expect(cfg.ReviewRules).To(Equal([]marvin.PathReviewRule{
			{Pattern: "go/**", Team: "backend-team"},
			{Pattern: "py/**", Team: "data-team"},
		}))
	})

	It("defaults ChangelogFile to CHANGELOG.md when check_changelog.file is not set", func() {
		expectListInstalledRepos()
		mockGithub.EXPECT().
			GetFileContent(gomock.Any(), gomock.Any(), marvin.RepoConfigFileName, defaultBranch).
			Return("features: [check_changelog]", &gogithub.Response{}, nil).
			Times(1)

		provider := marvin.NewRepoConfigProvider(mockGithub, config.Marvin{})
		provider.Start(context.Background(), 0)

		cfg, _ := provider.Get(webhook)
		Expect(cfg.ChangelogFile).To(Equal(marvin.DefaultChangelogFile))
	})

	It("overrides ChangelogFile from check_changelog.file", func() {
		expectListInstalledRepos()
		mockGithub.EXPECT().
			GetFileContent(gomock.Any(), gomock.Any(), marvin.RepoConfigFileName, defaultBranch).
			Return(`
features: [check_changelog]
check_changelog:
  file: docs/CHANGELOG.md
`, &gogithub.Response{}, nil).
			Times(1)

		provider := marvin.NewRepoConfigProvider(mockGithub, config.Marvin{})
		provider.Start(context.Background(), 0)

		cfg, _ := provider.Get(webhook)
		Expect(cfg.ChangelogFile).To(Equal("docs/CHANGELOG.md"))
	})

	It("disables Marvin for the repository (nil config, no warning) when .marvin.yaml is missing", func() {
		expectListInstalledRepos()
		mockGithub.EXPECT().
			GetFileContent(gomock.Any(), gomock.Any(), marvin.RepoConfigFileName, defaultBranch).
			Return("", &gogithub.Response{Response: &http.Response{StatusCode: http.StatusNotFound}}, errors.New("404 Not Found")).
			Times(1)

		provider := marvin.NewRepoConfigProvider(mockGithub, config.Marvin{})
		provider.Start(context.Background(), 0)

		cfg, warning := provider.Get(webhook)
		Expect(warning).To(BeNil())
		Expect(cfg).To(BeNil())
	})

	It("returns a nil config and a warning (not an error) when .marvin.yaml is invalid YAML on the first poll", func() {
		expectListInstalledRepos()
		mockGithub.EXPECT().
			GetFileContent(gomock.Any(), gomock.Any(), marvin.RepoConfigFileName, defaultBranch).
			Return("features: [auto_merge", &gogithub.Response{}, nil).
			Times(1)

		provider := marvin.NewRepoConfigProvider(mockGithub, config.Marvin{})
		provider.Start(context.Background(), 0)

		cfg, warning := provider.Get(webhook)
		Expect(cfg).To(BeNil())
		Expect(warning).NotTo(BeNil())
		Expect(warning.UsedFallback).To(BeFalse())
	})

	It("returns a nil config and a warning (not an error) on an unexpected fetch failure on the first poll", func() {
		expectListInstalledRepos()
		mockGithub.EXPECT().
			GetFileContent(gomock.Any(), gomock.Any(), marvin.RepoConfigFileName, defaultBranch).
			Return("", &gogithub.Response{Response: &http.Response{StatusCode: http.StatusInternalServerError}}, errors.New("boom")).
			Times(1)

		provider := marvin.NewRepoConfigProvider(mockGithub, config.Marvin{})
		provider.Start(context.Background(), 0)

		cfg, warning := provider.Get(webhook)
		Expect(cfg).To(BeNil())
		Expect(warning).NotTo(BeNil())
		Expect(warning.UsedFallback).To(BeFalse())
	})

	It("falls back to the last known-good config and returns a warning once .marvin.yaml becomes invalid YAML on a later poll", func() {
		firstList := mockGithub.EXPECT().
			ListInstalledRepos(gomock.Any(), gomock.Any()).
			Return(&gogithub.ListRepositories{Repositories: []*gogithub.Repository{repo}}, &gogithub.Response{NextPage: 0}, nil)
		firstFetch := mockGithub.EXPECT().
			GetFileContent(gomock.Any(), gomock.Any(), marvin.RepoConfigFileName, defaultBranch).
			Return("features: [auto_merge]", &gogithub.Response{}, nil).
			After(firstList)
		secondList := mockGithub.EXPECT().
			ListInstalledRepos(gomock.Any(), gomock.Any()).
			Return(&gogithub.ListRepositories{Repositories: []*gogithub.Repository{repo}}, &gogithub.Response{NextPage: 0}, nil).
			After(firstFetch)
		mockGithub.EXPECT().
			GetFileContent(gomock.Any(), gomock.Any(), marvin.RepoConfigFileName, defaultBranch).
			Return("features: [auto_merge", &gogithub.Response{}, nil).
			After(secondList)

		provider := marvin.NewRepoConfigProvider(mockGithub, config.Marvin{})

		provider.Start(context.Background(), 0)
		good, warning := provider.Get(webhook)
		Expect(warning).To(BeNil())
		Expect(good.AutoMerge).To(BeTrue())

		provider.Start(context.Background(), 0)
		cfg, warning := provider.Get(webhook)
		Expect(cfg).To(Equal(good))
		Expect(warning).NotTo(BeNil())
		Expect(warning.UsedFallback).To(BeTrue())
	})

	It("keeps every previously cached configuration when listing installed repositories fails", func() {
		firstList := mockGithub.EXPECT().
			ListInstalledRepos(gomock.Any(), gomock.Any()).
			Return(&gogithub.ListRepositories{Repositories: []*gogithub.Repository{repo}}, &gogithub.Response{NextPage: 0}, nil)
		firstFetch := mockGithub.EXPECT().
			GetFileContent(gomock.Any(), gomock.Any(), marvin.RepoConfigFileName, defaultBranch).
			Return("features: [auto_merge]", &gogithub.Response{}, nil).
			After(firstList)
		mockGithub.EXPECT().
			ListInstalledRepos(gomock.Any(), gomock.Any()).
			Return(nil, nil, errors.New("boom")).
			After(firstFetch)

		provider := marvin.NewRepoConfigProvider(mockGithub, config.Marvin{})

		provider.Start(context.Background(), 0)
		good, warning := provider.Get(webhook)
		Expect(warning).To(BeNil())
		Expect(good.AutoMerge).To(BeTrue())

		provider.Start(context.Background(), 0)
		cfg, warning := provider.Get(webhook)
		Expect(warning).To(BeNil())
		Expect(cfg).To(Equal(good))
	})
})
