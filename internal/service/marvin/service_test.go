package marvin_test

import (
	"errors"
	"fmt"
	"testing"

	gogithub "github.com/google/go-github/v63/github"
	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"

	"github.com/Flashgap/marvin/internal/service/github"
	mock_jira "github.com/Flashgap/marvin/internal/service/jira/mock"
	"github.com/Flashgap/marvin/internal/service/marvin"
	pkggithub "github.com/Flashgap/marvin/pkg/github"
	"github.com/Flashgap/marvin/pkg/github/githubtest"
	mock_github "github.com/Flashgap/marvin/pkg/github/mock"
	"github.com/Flashgap/marvin/pkg/linear"
	mock_linear "github.com/Flashgap/marvin/pkg/linear/mock"
	mock_slack "github.com/Flashgap/marvin/pkg/slack/mock"
	"github.com/Flashgap/marvin/pkg/utils"
)

const (
	repoName = "test-repo"
)

var testPRParserConfig = github.PRParserConfig{
	WorkspaceSlug: "your-org",
	IssuePrefixes: []string{"ENG", "APP", "BUG", "PRO"},
}

func Test(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Marvin service test suite")
}

var _ = Describe("Service tests", func() {
	var svc marvin.Service
	var mockCtrl *gomock.Controller
	var mockGithub *mock_github.MockClient
	var githubService github.Service
	var mockSlack *mock_slack.MockClient
	var mockLinear *mock_linear.MockClient
	var mockJira *mock_jira.MockService

	BeforeEach(func() {
		mockCtrl = gomock.NewController(GinkgoT())
		mockGithub = mock_github.NewMockClient(mockCtrl)
		mockSlack = mock_slack.NewMockClient(mockCtrl)
		mockLinear = mock_linear.NewMockClient(mockCtrl)
		githubService = github.NewService(mockGithub)
		mockJira = mock_jira.NewMockService(mockCtrl)
	})

	Context("Auto approve", func() {
		prNumber := 410
		branchName := utils.Ptr("some branch")
		requiredApprovingReviewCount := 2
		cfg := marvin.GitHubRepositoryConfiguration{
			AutoApprove: true,
		}
		cfgs := marvin.GitHubRepositoryConfigurations{
			repoName: &cfg,
		}

		reviewEvent := &gogithub.PullRequestReviewEvent{
			Action: utils.Ptr(pkggithub.EventPullRequestReviewActionSubmitted),
			Repo: &gogithub.Repository{
				Name: utils.Ptr(repoName),
			},
			Review: &gogithub.PullRequestReview{
				State: utils.Ptr(pkggithub.PullRequestReviewStateApproved),
			},
			PullRequest: &gogithub.PullRequest{
				Number: utils.Ptr(prNumber),
				Base: &gogithub.PullRequestBranch{
					Ref: branchName,
				},
			},
		}

		protection := &gogithub.Protection{
			RequiredPullRequestReviews: &gogithub.PullRequestReviewsEnforcement{
				RequiredApprovingReviewCount: requiredApprovingReviewCount,
			},
		}

		mxUser := &gogithub.User{Login: utils.Ptr("mx")}
		marvinUser := &gogithub.User{Login: utils.Ptr("marvin")}

		BeforeEach(func() {
			mockGithub.EXPECT().GetBranchProtection(gomock.Any(), reviewEvent, *branchName).Return(protection, nil, nil)
		})

		When("It doesn't have enough reviewers", func() {
			It("should be false", func(ctx SpecContext) {
				reviews := []*gogithub.PullRequestReview{
					{
						User:  mxUser,
						State: utils.Ptr(pkggithub.PullRequestReviewStateApproved),
					},
					{
						User:  marvinUser,
						State: utils.Ptr(pkggithub.PullRequestReviewStateApproved),
					},
					// should ignore duplicate ACK
					{
						User:  mxUser,
						State: utils.Ptr(pkggithub.PullRequestReviewStateApproved),
					},

					// it should only take the last review
					{
						User:  marvinUser,
						State: utils.Ptr(pkggithub.PullRequestReviewStateChangesRequested),
					},
				}

				mockGithub.EXPECT().ListReviews(gomock.Any(), reviewEvent, prNumber, gomock.Any()).Return(reviews, nil, nil)
				svc = marvin.NewService(githubService, mockJira, mockLinear, mockSlack, cfgs, testPRParserConfig)
				err := svc.OnPullRequestReview(ctx, reviewEvent)
				Expect(err).NotTo(HaveOccurred())
			})
		})

		When("It has enough reviewers", func() {
			It("should approve the PR", func(ctx SpecContext) {
				mockGithub.EXPECT().ListLabels(gomock.Any(), gomock.Any(), gomock.Any()).Return([]*gogithub.Label{
					{Name: utils.Ptr(github.LabelApproved)},
					{Name: utils.Ptr(github.LabelReadyForReview)},
				}, nil, nil).Times(2)
				mockGithub.EXPECT().AddPRLabels(gomock.Any(), reviewEvent, prNumber, []string{github.LabelApproved}).Times(1).Return(nil, nil, nil)
				mockGithub.EXPECT().RemovePRLabel(gomock.Any(), reviewEvent, prNumber, github.LabelReadyForReview).Times(1).Return(nil, nil)

				reviews := []*gogithub.PullRequestReview{
					{
						User:  marvinUser,
						State: utils.Ptr(pkggithub.PullRequestReviewStateChangesRequested),
					},

					{
						User:  mxUser,
						State: utils.Ptr(pkggithub.PullRequestReviewStateApproved),
					},
					// it should only take the last review
					{
						User:  marvinUser,
						State: utils.Ptr(pkggithub.PullRequestReviewStateApproved),
					},
				}

				mockGithub.EXPECT().ListReviews(gomock.Any(), reviewEvent, prNumber, gomock.Any()).Return(reviews, nil, nil)
				svc = marvin.NewService(githubService, mockJira, mockLinear, mockSlack, cfgs, testPRParserConfig)
				err := svc.OnPullRequestReview(ctx, reviewEvent)
				Expect(err).NotTo(HaveOccurred())
			})
		})
	})

	Context("Auto merge", func() {
		cfg := marvin.GitHubRepositoryConfiguration{
			AutoMerge: true,
		}
		cfgs := marvin.GitHubRepositoryConfigurations{
			repoName: &cfg,
		}

		When("User adds merge label", func() {
			mergeGHLabel := &gogithub.Label{Name: utils.Ptr(github.LabelMerge)}

			prEvent := gogithub.PullRequestEvent{
				Action: utils.Ptr("labeled"),
				Label:  mergeGHLabel,
				Repo: &gogithub.Repository{
					Name: utils.Ptr(repoName),
				},
				Sender: &gogithub.User{
					Name: utils.Ptr("mx-test"),
				},
				PullRequest: &gogithub.PullRequest{
					State:  utils.Ptr("open"),
					Number: utils.Ptr(900),
					Title:  utils.Ptr("PR to test merge with label"),
					Head: &gogithub.PullRequestBranch{
						Ref: utils.Ptr("my branch"),
						SHA: utils.Ptr("my branch"),
					},
					Labels: []*gogithub.Label{mergeGHLabel},
				},
			}

			When("PR doesn't have Marvin checks", func() {
				BeforeEach(func() {
					marvinCheck := gogithub.ListCheckRunsResults{
						CheckRuns: []*gogithub.CheckRun{
							{
								Name:       utils.Ptr(marvin.CheckName),
								Status:     utils.Ptr(pkggithub.CheckRunStatusCompleted),
								Conclusion: utils.Ptr("error"),
							},
						},
					}

					mockGithub.EXPECT().ListCheckRunsForRef(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(&marvinCheck, nil, nil)
					svc = marvin.NewService(githubService, mockJira, mockLinear, mockSlack, cfgs, testPRParserConfig)
				})

				It("should not merge and remove merge label", func(ctx SpecContext) {
					mockGithub.EXPECT().ListLabels(gomock.Any(), gomock.Any(), gomock.Any()).Return([]*gogithub.Label{mergeGHLabel}, nil, nil).Times(1)
					mockGithub.EXPECT().RemovePRLabel(gomock.Any(), gomock.Any(), gomock.Any(), github.LabelMerge).Return(nil, nil).Times(1)
					mockGithub.EXPECT().CreatePRComment(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, nil, nil).Times(1)

					prBody := githubtest.BuildPrBody(githubtest.PrData{
						TimeSpent:   "0.25",
						LinearLink:  "https://linear.app/your-org/issue/ENG-353",
						Description: "- hello world",
					})

					prEvent.PullRequest.Body = &prBody

					err := svc.OnPullRequest(ctx, &prEvent)
					Expect(err).NotTo(HaveOccurred())
				})
			})

			When("PR is ready to be merged", func() {
				BeforeEach(func() {
					marvinCheck := gogithub.ListCheckRunsResults{
						CheckRuns: []*gogithub.CheckRun{
							{
								Name:       utils.Ptr(marvin.CheckName),
								Status:     utils.Ptr(pkggithub.CheckRunStatusCompleted),
								Conclusion: utils.Ptr(pkggithub.CheckRunConclusionSuccess),
							},
						},
					}

					mockGithub.EXPECT().ListCheckRunsForRef(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(&marvinCheck, nil, nil)
					svc = marvin.NewService(githubService, mockJira, mockLinear, mockSlack, cfgs, testPRParserConfig)
				})

				It("should merge", func(ctx SpecContext) {
					prBody := githubtest.BuildPrBody(githubtest.PrData{
						TimeSpent:  "0.25",
						LinearLink: "https://linear.app/your-org/issue/ENG-353",
						Description: `
- hello world
<!--
High level summary of what this pull request does and why.

blabla
-->
`,
					})

					prBody = fmt.Sprintf("%s%s", `
<!-- If necessary, assign reviewers that know the area or changes well. Feel free to tag any additional reviewers you see fit. -->
`, prBody)

					prEvent.PullRequest.Body = &prBody
					mockGithub.EXPECT().MergePR(gomock.Any(), gomock.Any(), gomock.Any(), "- hello world", gomock.Any()).Return(nil, nil, nil)
					err := svc.OnPullRequest(ctx, &prEvent)
					Expect(err).NotTo(HaveOccurred())
				})

				It("should not merge if description is a mess", func(ctx SpecContext) {
					// Extra safety, Marvin checks are ok BUT description is a mess
					prBody := githubtest.BuildPrBody(githubtest.PrData{
						TimeSpent:   "0.25",
						LinearLink:  "https://linear.app/your-org/issue/ENG-353",
						Description: `something wrong`,
					})

					prBody = fmt.Sprintf("%s%s", `
<!-- If necessary, assign reviewers that know the area or changes well. Feel free to tag any additional reviewers you see fit. -->
`, prBody)

					prEvent.PullRequest.Body = &prBody
					mockGithub.EXPECT().ListLabels(gomock.Any(), gomock.Any(), gomock.Any()).Return([]*gogithub.Label{mergeGHLabel}, nil, nil).Times(1)
					mockGithub.EXPECT().RemovePRLabel(gomock.Any(), gomock.Any(), gomock.Any(), github.LabelMerge).Return(nil, nil).Times(1)
					mockGithub.EXPECT().CreatePRComment(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, nil, nil).Times(1)

					err := svc.OnPullRequest(ctx, &prEvent)
					Expect(err).To(HaveOccurred())
				})

				When("PR cannot be merge due to Github", func() {
					It("should not merge", func(ctx SpecContext) {
						// i.e: tests are not done
						// it should not remove the merge label in that case.
						prBody := githubtest.BuildPrBody(githubtest.PrData{
							TimeSpent:   "0.25",
							LinearLink:  "https://linear.app/your-org/issue/ENG-353",
							Description: "- hello world",
						})

						prEvent.PullRequest.Body = &prBody
						mockGithub.EXPECT().MergePR(gomock.Any(), gomock.Any(), gomock.Any(), "- hello world", gomock.Any()).Return(nil, nil, errors.New("some kind of error"))
						mockGithub.EXPECT().CreatePRComment(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any())
						err := svc.OnPullRequest(ctx, &prEvent)
						Expect(err).NotTo(HaveOccurred())
					})
				})
			})
		})
	})

	Context("Check title", func() {
		cfg := marvin.GitHubRepositoryConfiguration{
			CheckTitle: true,
		}
		cfgs := marvin.GitHubRepositoryConfigurations{
			repoName: &cfg,
		}

		var prEvent gogithub.PullRequestEvent
		BeforeEach(func() {
			prEvent = gogithub.PullRequestEvent{
				Action: utils.Ptr("opened"),
				Repo: &gogithub.Repository{
					Name: utils.Ptr(repoName),
				},
				Sender: &gogithub.User{
					Name: utils.Ptr("mx-test"),
				},
				PullRequest: &gogithub.PullRequest{
					State:  utils.Ptr("open"),
					Number: utils.Ptr(500),
					Head: &gogithub.PullRequestBranch{
						Ref: utils.Ptr("my branch"),
						SHA: utils.Ptr("my branch"),
					},
				},
			}
		})

		When("wrong title", func() {
			It("should warn", func(ctx SpecContext) {
				prBody := githubtest.BuildPrBody(githubtest.PrData{
					LinearLink: "https://linear.app/your-org/issue/ENG-583/new-authentication-middleware",
				})

				prEvent.PullRequest.Title = utils.Ptr("my title without issue id")
				prEvent.PullRequest.Body = utils.Ptr(prBody)

				mockGithub.EXPECT().CreatePRComment(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any())
				mockGithub.EXPECT().CreateCheckRun(gomock.Any(), gomock.Any(), gomock.Any())

				svc = marvin.NewService(githubService, mockJira, mockLinear, mockSlack, cfgs, testPRParserConfig)
				err := svc.OnPullRequest(ctx, &prEvent)
				Expect(err).NotTo(HaveOccurred())
			})
		})

		When("valid title", func() {
			It("should be ok", func(ctx SpecContext) {
				prBody := githubtest.BuildPrBody(githubtest.PrData{
					LinearLink: "https://linear.app/your-org/issue/ENG-583/new-authentication-middleware",
				})
				prEvent.PullRequest.Title = utils.Ptr("ENG-583: my title")
				prEvent.PullRequest.Body = utils.Ptr(prBody)

				mockGithub.EXPECT().CreateCheckRun(gomock.Any(), gomock.Any(), gogithub.CreateCheckRunOptions{
					Name:       marvin.CheckName,
					HeadSHA:    "my branch",
					Status:     utils.Ptr(pkggithub.CheckRunStatusCompleted),
					Conclusion: utils.Ptr("success"),
				})

				svc = marvin.NewService(githubService, mockJira, mockLinear, mockSlack, cfgs, testPRParserConfig)
				err := svc.OnPullRequest(ctx, &prEvent)
				Expect(err).NotTo(HaveOccurred())
			})
		})
	})

	Context("Check Linear project", func() {
		cfg := marvin.GitHubRepositoryConfiguration{
			CheckLinearProject: true,
		}
		cfgs := marvin.GitHubRepositoryConfigurations{
			repoName: &cfg,
		}

		var prEvent gogithub.PullRequestEvent
		BeforeEach(func() {
			prEvent = gogithub.PullRequestEvent{
				Action: utils.Ptr("opened"),
				Repo: &gogithub.Repository{
					Name: utils.Ptr(repoName),
				},
				PullRequest: &gogithub.PullRequest{
					State: utils.Ptr("open"),
					Title: utils.Ptr("ENG-583: my title"),
					Body: utils.Ptr(githubtest.BuildPrBody(githubtest.PrData{
						LinearLink: "https://linear.app/your-org/issue/ENG-583/new-authentication-middleware",
					})),
					Head: &gogithub.PullRequestBranch{
						Ref: utils.Ptr("my branch"),
						SHA: utils.Ptr("my branch"),
					},
				},
			}
		})

		When("missing Linear project ID", func() {
			BeforeEach(func() {
				mockLinear.EXPECT().Issue(gomock.Any(), gomock.Any()).Return(&linear.Issue{
					ProjectID: "",
				}, nil).Times(1)
			})

			It("should warn", func(ctx SpecContext) {
				mockGithub.EXPECT().CreatePRComment(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(1)
				mockGithub.EXPECT().CreateCheckRun(gomock.Any(), gomock.Any(), gomock.Any()).Times(1)

				svc = marvin.NewService(githubService, mockJira, mockLinear, mockSlack, cfgs, testPRParserConfig)
				err := svc.OnPullRequest(ctx, &prEvent)
				Expect(err).NotTo(HaveOccurred())
			})
		})

		When("Linear project set", func() {
			BeforeEach(func() {
				mockLinear.EXPECT().Issue(gomock.Any(), gomock.Any()).Return(&linear.Issue{
					ProjectID: "my-linear-project",
				}, nil).Times(1)
			})

			It("should be ok", func(ctx SpecContext) {
				mockGithub.EXPECT().CreateCheckRun(gomock.Any(), gomock.Any(), gogithub.CreateCheckRunOptions{
					Name:       marvin.CheckName,
					HeadSHA:    "my branch",
					Status:     utils.Ptr(pkggithub.CheckRunStatusCompleted),
					Conclusion: utils.Ptr("success"),
				}).Times(1)

				svc = marvin.NewService(githubService, mockJira, mockLinear, mockSlack, cfgs, testPRParserConfig)
				err := svc.OnPullRequest(ctx, &prEvent)
				Expect(err).NotTo(HaveOccurred())
			})
		})
	})

	Context("Update title", func() {
		getSvc := func(hasLinearTitle bool, hasLinearLink bool, hasLinearBranch bool) (marvin.GitHubRepositoryConfigurations, gogithub.PullRequestEvent) {
			cfg := marvin.GitHubRepositoryConfiguration{
				CheckTitle:  true,
				UpdateTitle: true,
			}
			cfgs := marvin.GitHubRepositoryConfigurations{
				repoName: &cfg,
			}

			title := "new authn middleware"
			if hasLinearTitle {
				title = "ENG-583: new authn middleware"
			}

			linearLink := ""
			if hasLinearLink {
				linearLink = "https://linear.app/your-org/issue/ENG-583/new-authentication-middleware"
			}

			branchName := "something"
			if hasLinearBranch {
				branchName = "feature/eng-583-new-authentication-middleware"
			}

			prBody := githubtest.BuildPrBody(githubtest.PrData{
				Description:    "- something",
				TimeSpent:      "1,5",
				LinearLink:     linearLink,
				ReviewersNotes: "Please read me",
				Tests:          "trust me!",
			})

			prEvent := gogithub.PullRequestEvent{
				Action: utils.Ptr("opened"),
				Repo: &gogithub.Repository{
					Name: utils.Ptr(repoName),
				},
				Sender: &gogithub.User{
					Name: utils.Ptr("mx-test"),
				},
				PullRequest: &gogithub.PullRequest{
					State:  utils.Ptr("open"),
					Title:  utils.Ptr(title),
					Body:   utils.Ptr(prBody),
					Number: utils.Ptr(500),
					Head: &gogithub.PullRequestBranch{
						Ref: utils.Ptr(branchName),
						SHA: utils.Ptr(branchName),
					},
				},
			}

			return cfgs, prEvent
		}

		When("PR title doesn't contain issue ID", func() {
			It("should get issue id from linear link", func(ctx SpecContext) {
				mockGithub.EXPECT().EditPR(
					gomock.Any(),
					gomock.Any(),
					gomock.Any(),
					&gogithub.PullRequest{
						Title: utils.Ptr("ENG-583: new authn middleware"),
					},
				).Times(1)
				// it also should put its check to ok
				mockGithub.EXPECT().CreateCheckRun(gomock.Any(), gomock.Any(), gogithub.CreateCheckRunOptions{
					Name:       marvin.CheckName,
					HeadSHA:    "feature/eng-583-new-authentication-middleware",
					Status:     utils.Ptr(pkggithub.CheckRunStatusCompleted),
					Conclusion: utils.Ptr("success"),
				})
				// it should not add any comments
				// issue ID isn't find in the title, but it's updated.

				cfgs, prEvent := getSvc(false, true, true)
				svc = marvin.NewService(githubService, mockJira, mockLinear, mockSlack, cfgs, testPRParserConfig)
				err := svc.OnPullRequest(ctx, &prEvent)
				Expect(err).ToNot(HaveOccurred())
			})

			It("should get issue id from git branch", func(ctx SpecContext) {
				mockGithub.EXPECT().EditPR(
					gomock.Any(),
					gomock.Any(),
					gomock.Any(),
					&gogithub.PullRequest{
						Title: utils.Ptr("ENG-583: new authn middleware"),
					},
				).Times(1)
				// it also should put its check to ok
				mockGithub.EXPECT().CreateCheckRun(gomock.Any(), gomock.Any(), gogithub.CreateCheckRunOptions{
					Name:       marvin.CheckName,
					HeadSHA:    "feature/eng-583-new-authentication-middleware",
					Status:     utils.Ptr(pkggithub.CheckRunStatusCompleted),
					Conclusion: utils.Ptr("success"),
				})
				// it should not add any comments
				// issue ID isn't find in the title, but it's updated.

				cfgs, prEvent := getSvc(false, true, true)
				svc = marvin.NewService(githubService, mockJira, mockLinear, mockSlack, cfgs, testPRParserConfig)
				err := svc.OnPullRequest(ctx, &prEvent)
				Expect(err).ToNot(HaveOccurred())
			})
		})

		When("PR doesn't contain linear link and git branch doesn't contain issue ID", func() {
			It("should not be able to update title", func(ctx SpecContext) {
				mockGithub.EXPECT().CreatePRComment(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any())
				mockGithub.EXPECT().CreateCheckRun(gomock.Any(), gomock.Any(), gomock.Any())

				cfgs, prEvent := getSvc(false, false, false)
				svc = marvin.NewService(githubService, mockJira, mockLinear, mockSlack, cfgs, testPRParserConfig)
				err := svc.OnPullRequest(ctx, &prEvent)
				Expect(err).ToNot(HaveOccurred())
			})
		})

		When("PR title already contains issue ID", func() {
			It("should not update the title", func(ctx SpecContext) {
				// it also should put its check to ok
				mockGithub.EXPECT().CreateCheckRun(gomock.Any(), gomock.Any(), gogithub.CreateCheckRunOptions{
					Name:       marvin.CheckName,
					HeadSHA:    "feature/eng-583-new-authentication-middleware",
					Status:     utils.Ptr(pkggithub.CheckRunStatusCompleted),
					Conclusion: utils.Ptr("success"),
				})

				cfgs, prEvent := getSvc(true, true, true)
				svc = marvin.NewService(githubService, mockJira, mockLinear, mockSlack, cfgs, testPRParserConfig)
				err := svc.OnPullRequest(ctx, &prEvent)
				Expect(err).ToNot(HaveOccurred())
			})
		})
	})

	Context("Update linear link", func() {
		cfg := marvin.GitHubRepositoryConfiguration{
			// to verify that it doesn't raise error if we can update
			CheckLinearLink:  true,
			UpdateLinearLink: true,
		}
		cfgs := marvin.GitHubRepositoryConfigurations{
			repoName: &cfg,
		}

		When("git branch contains issue ID", func() {
			It("should update linear link", func(ctx SpecContext) {
				prBody := githubtest.BuildPrBody(githubtest.PrData{})

				prEvent := gogithub.PullRequestEvent{
					Action: utils.Ptr("opened"),
					Repo: &gogithub.Repository{
						Name: utils.Ptr(repoName),
					},
					Sender: &gogithub.User{
						Name: utils.Ptr("mx-test"),
					},
					PullRequest: &gogithub.PullRequest{
						State:  utils.Ptr("open"),
						Title:  utils.Ptr("my title"),
						Body:   utils.Ptr(prBody),
						Number: utils.Ptr(500),
						Head: &gogithub.PullRequestBranch{
							Ref: utils.Ptr("feature/eng-583-new-authentication-middleware"),
							SHA: utils.Ptr("feature/eng-583-new-authentication-middleware"),
						},
					},
				}

				mockGithub.EXPECT().EditPR(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any())
				mockGithub.EXPECT().CreateCheckRun(gomock.Any(), gomock.Any(), gogithub.CreateCheckRunOptions{
					Name:       marvin.CheckName,
					HeadSHA:    "feature/eng-583-new-authentication-middleware",
					Status:     utils.Ptr(pkggithub.CheckRunStatusCompleted),
					Conclusion: utils.Ptr("success"),
				})

				svc = marvin.NewService(githubService, mockJira, mockLinear, mockSlack, cfgs, testPRParserConfig)
				err := svc.OnPullRequest(ctx, &prEvent)
				Expect(err).ToNot(HaveOccurred())
			})
		})

		When("git branch doesn't contain issue ID", func() {
			It("should not update linear link", func(ctx SpecContext) {
				prBody := githubtest.BuildPrBody(githubtest.PrData{})

				prEvent := gogithub.PullRequestEvent{
					Action: utils.Ptr("opened"),
					Repo: &gogithub.Repository{
						Name: utils.Ptr(repoName),
					},
					Sender: &gogithub.User{
						Name: utils.Ptr("mx-test"),
					},
					PullRequest: &gogithub.PullRequest{
						State:  utils.Ptr("open"),
						Title:  utils.Ptr("my title"),
						Body:   utils.Ptr(prBody),
						Number: utils.Ptr(500),
						Head: &gogithub.PullRequestBranch{
							Ref: utils.Ptr("mybranch"),
							SHA: utils.Ptr("mybranch"),
						},
					},
				}

				mockGithub.EXPECT().CreateCheckRun(gomock.Any(), gomock.Any(), gomock.Any())
				// as we can't update, comment should be created about missing linear link
				mockGithub.EXPECT().CreatePRComment(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any())

				svc = marvin.NewService(githubService, mockJira, mockLinear, mockSlack, cfgs, testPRParserConfig)
				err := svc.OnPullRequest(ctx, &prEvent)
				Expect(err).ToNot(HaveOccurred())
			})

		})
	})

	Context("HasMergeLabels", func() {
		DescribeTable("tests", func(labels []*gogithub.Label, success bool) {
			err := marvin.IsMergeable(labels)

			if success {
				Expect(err).ToNot(HaveOccurred())
			} else {
				Expect(err).To(HaveOccurred())
			}
		},
			Entry(
				"it should be ok with merge, dependencies, approved",
				[]*gogithub.Label{
					{Name: utils.Ptr(github.LabelMerge)},
					{Name: utils.Ptr(github.LabelDependencies)},
					{Name: utils.Ptr(github.LabelApproved)},
				},
				true,
			),
			Entry(
				"it should be ok with merge, hotfix",
				[]*gogithub.Label{
					{Name: utils.Ptr(github.LabelMerge)},
					{Name: utils.Ptr(github.LabelHotfix)},
				},
				true,
			),
			Entry(
				"it should be ok with merge, approved",
				[]*gogithub.Label{
					{Name: utils.Ptr(github.LabelMerge)},
					{Name: utils.Ptr(github.LabelApproved)},
				},
				true,
			),
			Entry(
				"it should be ok with merge, dependencies",
				[]*gogithub.Label{
					{Name: utils.Ptr(github.LabelMerge)},
					{Name: utils.Ptr(github.LabelDependencies)},
				},
				true,
			),
			Entry("it should be ok with merge only",
				[]*gogithub.Label{
					{Name: utils.Ptr(github.LabelMerge)},
				},
				true,
			),
			Entry("it should not be ok with merge and ready to be reviewed",
				[]*gogithub.Label{
					{Name: utils.Ptr(github.LabelMerge)},
					{Name: utils.Ptr(github.LabelReadyForReview)},
				},
				false,
			),
			Entry("it should not be ok with ready to be reviewed",
				[]*gogithub.Label{
					{Name: utils.Ptr(github.LabelReadyForReview)},
				},
				false,
			),
			Entry("it should not be ok with hotfix",
				[]*gogithub.Label{
					{Name: utils.Ptr(github.LabelHotfix)},
				},
				false,
			),
		)
	})

	Context("Notify", func() {
		cfg := marvin.GitHubRepositoryConfiguration{
			// to verify that it doesn't raise error if we can update
			SlackNotify:   true,
			GithubToSlack: map[string]string{"marvin": "U1234W5678"},
		}
		cfgs := marvin.GitHubRepositoryConfigurations{
			repoName: &cfg,
		}
		When("someone gets assigned to a PR", func() {
			It("should notify them on Slack", func(ctx SpecContext) {
				prBody := githubtest.BuildPrBody(githubtest.PrData{})

				prEvent := gogithub.PullRequestEvent{
					Action: utils.Ptr("review_requested"),
					Repo: &gogithub.Repository{
						Name: utils.Ptr(repoName),
					},
					Sender: &gogithub.User{
						Name: utils.Ptr("mx-test"),
					},
					PullRequest: &gogithub.PullRequest{
						State:  utils.Ptr("open"),
						Title:  utils.Ptr("my title"),
						Body:   utils.Ptr(prBody),
						Number: utils.Ptr(500),
						Head: &gogithub.PullRequestBranch{
							Ref: utils.Ptr("mybranch"),
							SHA: utils.Ptr("mybranch"),
						},
					},
					RequestedReviewer: &gogithub.User{
						Login: utils.Ptr("marvin"),
						Name:  utils.Ptr("marvin user"),
					},
				}

				mockSlack.EXPECT().SendMessage(gomock.Any(), "U1234W5678", gomock.Any()).Return(nil).Times(1)
				svc = marvin.NewService(githubService, mockJira, mockLinear, mockSlack, cfgs, testPRParserConfig)
				err := svc.OnPullRequest(ctx, &prEvent)
				Expect(err).ToNot(HaveOccurred())
			})
		})
	})

	Context("Cap report", func() {
		It("should call the linear API on a merged PR event", func(ctx SpecContext) {
			cfg := marvin.GitHubRepositoryConfiguration{
				AutoCapReport: true,
			}

			cfgs := marvin.GitHubRepositoryConfigurations{
				repoName: &cfg,
			}

			prBody := githubtest.BuildPrBody(githubtest.PrData{})

			prEvent := gogithub.PullRequestEvent{
				Action: utils.Ptr("closed"),
				Repo: &gogithub.Repository{
					Name: utils.Ptr(repoName),
				},
				Sender: &gogithub.User{
					Name: utils.Ptr("mx-test"),
				},
				PullRequest: &gogithub.PullRequest{
					State:  utils.Ptr("open"),
					Title:  utils.Ptr("my title"),
					Body:   utils.Ptr(prBody),
					Number: utils.Ptr(500),
					Merged: utils.Ptr(true),
					Head: &gogithub.PullRequestBranch{
						Ref: utils.Ptr("feature/eng-583-new-authentication-middleware"),
						SHA: utils.Ptr("feature/eng-583-new-authentication-middleware"),
					},
				},
			}

			mockLinear.EXPECT().Issue(gomock.Any(), gomock.Any()).Return(&linear.Issue{ProjectID: uuid.NewString()}, nil).Times(1)
			mockJira.EXPECT().DoCapReportWorkflow(gomock.Any(), gomock.Any(), gomock.Any()).Times(1)
			svc = marvin.NewService(githubService, mockJira, mockLinear, mockSlack, cfgs, testPRParserConfig)
			Expect(svc.OnPullRequest(ctx, &prEvent)).To(Succeed())
		})
	})
})
