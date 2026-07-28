package github_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/Flashgap/marvin/internal/service/github"
)

var _ = Describe("IsAIReviewerLogin", func() {
	knownAILogins := []string{"coderabbitai[bot]", "graphite-app[bot]", "copilot-pull-request-reviewer[bot]"}

	DescribeTable("matching against the given AI login list",
		func(login string, aiLogins []string, expected bool) {
			Expect(github.IsAIReviewerLogin(login, aiLogins)).To(Equal(expected))
		},
		Entry("matches coderabbitai's bot login", "coderabbitai[bot]", knownAILogins, true),
		Entry("matches graphite-app's bot login", "graphite-app[bot]", knownAILogins, true),
		Entry("matches GitHub Copilot's review login", "copilot-pull-request-reviewer[bot]", knownAILogins, true),
		Entry("matches case-insensitively", "CodeRabbitAI[bot]", knownAILogins, true),
		Entry("does not match a human login", "octocat", knownAILogins, false),
		Entry("does not match a human login that contains a bot name as substring", "clement-copilot", knownAILogins, false),
		Entry("does not match a human login prefixed with a bot name", "airplane-copilot", knownAILogins, false),
		Entry("matches a configured extra login", "my-custom-bot", []string{"my-custom-bot"}, true),
		Entry("does not match a login as a substring of an entry", "my-custom-bot-2", []string{"my-custom-bot"}, false),
		Entry("does not match when the login list is unrelated", "octocat", []string{"my-custom-bot"}, false),
		Entry("does not match when the login list is empty", "coderabbitai[bot]", nil, false),
		Entry("does not match an empty login", "", knownAILogins, false),
	)
})
