package marvin_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/Flashgap/marvin/internal/config"
	"github.com/Flashgap/marvin/internal/service/marvin"
)

var _ = Describe("GetGitHubRepositoryConfigurations", func() {
	It("enables RequireAIReview and carries the configured AI reviewer logins", func() {
		cfg := config.Marvin{
			MarvinRepositories:     map[string]string{repoName: "require_ai_review"},
			MarvinAIReviewerLogins: []string{"my-custom-bot"},
		}

		configs := marvin.GetGitHubRepositoryConfigurations(cfg)

		Expect(configs).To(HaveKey(repoName))
		Expect(configs[repoName].RequireAIReview).To(BeTrue())
		Expect(configs[repoName].AIReviewerLogins).To(Equal([]string{"my-custom-bot"}))
	})

	It("leaves AIReviewerLogins empty when require_ai_review is not enabled", func() {
		cfg := config.Marvin{
			MarvinRepositories:     map[string]string{repoName: "auto_merge"},
			MarvinAIReviewerLogins: []string{"my-custom-bot"},
		}

		configs := marvin.GetGitHubRepositoryConfigurations(cfg)

		Expect(configs[repoName].RequireAIReview).To(BeFalse())
		Expect(configs[repoName].AIReviewerLogins).To(BeEmpty())
	})
})
