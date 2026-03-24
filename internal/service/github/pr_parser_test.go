package github_test

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/Flashgap/marvin/internal/service/github"
	"github.com/Flashgap/marvin/pkg/github/githubtest"
)

var testPRParserConfig = github.PRParserConfig{
	WorkspaceSlug: "your-org",
	IssuePrefixes: []string{"ENG", "APP", "BUG", "PRO"},
}

var _ = Describe("PR Parser tests", func() {
	Context("AddLinearLink", func() {
		DescribeTable("should work", func(prBody string, branch string, expectedIssueID string) {
			newBody, err := github.AddLinearLink(prBody, branch)
			Expect(err).NotTo(HaveOccurred())
			linearLink, err := github.ExtractFixedIssue(newBody, testPRParserConfig)
			Expect(err).NotTo(HaveOccurred(), "it should be able to find the linear link after added it. Body:\n%s", newBody)
			Expect(linearLink).To(Equal(expectedIssueID))
		},
			Entry("testing for blabla",
				githubtest.BuildPrBody(githubtest.PrData{
					Description: "- some blabla",
					TimeSpent:   "2 hours",
				}),
				"https://linear.app/your-org/issue/ENG-445",
				"ENG-445",
			))
	})
	Context("LinearLinkFromBranch", func() {
		DescribeTable("should work", func(branch string, expectedLink string, expectedIssue string) {
			link, issueID, err := github.LinearLinkFromBranch(branch, testPRParserConfig)
			Expect(err).NotTo(HaveOccurred())
			Expect(link).To(Equal(expectedLink))
			Expect(issueID).To(Equal(expectedIssue))
		},
			Entry(
				"should work for eng",
				"feature/eng-445-implementation",
				"https://linear.app/your-org/issue/ENG-445",
				"ENG-445"),
			Entry("should work for app",
				"feature/pro-777-implementation",
				"https://linear.app/your-org/issue/PRO-777",
				"PRO-777"),
			Entry(
				"should work for fix",
				"fix/eng-445-implementation",
				"https://linear.app/your-org/issue/ENG-445",
				"ENG-445"),
		)
	})
	Context("RemoveHTMLComments", func() {
		It("should remove html comments", func() {
			body := `
Hi, how are you?
<!-- inline comment -->
<!-- inline
with multi line there

-->
Pretty good.`
			output := github.RemoveHTMLComments(body)
			Expect(output).To(Equal("\nHi, how are you?\n\n\nPretty good."))
		})
	})

	Context("Clean title", func() {
		DescribeTable("table",
			func(input string, expected string) {
				output := github.CleanTitle(input, testPRParserConfig)
				Expect(output).To(Equal(expected))
			},
			Entry("should remove the Feature/eng xx", "ENG-489: Feature/eng 489 promote incognito chat as a violation", "ENG-489: promote incognito chat as a violation"),
			Entry("should remove the feature/eng xx", "ENG-489: feature/eng 489 promote incognito chat as a violation", "ENG-489: promote incognito chat as a violation"),
			Entry("should remove the feature/pro xx", "PRO-1076: Feature/pro 1076 fix rniap types", "PRO-1076: fix rniap types"),
			Entry("should remove the fix/eng xxx", "Fix/eng 445 discountsjourneys experiment implementation", "discountsjourneys experiment implementation"),
		)
	})

	Context("Extract issue from title", func() {
		DescribeTable("table",
			func(input string, expected string, expectedErr error) {
				title, err := github.ExtractIssueFromTitle(input, testPRParserConfig)
				if expectedErr != nil {
					Expect(err).To(MatchError(expectedErr))
				} else {
					Expect(err).NotTo(HaveOccurred())
					Expect(title).To(Equal(expected))
				}
			},
			Entry("should find the issue with uppercase", "ENG-563: Build a webhook auth middleware", "ENG-563", nil),
			Entry("should find the issue with lowercase", "eng-563: Build a webhook auth middleware", "eng-563", nil),
			Entry("should not find the issue", "Build a webhook auth middleware", "eng-563", github.ErrIssueNotFoundInTitle),
			Entry("should not find the issue if it's not at the beginning", "Build ENG-300 a webhook auth middleware", "", github.ErrIssueNotFoundInTitle),
		)
	})

	Context("Extract time spent", func() {
		DescribeTable("Table",
			func(input string, expected float64, isError bool) {
				output, err := github.ExtractTimeSpent(input)
				if !isError {
					Expect(err).NotTo(HaveOccurred())
					Expect(output).To(Equal(expected))
				} else {
					Expect(err).To(HaveOccurred())
				}
			},
			Entry("it should work with comma", "## Time spent\n 1.5 hours ##", 1.5, false),
			Entry("it should work with dot", "## Time spent\n 2,5 hours ##", 2.5, false),
			Entry("it should work with integer", "## Time spent\n 1 hour ##", 1.0, false),
			Entry("it should work without section afterwards", "## Time spent\n 1 hour blabla", 1.0, false),
			Entry("it should returns an error without time spent", "## Time spent\n xx hour ##", 0.0, true),
		)
	})

	Context("Extract fixed issue", func() {
		DescribeTable("Table",
			func(input string, url string) {
				output, err := github.ExtractFixedIssue(input, testPRParserConfig)
				Expect(err).NotTo(HaveOccurred())
				Expect(output).To(Equal(url))
			},
			Entry(
				"should work with markdown url",
				"## Fixed issues\n [LINEAR_LINK](https://linear.app/your-org/issue/PRO-557/implement-new-pollen-screen-design) ## Time spend",
				"PRO-557",
			),
			Entry(
				"should work with url as string",
				"## Fixed issues\n https://linear.app/your-org/issue/PRO-557/implement-new-pollen-screen-design ## Time spend",
				"PRO-557",
			),
			Entry(
				"should work only with issue ID",
				"## Fixed issues\n https://linear.app/your-org/issue/PRO-557 ## Time spend",
				"PRO-557",
			),
		)
	})

	Context("ExtractDescription", func() {
		DescribeTable("Table",
			func(input string, expected string) {
				output, err := github.ExtractDescription(input)
				Expect(err).NotTo(HaveOccurred(), fmt.Sprintf("wrong description: %s", input))
				Expect(output).To(Equal(expected))
			},
			Entry("it should extract", `
				## Description
- Something to say
				## Linear link
			`, `- Something to say`),
			Entry("it should extract and keep bullet points", `
				##Description
- point 1
- point 2
- point 3

## Linear Link
			`, `- point 1
- point 2
- point 3`),
		)
	})

	Context("ValidateDescription", func() {
		DescribeTable("It",
			func(input string, isValid bool) {
				Expect(github.ValidateDescription(input)).To(Equal(isValid))
			},
			Entry("should not work with string only", "only string blabla", false),
			Entry("should not work with bullet points and chars", `
				- some description
				and others
			`, false),
			Entry("should work with only bullet points on multi line using -", `
				- point 1
				- point 2
				- point 3

			`, true),
			Entry("should work with only bullet points on multi line using *", `
				* point 1
				* point 2
				* point 3
			`, true),
			Entry("should work with only bullet points on multi line using +", `
				+ point 1
				+ point 2
				+ point 3
			`, true),
		)
	})

	Context("RemoveMarkdownCheckboxes", func() {
		It("should remove checkboxes", func() {
			input := `
This is a list with checkboxes:
- [x] Item 1
- [ ] Item 2
- [x] Item 3
- [X] Item 4
			`
			expectedOutput := `
This is a list with checkboxes:
- Item 1
- Item 2
- Item 3
- Item 4
			`
			output := github.RemoveMarkdownCheckboxes(input)
			Expect(output).To(Equal(expectedOutput))
		})
	})

	Context("ParsePRContents", func() {
		When("PR title doesn't contain issue ID", func() {
			When("PR body contains linear link", func() {
				It("should set the cleaned title with the issue ID from linear", func() {
					// add some garbage to the title
					// to be sure it cleans the title + updates it
					branch := "some-name"
					title := "feature/eng 605 my title"
					pr := githubtest.PrData{
						LinearLink:  "https://linear.app/your-org/issue/ENG-605/automatically-replace-linear-link",
						Description: "- hello world",
						TimeSpent:   "1.5",
					}
					prBody := githubtest.BuildPrBody(pr)

					info, err := github.ParsePRContents(title, prBody, branch, testPRParserConfig)
					Expect(info.AddTitleIssueID).To(BeTrue())
					Expect(err).To(MatchError(github.ErrIssueNotFoundInTitle))
					Expect(info.CleanedTitle).To(Equal("ENG-605: my title"))
				})
			})

			When("PR body doesn't contain linear link", func() {
				It("should get issue id from git branch and update title + body", func() {
					// add some garbage to the title
					// to be sure it cleans the title + updates it
					title := "feature/eng 605 my title"
					branch := "feature/eng-605-automatically-replace-linear_link"

					pr := githubtest.PrData{
						Description: "- hello world",
						TimeSpent:   "1.5",
					}
					prBody := githubtest.BuildPrBody(pr)

					info, err := github.ParsePRContents(title, prBody, branch, testPRParserConfig)
					// Expect(err).NotTo(HaveOccurred())
					Expect(err).To(MatchError(github.ErrIssueNotFoundInTitle))
					Expect(err).To(MatchError(github.ErrLinearLink))

					Expect(info.AddLinearLink).To(BeTrue())
					Expect(info.AddTitleIssueID).To(BeTrue())

					Expect(info.CleanedTitle).To(Equal("ENG-605: my title"))
					linearIssueID, err := github.ExtractFixedIssue(info.CleanedBody, testPRParserConfig)
					Expect(err).NotTo(HaveOccurred())
					Expect(linearIssueID).To(Equal("ENG-605"))
				})
			})
		})
	})
})
