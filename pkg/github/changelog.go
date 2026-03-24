package github

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/Flashgap/logrus"
	"github.com/google/go-github/v63/github"

	"github.com/Flashgap/marvin/pkg/utils"
)

const (
	changelogStartChar = "-"
	deployLabelPrefix  = "deploy:"
)

var (
	githubPRNumberRegex = regexp.MustCompile(`#\d*`)
)

type ChangelogRow struct {
	ShortDescription string
	LongDescription  string
	URL              string // Can be a pull request URL or a commit URL.
}

type Changelog struct {
	repoOwner   string
	repoName    string
	startCommit string
	stopCommit  string
	Rows        []ChangelogRow
	IsRollback  bool
}

func (c Changelog) String() string {
	var str strings.Builder
	for _, row := range c.Rows {
		_, _ = str.WriteString(changelogStartChar)
		_, _ = str.WriteString(" ")
		_, _ = str.WriteString(row.ShortDescription)
		_, _ = str.WriteString("\n")
	}
	return str.String()
}

func (c Changelog) GithubCompareURL() string {
	return fmt.Sprintf("https://github.com/%s/%s/compare/%s...%s", c.repoOwner, c.repoName, c.startCommit, c.stopCommit)
}

// GenerateChangelog generates a changelog for the given service between two commits.
// It uses the GitHub API to retrieve the commits and extract the PR number from the commit message.
// It relies on specific labels to determine if a PR is related to the service.
// The label naming convention is: deploy:{service-name}.
func GenerateChangelog(
	ctx context.Context,
	client *github.Client,
	repoOwner, repoName, serviceName, startCommit, stopCommit string,
) (*Changelog, error) {
	log := logrus.WithContext(ctx).WithPrefix("[github.generateChangelog]")

	cmp, resp, err := client.Repositories.CompareCommits(ctx, repoOwner, repoName, startCommit, stopCommit, &github.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to compare commits: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to compare commits: %v", resp.Status)
	}

	changelog := Changelog{
		repoOwner:   repoOwner,
		repoName:    repoName,
		startCommit: startCommit,
		stopCommit:  stopCommit,
	}

	if len(cmp.Commits) == 0 { // This is maybe a rollback.
		// Compare commits in the other way:
		cmp, resp, err = client.Repositories.CompareCommits(ctx, repoOwner, repoName, stopCommit, startCommit, &github.ListOptions{})
		if err != nil {
			return nil, fmt.Errorf("failed to compare commits: %w", err)
		}
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("failed to compare commits: %v", resp.Status)
		}

		if len(cmp.Commits) == 0 { // There is no diff.
			return &changelog, nil
		}

		changelog.IsRollback = true
	}

	for _, commit := range cmp.Commits {
		fullMessage := *commit.Commit.Message

		// Extract PR number from commit message:
		prNumbers := githubPRNumberRegex.FindStringSubmatch(fullMessage)

		var pullRequest *github.PullRequest // Can be nil if PR is not found or no PR associated to commit.

		if len(prNumbers) == 0 || len(prNumbers) > 1 {
			// No PR number found or multiple PR numbers found: We log a message but we don't stop the process and include
			// the commit in the changelog.
			log.Warnf("Cannot extract PR number from commit: %v", *commit.SHA)
		} else {
			// Parse number to int:
			prNumber, err := strconv.ParseInt(strings.Trim(prNumbers[0], "#"), 10, 64)
			if err != nil {
				return nil, fmt.Errorf("failed to parse PR number: %w", err)
			}

			pullRequest, resp, err = client.PullRequests.Get(ctx, repoOwner, repoName, int(prNumber))
			if err != nil {
				return nil, fmt.Errorf("failed to get pr: %w", err)
			}
			if resp.StatusCode != http.StatusOK {
				return nil, fmt.Errorf("failed to get pr: %v", resp.Status)
			}

			// Check if PR is related to the service:
			isServiceConcerned := false
			hasDeployLabel := false
			for _, label := range pullRequest.Labels {
				if strings.HasPrefix(*label.Name, deployLabelPrefix) {
					hasDeployLabel = true

					if strings.Contains(*label.Name, serviceName) {
						isServiceConcerned = true
						break
					}
				}
			}

			if !isServiceConcerned && hasDeployLabel {
				continue // Skip commit from changelog.
			}
		}

		shortDesc := strings.Split(fullMessage, "\n")[0]
		longDesc := strings.Join(strings.Split(fullMessage, "\n")[1:], "\n")

		changelog.Rows = append(changelog.Rows, ChangelogRow{
			ShortDescription: shortDesc,
			LongDescription:  longDesc,
			URL:              utils.SafeVal(commit.HTMLURL),
		})

		if pullRequest != nil {
			changelog.Rows[len(changelog.Rows)-1].URL = utils.SafeVal(pullRequest.HTMLURL)
		}
	}

	return &changelog, nil
}
