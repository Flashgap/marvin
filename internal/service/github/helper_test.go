package github_test

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/Flashgap/marvin/internal/service/github"
)

var _ = Describe("Helper tests", func() {
	DescribeTable("TitleOrDescriptionWithPRNumber",
		func(commitTitle, commitDescription string, shouldAddInTitle, shouldAddInDesc bool, prNumber int) {
			prRef := fmt.Sprintf("#%d$", prNumber)
			title, desc := github.TitleOrDescriptionWithPRNumber(commitTitle, commitDescription, prNumber)
			if shouldAddInTitle {
				Expect(title).Should(MatchRegexp(prRef))
			}

			if shouldAddInDesc {
				Expect(desc).Should(MatchRegexp(prRef))
			}

			if !shouldAddInTitle && !shouldAddInDesc {
				// should remain the same
				Expect(title).To(Equal(commitTitle))
				Expect(desc).To(Equal(commitDescription))
			}
		},
		Entry("should add the PR number in the title", "ENG-489: promote incognito chat as a violation", "", true, false, 40),
		Entry("should add the PR number in the description", "ENG-489: promote incognito chat as a violation and be extra verbose so you can't add things here", "", false, true, 40),
		Entry("should not add the PR number, it's already in the title", "ENG-489: promote incognito chat as a violation #40", "", false, false, 40),
		Entry("should not add the PR number, it's already in the description", "ENG-489: promote incognito chat as a violation", "#40", false, false, 40),
	)
})
