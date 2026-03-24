package marvin

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	gogithub "github.com/google/go-github/v63/github"

	"github.com/Flashgap/marvin/internal/middlewares"
	"github.com/Flashgap/marvin/internal/service/github"
	"github.com/Flashgap/marvin/internal/service/jira"
	pkggithub "github.com/Flashgap/marvin/pkg/github"
	"github.com/Flashgap/marvin/pkg/linear"
	"github.com/Flashgap/marvin/pkg/slack"
	"github.com/Flashgap/marvin/pkg/utils"
)

const (
	CheckName     = "🤖 Marvin checks"
	GitHubAppName = "marvin"
	githubCopilot = "copilot"
	changelogFile = "CHANGELOG.md"

	timeSpentToCheckAfterCommits   = 3
	timeSpentToCheckAfterAdditions = 50

	// comments on PR
	commentNotMergeable = `Hey @%s, you added the merge label but the PR is not ready to be merged yet. 
I removed this label, please add it back when the PR is ready to be merged!`
	commentNotMergeableYet        = "Hey @%s, you added the merge label but the PR is not ready to be merged yet. I'll try when all status check succeed."
	commentCheckTimeSpent         = "Hey @%s, you added the merge label but you might forgot to update the time spent. Please take a look at it."
	commentErrorsToFix            = "Hey @%s, thanks for your PR! It contains some errors to fix:\n %s"
	commentNotEnoughReviewers     = "Hey @%s, I didn't find enough reviewers for your PR. Please take a look at it!"
	commentReadyToMerge           = "Hey @%s, your PR is ready to be merged."
	commentInvalidTitle           = "Invalid PR title, should contain Linear ticket and Linear ticket should be in PR"
	commentMissingLinearURL       = "Invalid PR, should contain Linear url."
	commentMissingLinearProjectID = "Invalid PR, Linear ticket should be in a project."
	commentMissingTimeSpent       = "Invalid PR, should contain time spent."
	commentWrongDescription       = "Description should only be composed by bullet points."
	commentTitleTooLong           = "Invalid PR title, it's too long. It contains %d characters, it should be less than %d."
	commentChanglogNotUpdated     = "PR should update the CHANGELOG.md and contain a reference to this PR."

	// check runs
	checkRunTitleHotfix   = "Hotfix PR"
	checkRunSummaryHotfix = "This is a hotfix PR, Marvin's checks are bypassed."
	checkRunSummaryFailed = "Marvin checks failed."
)

type service struct {
	githubService  github.Service
	jiraService    jira.Service
	linearClient   linear.Client
	slackClient    slack.Client
	repoConfigs    GitHubRepositoryConfigurations
	prParserConfig github.PRParserConfig
}

func NewService(
	githubService github.Service,
	jiraService jira.Service,
	linearClient linear.Client,
	slackClient slack.Client,
	repoConfigs GitHubRepositoryConfigurations,
	prParserConfig github.PRParserConfig,
) Service {
	return &service{
		githubService:  githubService,
		jiraService:    jiraService,
		linearClient:   linearClient,
		slackClient:    slackClient,
		repoConfigs:    repoConfigs,
		prParserConfig: prParserConfig,
	}
}

// The Marvin Service is a GitHub webhook manager

func (s *service) OnPullRequest(ctx context.Context, event *gogithub.PullRequestEvent) error {
	config := s.repoConfigs[event.GetRepo().GetName()]
	if config == nil {
		return nil
	}

	action := event.GetAction()
	pr := event.GetPullRequest()

	log := middlewares.LoggerFromGHContext(ctx, "marvin.OnPullRequest")
	log.Infof("Got a pull request webhook with action %q", action)

	// Do nothing on closed PRs (unless it's the actual closed event) and WIPs
	if (pr.GetState() == pkggithub.PullRequestStateClosed && action != pkggithub.EventPullRequestActionClosed) ||
		github.IsWorkInProgress(pr) {
		log.Info("Ignoring event as the PR is closed or WIP")
		return nil
	}

	// Ignore Marvin generated events that are not review requests (auto assign) and closes (auto merge)
	if isMarvinEvent(event) {
		if action != pkggithub.EventPullRequestActionReviewRequested &&
			action != pkggithub.EventPullRequestActionClosed {
			log.Info("Ignoring marvin generated event")
			return nil
		}
	}

	switch action {
	case pkggithub.EventPullRequestActionOpened,
		pkggithub.EventPullRequestActionReopened,
		pkggithub.EventPullRequestActionEdited,
		pkggithub.EventPullRequestActionSynchronize:
		return s.checkAndFormatPR(ctx, event, action, pr, config, action == pkggithub.EventPullRequestActionSynchronize)
	case pkggithub.EventPullRequestActionUnlabeled:
		if pkggithub.IsLabel(event.GetLabel(), github.LabelWorkInProgress) {
			return s.checkAndFormatPR(ctx, event, action, pr, config, false)
		}
	case pkggithub.EventPullRequestActionLabeled:
		return s.labelActions(ctx, event, pr, event.GetLabel(), config)
	case pkggithub.EventPullRequestActionReviewRequested:
		return s.notifyReviewRequestBySlack(ctx, pr, event.GetRequestedReviewer().GetLogin(), config)
	case pkggithub.EventPullRequestActionClosed:
		if config.AutoCapReport && pr.GetMerged() {
			return s.reportCapitalization(ctx, pr)
		}
	default:
		log.Info("Nothing to do for this event")
	}

	return nil
}

func (s *service) OnCheckRun(ctx context.Context, event *gogithub.CheckRunEvent) error {
	config := s.repoConfigs[event.GetRepo().GetName()]
	if config == nil || !config.AutoMerge {
		return nil
	}

	action := event.GetAction()

	log := middlewares.LoggerFromGHContext(ctx, "marvin.OnCheckRun")
	log.Infof("Got a check run webhook with action %q", action)

	switch action {
	case pkggithub.EventCheckRunActionCompleted:
		for _, checkPR := range event.GetCheckRun().PullRequests {
			if checkPR == nil {
				continue
			}
			prNumber := checkPR.GetNumber()

			middlewares.AmendGHContextIdentifier(ctx, prNumber)
			log = middlewares.LoggerFromGHContext(ctx, "marvin.OnCheckRun")

			if ok, err := s.githubService.AreAllCheckRunsDone(ctx, event, prNumber); err != nil {
				return fmt.Errorf("error fetching PR #%d checks: %w", prNumber, err)
			} else if !ok {
				log.Infof("Not all checks are completed, returning")
				return nil
			}

			// Getting the PR info as is not fully populated in the check run event payload
			pr, _, err := s.githubService.PR(ctx, event, prNumber)
			if err != nil {
				return fmt.Errorf("error fetching PR #%d details: %w", prNumber, err)
			}

			if err = s.attemptMerge(ctx, event, pr, config); err != nil {
				if errors.Is(err, github.ErrMerge) {
					log.Errorf("cannot merge PR, cancel merge: %v", err)
					// Cannot merge, remove the label so user can retry
					return s.cancelMerge(ctx, event, prNumber)
				}
				return fmt.Errorf("error during merge: %w", err)
			}
		}
	default:
		log.Info("Nothing to do for this event")
	}

	return nil
}

func (s *service) OnPullRequestReview(ctx context.Context, event *gogithub.PullRequestReviewEvent) error {
	config := s.repoConfigs[event.GetRepo().GetName()]
	if config == nil {
		return nil
	}

	action := event.GetAction()
	pr := event.GetPullRequest()

	log := middlewares.LoggerFromGHContext(ctx, "marvin.OnPullRequestReview")
	log.Infof("Got a pull request review webhook with action %q and state %q", action, event.GetReview().GetState())

	switch action {
	case pkggithub.EventPullRequestReviewActionSubmitted:
		switch {
		case strings.EqualFold(event.GetReview().GetState(), pkggithub.PullRequestReviewStateChangesRequested):
			if config.AutoChangesRequired {
				var errs error
				if err := s.notifyChangesRequestedBySlack(ctx, event.GetSender().GetLogin(), pr, pr.GetUser().GetLogin(), config); err != nil {
					errs = errors.Join(errs, err)
				}
				if err := s.githubService.AddLabel(ctx, event, pr.GetNumber(), github.LabelChangesRequired); err != nil {
					errs = errors.Join(errs, err)
				}

				return errs
			}
		case strings.EqualFold(event.GetReview().GetState(), pkggithub.PullRequestReviewStateApproved):
			if config.AutoApprove || config.AutoMerge {
				ok, err := s.githubService.HasEnoughApprovals(ctx, event, pr)
				if err != nil {
					return fmt.Errorf("error checking for approval: %w", err)
				}
				if !ok {
					return nil
				}

				if config.AutoApprove {
					if err = s.githubService.AddLabel(ctx, event, pr.GetNumber(), github.LabelApproved); err != nil {
						return fmt.Errorf("error adding approved label: %w", err)
					}

					if err = s.githubService.RemoveLabel(ctx, event, pr.GetNumber(), github.LabelReadyForReview); err != nil {
						return fmt.Errorf("error removing ready for review label: %w", err)
					}
				}

				return s.attemptMerge(ctx, event, pr, config)
			}
		default:
			log.Info("Nothing to do for this event")
		}
	default:
		log.Info("Nothing to do for this event")
	}

	return nil
}

func (s *service) checkAndFormatPR(ctx context.Context, webhook pkggithub.RepoSenderGetter, action string, pr *gogithub.PullRequest, config *GitHubRepositoryConfiguration, silent bool) error {
	log := middlewares.LoggerFromGHContext(ctx, "marvin.checkAndFormatPR")

	if pkggithub.IsLabelInList(pr.Labels, github.LabelHotfix) {
		log.Info("Skipping checks for hotfix PR")
		return s.githubService.AddCheckRun(ctx, webhook, CheckName, pr.GetHead().GetSHA(), true, &gogithub.CheckRunOutput{
			Title:   utils.Ptr(checkRunTitleHotfix),
			Summary: utils.Ptr(checkRunSummaryHotfix),
		})
	}

	parsedPRContent, err := github.ParsePRContents(pr.GetTitle(), pr.GetBody(), pr.GetHead().GetRef(), s.prParserConfig)
	var comments []string

	if config.CheckLinearLink && errors.Is(err, github.ErrLinearLink) {
		if !config.UpdateLinearLink || (config.UpdateLinearLink && !parsedPRContent.AddLinearLink) {
			comments = append(comments, commentMissingLinearURL)
			log.Warnf("error: %v", github.ErrLinearLink)
		}
	}

	if config.CheckTimeSpent && errors.Is(err, github.ErrTimeSpent) {
		comments = append(comments, commentMissingTimeSpent)
		log.Warnf("error: %v", github.ErrTimeSpent)
	}

	if config.CheckDescription && errors.Is(err, github.ErrInvalidDescription) {
		comments = append(comments, commentWrongDescription)
		log.Warnf("error: %v", github.ErrInvalidDescription)
	}

	if config.CheckTitle && errors.Is(err, github.ErrIssueNotFoundInTitle) {
		// if it's not found and Marvin doesn't add the issue ID, then it's an issue
		if !config.UpdateTitle || (config.UpdateTitle && !parsedPRContent.AddTitleIssueID) {
			comments = append(comments, commentInvalidTitle)
			log.Warnf("error: %v", github.ErrIssueNotFoundInTitle)
		}
	}

	if config.AutoMerge {
		// as the title is used as a commit message, we check the length of it
		titleToCheck := []rune(parsedPRContent.Title)
		if config.UpdateTitle {
			titleToCheck = []rune(parsedPRContent.CleanedTitle)
		}

		if len(titleToCheck) >= github.CommitTitleSizeLimit {
			comments = append(comments, fmt.Sprintf(commentTitleTooLong, len(titleToCheck), github.CommitTitleSizeLimit))
			log.Warn("error: title too long")
		}
	}

	if config.CheckChangelog && action == pkggithub.EventPullRequestActionSynchronize {
		if ok, err := s.hasChangelogBeenUpdated(ctx, webhook, pr.GetNumber()); err != nil {
			return fmt.Errorf("error checking if backlog is updated: %w", err)
		} else if !ok {
			// change log not updated, it's an error
			comments = append(comments, commentChanglogNotUpdated)
			log.Warn("error: changelog not updated")
		}
	}

	log.Infof("Parsed contents: %+v\nError: %v\nComments: %+v", parsedPRContent, err, comments)

	// Take actions
	var errs error
	if config.AutoAssignee && pr.GetAssignee() == nil {
		senderLogin := utils.SafeVal(webhook.GetSender().Login)
		log.Infof("Assigning %s to this PR", senderLogin)

		_, _, err = s.githubService.AddAssignees(
			ctx,
			webhook,
			pr.GetNumber(),
			[]string{senderLogin})
		if err != nil {
			errs = errors.Join(errs, fmt.Errorf("error adding assignee: %w", err))
		}
	}

	if config.UpdateTitle && parsedPRContent.AddTitleIssueID {
		log.Infof("Updating PR title from: %s to %s", parsedPRContent.Title, parsedPRContent.CleanedTitle)
		if _, _, err = s.githubService.EditPR(ctx, webhook, pr.GetNumber(), &gogithub.PullRequest{
			Title: utils.Ptr(parsedPRContent.CleanedTitle),
		}); err != nil {
			errs = errors.Join(errs, fmt.Errorf("error editing PR title: %w", err))
		}
	}

	if config.UpdateLinearLink && parsedPRContent.AddLinearLink {
		log.Infof("Updating PR body to add linear link")
		if _, _, err = s.githubService.EditPR(ctx, webhook, pr.GetNumber(), &gogithub.PullRequest{
			Body: utils.Ptr(parsedPRContent.CleanedBody),
		}); err != nil {
			errs = errors.Join(errs, fmt.Errorf("error editing PR body: %w", err))
		}
	}

	linearIssueID := parsedPRContent.LinearLinkIssueID
	if linearIssueID == "" {
		linearIssueID = parsedPRContent.BranchIssueID
	}

	// Check for missing Linear project ID. It can be from the linear link or the branch name:
	if config.CheckLinearProject && linearIssueID != "" {
		issue, err := s.linearClient.Issue(ctx, linearIssueID)
		if err != nil {
			return fmt.Errorf("error querying linear issue: %w", err)
		}
		log.Infof("Found Linear issue %s with project ID %q", issue.ID, issue.ProjectID)

		if issue.ProjectID == "" {
			comments = append(comments, commentMissingLinearProjectID)
			log.Warn("error: missing linear project ID")
		}
	}

	// Create check run with results
	var crOutput *gogithub.CheckRunOutput
	var crSuccess bool

	if len(comments) > 0 {
		var commentBuilder strings.Builder
		for _, comment := range comments {
			_, _ = fmt.Fprintf(&commentBuilder, "- %s\n", comment)
		}
		comment := commentBuilder.String()
		comment = fmt.Sprintf(commentErrorsToFix, webhook.GetSender().GetLogin(), comment)

		if !silent {
			if _, _, err = s.githubService.CreatePRComment(ctx, webhook, pr.GetNumber(), &gogithub.IssueComment{
				Body: utils.Ptr(comment),
			}); err != nil {
				errs = errors.Join(errs, err)
			}
		}

		crOutput = &gogithub.CheckRunOutput{
			Title:   utils.Ptr(CheckName),
			Summary: utils.Ptr(checkRunSummaryFailed),
			Text:    utils.Ptr(comment),
		}
		crSuccess = false
		log.Warn("PR has errors, check run will be marked as failed")
	} else {
		log.Info("No errors found, PR is ready to be merged")
		crSuccess = true
	}

	if err = s.githubService.AddCheckRun(ctx, webhook, CheckName, pr.GetHead().GetSHA(), crSuccess, crOutput); err != nil {
		errs = errors.Join(errs, err)
		log.Errorf("error adding check run: %v", err)
	} else {
		log.Info("check run added successfully")
	}

	return errs
}

func (s *service) labelActions(ctx context.Context, webhook pkggithub.RepoSenderGetter, pr *gogithub.PullRequest, addedLabel *gogithub.Label, config *GitHubRepositoryConfiguration) error {
	var err error

	log := middlewares.LoggerFromGHContext(ctx, "marvin.labelActions")
	log.Infof("Performing actions associated with label %q", addedLabel.GetName())

	switch {
	case pkggithub.IsLabel(addedLabel, github.LabelReadyForReview):
		if config.AutoReviewAssign {
			success, err := s.githubService.FindAndAssignReviewers(ctx, webhook, pr, config.ReviewersTeam)
			if err != nil {
				return err
			}
			if !success {
				log.Info("didn't get enough reviewers to request")
				if _, _, err := s.githubService.CreatePRComment(ctx, webhook, pr.GetNumber(), &gogithub.IssueComment{
					Body: utils.Ptr(
						fmt.Sprintf(commentNotEnoughReviewers, webhook.GetSender().GetLogin()),
					),
				}); err != nil {
					return fmt.Errorf("cannot add comment to ask for reviewers: %w", err)
				}
			}
		}
	case pkggithub.IsLabel(addedLabel, github.LabelMerge):
		if err = s.attemptMerge(ctx, webhook, pr, config); err != nil {
			if errors.Is(err, github.ErrMerge) {
				return s.delayMerge(ctx, webhook, pr.GetNumber())
			}
			return err
		}
	default:
		log.Info("Nothing to do for this label")
	}

	return nil
}

func (s *service) attemptMerge(ctx context.Context, webhook pkggithub.RepoSenderGetter, pr *gogithub.PullRequest, config *GitHubRepositoryConfiguration) error {
	log := middlewares.LoggerFromGHContext(ctx, "marvin.attemptMerge")

	if !config.AutoMerge {
		log.Info("Auto merge is disabled")
		return nil
	}

	if err := IsMergeable(pr.Labels); err != nil {
		log.Infof("PR is not mergeable: %v", err)

		return nil
	}

	// Non-critical reminder for the dev to update their time logged
	if config.CheckTimeSpent && pkggithub.IsLabelInList(pr.Labels, github.LabelMerge) {
		log.Info("Checking if developper should update their time logged on the PR ")
		isTimeSpentOutdated, err := s.isTimeSpentOutdated(ctx, webhook, pr)
		if err != nil {
			log.Errorf("error checking for time spent: %v", err)
		}

		if isTimeSpentOutdated {
			log.Infof("time spent outdated, ask the user to update it")
			if _, _, err = s.githubService.CreatePRComment(ctx, webhook, pr.GetNumber(), &gogithub.IssueComment{
				Body: utils.Ptr(
					fmt.Sprintf(commentCheckTimeSpent, utils.SafeVal(webhook.GetSender().Login)),
				),
			}); err != nil {
				log.Errorf("error creating PR comment: %v", err)
			}
		}
	}

	var err error
	defer func() {
		if err != nil && !errors.Is(err, github.ErrMerge) {
			log.Warnf("got an error preventing merge, removing merge label: %v", err)
			if rmError := s.cancelMerge(ctx, webhook, pr.GetNumber(), "Unexpected error."); rmError != nil {
				log.Errorf("error removing merge label: %v", rmError)
			}
		}
	}()

	// Marvin's checks are not performed on hotfixes
	if !pkggithub.IsLabelInList(pr.Labels, github.LabelHotfix) {
		log.Info("Checking for Marvin's check run success before merging")
		var ok bool
		ok, err = s.githubService.HasCheckRunSucceeded(ctx, webhook, pr.GetNumber(), CheckName)
		if err != nil {
			err = fmt.Errorf("error checking for Marvin's approval: %w", err)
			return err
		}

		if !ok {
			return s.cancelMerge(ctx, webhook, pr.GetNumber(), "Marvin's checks isn't ok.")
		}
	}

	log.Infof("Merging the PR")
	// Purposefully assigning to err so our deferred function can catch it
	err = s.githubService.UpdateAndMergePR(ctx, webhook, pr)
	return err
}

func (s *service) cancelMerge(
	ctx context.Context,
	webhook pkggithub.RepoSenderGetter,
	prNumber int,
	description ...string,
) error {
	if err := s.githubService.RemoveLabel(ctx, webhook, prNumber, github.LabelMerge); err != nil {
		return fmt.Errorf("error removing merge label: %w", err)
	}

	desc := strings.Join(description, ".")
	comment := commentNotMergeable + ".\n" + desc
	if _, _, err := s.githubService.CreatePRComment(ctx, webhook, prNumber, &gogithub.IssueComment{
		Body: utils.Ptr(
			fmt.Sprintf(comment, webhook.GetSender().GetLogin()),
		),
	}); err != nil {
		return fmt.Errorf("error adding comment on merge abort: %w", err)
	}

	return nil
}

func (s *service) delayMerge(ctx context.Context, webhook pkggithub.RepoSenderGetter, prNumber int) error {
	if _, _, err := s.githubService.CreatePRComment(ctx, webhook, prNumber, &gogithub.IssueComment{
		Body: utils.Ptr(
			fmt.Sprintf(commentNotMergeableYet, webhook.GetSender().GetLogin()),
		),
	}); err != nil {
		return fmt.Errorf("error adding comment on merge delay: %w", err)
	}

	return nil
}

func (s *service) hasChangelogBeenUpdated(ctx context.Context, webhook pkggithub.RepoSenderGetter, prNumber int) (bool, error) {
	log := middlewares.LoggerFromGHContext(ctx, "marvin.hasChangelogBeenUpdated")

	var changelogUpdated bool
	err := pkggithub.ConsumePaginatedResource(100, func(opts *gogithub.ListOptions) (*gogithub.Response, bool, error) {
		files, res, err := s.githubService.ListPRFiles(ctx, webhook, prNumber, opts)
		if err != nil {
			return res, false, fmt.Errorf("cannot list pr files: %w", err)
		}

		for _, file := range files {
			if file.GetFilename() != changelogFile {
				continue
			}

			if file.GetStatus() == "added" || file.GetStatus() == "modified" {
				changes := github.ExtractPatchModifications(*file.Patch)
				if strings.Contains(changes, fmt.Sprintf("#%d", prNumber)) {
					changelogUpdated = true
					return res, false, nil
				}
			}

			// CHANGELOG doesn't mention this PR, stop checking for other files
			return res, false, nil
		}

		// CHANGELOG not found, continue to consume PR updated files
		return res, true, nil
	})
	if err != nil {
		return false, err
	}

	if changelogUpdated {
		log.Info("Change log has been updated")
	} else {
		log.Info("Change log has not been updated")
	}

	return changelogUpdated, nil
}

func (s *service) isTimeSpentOutdated(ctx context.Context, webhook pkggithub.RepoSenderGetter, pr *gogithub.PullRequest) (bool, error) {
	log := middlewares.LoggerFromGHContext(ctx, "marvin.isTimeSpentOutdated")

	commits, _, err := s.githubService.ListPRCommits(ctx, webhook, pr.GetNumber(), nil)
	if err != nil {
		return false, fmt.Errorf("cannot list PR commits: %w", err)
	}
	log.Infof("%d commits found for this PR", len(commits))

	var prTime time.Time
	if !pr.GetUpdatedAt().IsZero() {
		prTime = pr.GetUpdatedAt().Time
	} else {
		log.Info("PR not updated, taking creation time")
		prTime = pr.CreatedAt.Time
	}

	log.Infof("PR last edited at %s", prTime)

	commitsAfterPR := 0
	totalAdditions := 0
	for _, commit := range commits {
		commitDate := commit.GetCommit().GetAuthor().GetDate().Time
		log.Infof("commit %s date: %s", commit.GetCommit().GetSHA(), commitDate)
		if commitDate.UTC().After(prTime.UTC()) {
			log.Info("commit done after last PR edit")
			commitDetails, _, err := s.githubService.GetCommit(
				ctx,
				webhook,
				commit.GetSHA(),
				nil)
			if err != nil {
				log.Warnf("cannot get details for commit %s: %v", commit.GetCommit().GetMessage(), err)
			} else {
				additions := commitDetails.GetStats().GetAdditions()
				totalAdditions += additions
			}

			commitsAfterPR++
		} else {
			log.Info("commit done before last PR edit")
		}
	}

	log.Infof("commits after PR last edit: %d and total additions: %d", commitsAfterPR, totalAdditions)
	if commitsAfterPR > timeSpentToCheckAfterCommits || totalAdditions > timeSpentToCheckAfterAdditions {
		return true, nil
	}

	return false, nil
}

func (s *service) notifyReviewRequestBySlack(ctx context.Context, pr *gogithub.PullRequest, ghLogin string, config *GitHubRepositoryConfiguration) error {
	if ghLogin != "" && ghLogin != githubCopilot {
		// in config, keys are in lowercase.
		ghLogin = strings.ToLower(ghLogin)
		slackID, ok := config.GithubToSlack[ghLogin]
		if !ok {
			return fmt.Errorf("no slack ID mapping for github user %q", ghLogin)
		}

		msg := fmt.Sprintf("You've been assigned to PR #%d\n%s\n%s", pr.GetNumber(), pr.GetTitle(), pr.GetHTMLURL())

		if err := s.slackClient.SendMessage(ctx, slackID, msg); err != nil {
			return fmt.Errorf("error sending slack message: %w", err)
		}
	}

	return nil
}

func (s *service) notifyChangesRequestedBySlack(ctx context.Context, senderLogin string, pr *gogithub.PullRequest, ghLogin string, config *GitHubRepositoryConfiguration) error {
	if ghLogin != "" {
		ghLogin = strings.ToLower(ghLogin)
		slackID, ok := config.GithubToSlack[ghLogin]
		if !ok {
			return fmt.Errorf("no slack ID mapping for github user %q", ghLogin)
		}

		msg := fmt.Sprintf("%s requested changes to your PR #%d\n%s\n%s", senderLogin, pr.GetNumber(), pr.GetTitle(), pr.GetHTMLURL())

		if err := s.slackClient.SendMessage(ctx, slackID, msg); err != nil {
			return fmt.Errorf("error sending slack message: %w", err)
		}
	}

	return nil
}

func (s *service) reportCapitalization(ctx context.Context, pr *gogithub.PullRequest) error {
	log := middlewares.LoggerFromGHContext(ctx, "marvin.reportCapitalization")

	// We're ignoring errors here as the PR is not supposed to have been merged if this didn't succeed, see checkAndFormatPR
	parsedPRContent, _ := github.ParsePRContents(pr.GetTitle(), pr.GetBody(), pr.GetHead().GetRef(), s.prParserConfig)

	var linearIssueID string
	switch {
	case parsedPRContent.LinearLinkIssueID != "":
		linearIssueID = parsedPRContent.LinearLinkIssueID
	case parsedPRContent.TitleIssue != "":
		linearIssueID = parsedPRContent.TitleIssue
	case parsedPRContent.BranchIssueID != "":
		linearIssueID = parsedPRContent.BranchIssueID
	default:
		return fmt.Errorf("could not find linear issue ID in PR")
	}

	issue, err := s.linearClient.Issue(ctx, linearIssueID)
	if err != nil {
		return fmt.Errorf("error querying linear for project ID: %w", err)
	}

	if issue == nil || issue.ProjectID == "" {
		log.Infof("Issue %q is not linked to a project. Aborting cap report automation", linearIssueID)
		return nil
	}

	return s.jiraService.DoCapReportWorkflow(ctx, issueLinearToJira(issue), parsedPRContent.TimeSpent)
}
