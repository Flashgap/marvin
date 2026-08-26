package marvin_test

import (
	"context"
	"errors"
	"net/http"
	"time"

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

	const defaultBranch = "main"

	BeforeEach(func() {
		mockCtrl = gomock.NewController(GinkgoT())
		mockGithub = mock_github.NewMockClient(mockCtrl)
		webhook = &gogithub.PullRequestEvent{
			Repo: &gogithub.Repository{
				Name:          utils.Ptr(repoName),
				FullName:      utils.Ptr("acme/" + repoName),
				DefaultBranch: utils.Ptr(defaultBranch),
			},
			Sender: &gogithub.User{},
		}
	})

	It("fetches .marvin.yaml off the repository's default branch, not the PR head", func() {
		mockGithub.EXPECT().
			GetFileContent(gomock.Any(), gomock.Any(), marvin.RepoConfigFileName, defaultBranch).
			Return("features: [auto_merge]", &gogithub.Response{}, nil).
			Times(1)

		provider := marvin.NewRepoConfigProvider(mockGithub, config.Marvin{})
		cfg, warning, err := provider.Get(context.Background(), webhook)
		Expect(err).NotTo(HaveOccurred())
		Expect(warning).To(BeNil())
		Expect(cfg.AutoMerge).To(BeTrue())
	})

	It("enables RequireAIReview and carries the default AI reviewer logins plus the configured ones", func() {
		mockGithub.EXPECT().
			GetFileContent(gomock.Any(), gomock.Any(), marvin.RepoConfigFileName, defaultBranch).
			Return("features: [require_ai_review]\nai_review:\n  reviewer_logins: [my-custom-bot]", &gogithub.Response{}, nil).
			Times(1)

		provider := marvin.NewRepoConfigProvider(mockGithub, config.Marvin{MarvinAIReviewerLogins: []string{"org-bot"}})
		cfg, _, err := provider.Get(context.Background(), webhook)
		Expect(err).NotTo(HaveOccurred())
		Expect(cfg.RequireAIReview).To(BeTrue())
		Expect(cfg.AIReviewerLogins).To(Equal(append(append(append([]string{}, marvin.DefaultAIReviewerLogins...), "org-bot"), "my-custom-bot")))
	})

	It("leaves AIReviewerLogins empty when require_ai_review is not enabled", func() {
		mockGithub.EXPECT().
			GetFileContent(gomock.Any(), gomock.Any(), marvin.RepoConfigFileName, defaultBranch).
			Return("features: [auto_merge]", &gogithub.Response{}, nil).
			Times(1)

		provider := marvin.NewRepoConfigProvider(mockGithub, config.Marvin{MarvinAIReviewerLogins: []string{"org-bot"}})
		cfg, _, err := provider.Get(context.Background(), webhook)
		Expect(err).NotTo(HaveOccurred())
		Expect(cfg.RequireAIReview).To(BeFalse())
		Expect(cfg.AIReviewerLogins).To(BeEmpty())
	})

	It("parses reviewers.rules into ReviewRules and reviewers.default_team into DefaultTeam", func() {
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
		cfg, _, err := provider.Get(context.Background(), webhook)
		Expect(err).NotTo(HaveOccurred())
		Expect(cfg.DefaultTeam).To(Equal("platform"))
		Expect(cfg.ReviewRules).To(Equal([]marvin.PathReviewRule{
			{Pattern: "go/**", Team: "backend-team"},
			{Pattern: "py/**", Team: "data-team"},
		}))
	})

	It("defaults ChangelogFile to CHANGELOG.md when check_changelog.file is not set", func() {
		mockGithub.EXPECT().
			GetFileContent(gomock.Any(), gomock.Any(), marvin.RepoConfigFileName, defaultBranch).
			Return("features: [check_changelog]", &gogithub.Response{}, nil).
			Times(1)

		provider := marvin.NewRepoConfigProvider(mockGithub, config.Marvin{})
		cfg, _, err := provider.Get(context.Background(), webhook)
		Expect(err).NotTo(HaveOccurred())
		Expect(cfg.ChangelogFile).To(Equal(marvin.DefaultChangelogFile))
	})

	It("overrides ChangelogFile from check_changelog.file", func() {
		mockGithub.EXPECT().
			GetFileContent(gomock.Any(), gomock.Any(), marvin.RepoConfigFileName, defaultBranch).
			Return(`
features: [check_changelog]
check_changelog:
  file: docs/CHANGELOG.md
`, &gogithub.Response{}, nil).
			Times(1)

		provider := marvin.NewRepoConfigProvider(mockGithub, config.Marvin{})
		cfg, _, err := provider.Get(context.Background(), webhook)
		Expect(err).NotTo(HaveOccurred())
		Expect(cfg.ChangelogFile).To(Equal("docs/CHANGELOG.md"))
	})

	It("disables Marvin for the repository (nil config, no error) when .marvin.yaml is missing", func() {
		mockGithub.EXPECT().
			GetFileContent(gomock.Any(), gomock.Any(), marvin.RepoConfigFileName, defaultBranch).
			Return("", &gogithub.Response{Response: &http.Response{StatusCode: http.StatusNotFound}}, errors.New("404 Not Found")).
			Times(1)

		provider := marvin.NewRepoConfigProvider(mockGithub, config.Marvin{})
		cfg, warning, err := provider.Get(context.Background(), webhook)
		Expect(err).NotTo(HaveOccurred())
		Expect(warning).To(BeNil())
		Expect(cfg).To(BeNil())
	})

	It("falls back to the last known-good config and returns a warning when .marvin.yaml becomes invalid YAML", func() {
		gomock.InOrder(
			mockGithub.EXPECT().
				GetFileContent(gomock.Any(), gomock.Any(), marvin.RepoConfigFileName, defaultBranch).
				Return("features: [auto_merge]", &gogithub.Response{}, nil).
				Times(1),
			mockGithub.EXPECT().
				GetFileContent(gomock.Any(), gomock.Any(), marvin.RepoConfigFileName, defaultBranch).
				Return("features: [auto_merge", &gogithub.Response{}, nil).
				Times(1),
		)

		provider := marvin.NewRepoConfigProvider(mockGithub, config.Marvin{MarvinRepoConfigCacheTTL: time.Nanosecond})

		good, warning, err := provider.Get(context.Background(), webhook)
		Expect(err).NotTo(HaveOccurred())
		Expect(warning).To(BeNil())
		Expect(good.AutoMerge).To(BeTrue())

		time.Sleep(time.Millisecond)

		cfg, warning, err := provider.Get(context.Background(), webhook)
		Expect(err).NotTo(HaveOccurred())
		Expect(cfg).To(Equal(good))
		Expect(warning).NotTo(BeNil())
		Expect(warning.UsedFallback).To(BeTrue())
	})

	It("returns a nil config and a warning (not an error) when .marvin.yaml is invalid YAML on the first fetch", func() {
		mockGithub.EXPECT().
			GetFileContent(gomock.Any(), gomock.Any(), marvin.RepoConfigFileName, defaultBranch).
			Return("features: [auto_merge", &gogithub.Response{}, nil).
			Times(1)

		provider := marvin.NewRepoConfigProvider(mockGithub, config.Marvin{})
		cfg, warning, err := provider.Get(context.Background(), webhook)
		Expect(err).NotTo(HaveOccurred())
		Expect(cfg).To(BeNil())
		Expect(warning).NotTo(BeNil())
		Expect(warning.UsedFallback).To(BeFalse())
	})

	It("returns a nil config and a warning (not an error) on an unexpected fetch failure on the first fetch", func() {
		mockGithub.EXPECT().
			GetFileContent(gomock.Any(), gomock.Any(), marvin.RepoConfigFileName, defaultBranch).
			Return("", &gogithub.Response{Response: &http.Response{StatusCode: http.StatusInternalServerError}}, errors.New("boom")).
			Times(1)

		provider := marvin.NewRepoConfigProvider(mockGithub, config.Marvin{})
		cfg, warning, err := provider.Get(context.Background(), webhook)
		Expect(err).NotTo(HaveOccurred())
		Expect(cfg).To(BeNil())
		Expect(warning).NotTo(BeNil())
		Expect(warning.UsedFallback).To(BeFalse())
	})

	It("caches the resolved config within the TTL and refetches once it expires", func() {
		mockGithub.EXPECT().
			GetFileContent(gomock.Any(), gomock.Any(), marvin.RepoConfigFileName, defaultBranch).
			Return("features: [auto_merge]", &gogithub.Response{}, nil).
			Times(2)

		provider := marvin.NewRepoConfigProvider(mockGithub, config.Marvin{MarvinRepoConfigCacheTTL: 10 * time.Millisecond})

		_, _, err := provider.Get(context.Background(), webhook)
		Expect(err).NotTo(HaveOccurred())

		// Within TTL: served from cache, no second call yet.
		_, _, err = provider.Get(context.Background(), webhook)
		Expect(err).NotTo(HaveOccurred())

		time.Sleep(20 * time.Millisecond)

		// TTL expired: refetches.
		_, _, err = provider.Get(context.Background(), webhook)
		Expect(err).NotTo(HaveOccurred())
	})
})
