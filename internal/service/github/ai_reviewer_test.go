package github_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/Flashgap/marvin/internal/service/github"
)

var _ = Describe("IsAIReviewerLogin", func() {
	DescribeTable("matching against the default and extra login lists",
		func(login string, extraLogins []string, expected bool) {
			Expect(github.IsAIReviewerLogin(login, extraLogins)).To(Equal(expected))
		},
		Entry("matches coderabbitai's bot login", "coderabbitai[bot]", nil, true),
		Entry("matches graphite-app's bot login", "graphite-app[bot]", nil, true),
		Entry("matches GitHub Copilot's review login", "copilot-pull-request-reviewer[bot]", nil, true),
		Entry("matches case-insensitively", "CodeRabbitAI[bot]", nil, true),
		Entry("does not match a human login", "octocat", nil, false),
		Entry("does not match a human login that contains a bot name as substring", "clement-copilot", nil, false),
		Entry("does not match a human login prefixed with a bot name", "airplane-copilot", nil, false),
		Entry("matches a configured extra login", "my-custom-bot", []string{"my-custom-bot"}, true),
		Entry("does not match an extra login as a substring", "my-custom-bot-2", []string{"my-custom-bot"}, false),
		Entry("does not match when extra login list is unrelated", "octocat", []string{"my-custom-bot"}, false),
		Entry("does not match an empty login", "", nil, false),
	)
})
