package marvin

import (
	"strings"

	"github.com/Flashgap/logrus"

	"github.com/Flashgap/marvin/pkg/utils/maputil"
)

// DefaultAIReviewerLogins lists known AI code-review bot logins recognized out of the box by
// require_ai_review and auto_changes_required, using their exact GitHub review-author login.
// RepoConfigProvider merges this with org-specific bots configured via the
// MARVIN_AI_REVIEWER_LOGINS env var (the MarvinAIReviewerLogins field on config.Marvin) and any
// repo-specific logins declared in that repo's .marvin.yaml into GitHubRepositoryConfiguration.AIReviewerLogins.
var DefaultAIReviewerLogins = []string{"coderabbitai[bot]", "graphite-app[bot]", "copilot-pull-request-reviewer[bot]"}

// DefaultAIReviewStatusContexts lists commit-status contexts that count as a completed AI review when
// no formal review is found. Some AI reviewers (e.g. CodeRabbit) skip submitting a review on trivial
// or no-op diffs but still publish a success status named after themselves. Merged with the
// MARVIN_AI_REVIEW_STATUS_CONTEXTS env var and any repo-specific contexts declared in that repo's
// .marvin.yaml into GitHubRepositoryConfiguration.AIReviewStatusContexts.
var DefaultAIReviewStatusContexts = []string{"CodeRabbit"}

// DefaultChangelogFile is the changelog path check_changelog validates against when a repository's
// .marvin.yaml doesn't override it via `check_changelog.file`.
const DefaultChangelogFile = "CHANGELOG.md"

// PathReviewRule maps a glob pattern (matched against changed file paths, doublestar syntax e.g. "go/**")
// to the GitHub team that should review matching changes.
type PathReviewRule struct {
	Pattern string
	Team    string
}

type GitHubRepositoryConfiguration struct {
	// ReviewRules is the ordered list of path->team rules declared under `reviewers.rules` in .marvin.yaml.
	// A PR requests reviewers from the union of every team whose pattern matches a changed file.
	ReviewRules []PathReviewRule
	// DefaultTeam is used when no ReviewRules pattern matches any changed file. Declared as
	// `reviewers.default_team` in .marvin.yaml.
	DefaultTeam         string
	AutoApprove         bool
	AutoChangesRequired bool
	AutoMerge           bool
	AutoReviewAssign    bool
	AutoDraftLabels     bool
	UpdateTitle         bool
	CheckTitle          bool
	CheckDescription    bool
	CheckTimeSpent      bool
	CheckLinearLink     bool
	CheckLinearProject  bool
	CheckChangelog      bool
	// ChangelogFile is the path check_changelog validates against. Defaults to DefaultChangelogFile;
	// overridable per repo via `check_changelog.file` in .marvin.yaml.
	ChangelogFile          string
	AutoAssignee           bool
	UpdateLinearLink       bool
	SlackNotify            bool
	AutoCapReport          bool
	RequireAIReview        bool
	AIReviewerLogins       []string
	AIReviewStatusContexts []string
	GithubToSlack          map[string]string
}

func withAutoApprove(c *GitHubRepositoryConfiguration) {
	c.AutoApprove = true
}

func withAutoChangesRequired(c *GitHubRepositoryConfiguration) {
	c.AutoChangesRequired = true
}

func withAutoMerge(c *GitHubRepositoryConfiguration) {
	c.AutoMerge = true
}

func withAutoReviewAssign(c *GitHubRepositoryConfiguration) {
	c.AutoReviewAssign = true
}

func withAutoDraftLabels(c *GitHubRepositoryConfiguration) {
	c.AutoDraftLabels = true
}

func withUpdateTitle(c *GitHubRepositoryConfiguration) {
	c.UpdateTitle = true
}

func withCheckTitle(c *GitHubRepositoryConfiguration) {
	c.CheckTitle = true
}

func withCheckDescription(c *GitHubRepositoryConfiguration) {
	c.CheckDescription = true
}

func withCheckTimeSpent(c *GitHubRepositoryConfiguration) {
	c.CheckTimeSpent = true
}

func withCheckLinear(c *GitHubRepositoryConfiguration) {
	c.CheckLinearLink = true
}

func withCheckLinearProject(c *GitHubRepositoryConfiguration) {
	c.CheckLinearProject = true
}

func withCheckChangelog(c *GitHubRepositoryConfiguration) {
	c.CheckChangelog = true
}

func withAutoAssignee(c *GitHubRepositoryConfiguration) {
	c.AutoAssignee = true
}

func withUpdateLinearLink(c *GitHubRepositoryConfiguration) {
	c.UpdateLinearLink = true
}

func withSlackNotify(c *GitHubRepositoryConfiguration) {
	c.SlackNotify = true
}

func withAutoCapReport(c *GitHubRepositoryConfiguration) {
	c.AutoCapReport = true
}

func withRequireAIReview(c *GitHubRepositoryConfiguration) {
	c.RequireAIReview = true
}

type optionFunc func(c *GitHubRepositoryConfiguration)

var configToFunc = map[string]optionFunc{
	"auto_approve":          withAutoApprove,
	"auto_changes_required": withAutoChangesRequired,
	"auto_merge":            withAutoMerge,
	"auto_review_assign":    withAutoReviewAssign,
	"auto_draft_labels":     withAutoDraftLabels,
	"update_title":          withUpdateTitle,
	"check_title":           withCheckTitle,
	"check_description":     withCheckDescription,
	"check_time_spent":      withCheckTimeSpent,
	"check_linear_link":     withCheckLinear,
	"check_linear_project":  withCheckLinearProject,
	"check_changelog":       withCheckChangelog,
	"update_linear_link":    withUpdateLinearLink,
	"auto_assignee":         withAutoAssignee,
	"slack_notify":          withSlackNotify,
	"auto_cap_report":       withAutoCapReport,
	"require_ai_review":     withRequireAIReview,
}

// applyFeatures turns a list of feature-name strings (as declared under `features:` in a repo's
// .marvin.yaml) into flag toggles on repoConfig, logging a critical error for any unrecognized name.
func applyFeatures(repoConfig *GitHubRepositoryConfiguration, features []string) {
	for _, featureName := range features {
		featureName = strings.TrimSpace(featureName)
		opt, ok := configToFunc[featureName]
		if !ok {
			logrus.Criticalf("unknown feature: %q. Known features are: %q", featureName, strings.Join(maputil.Keys(configToFunc), ","))
		} else {
			opt(repoConfig)
		}
	}
}
