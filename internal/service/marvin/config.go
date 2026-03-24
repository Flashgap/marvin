package marvin

import (
	"strings"

	"github.com/Flashgap/logrus"

	"github.com/Flashgap/marvin/internal/config"
	"github.com/Flashgap/marvin/pkg/utils/maputil"
)

type GitHubRepositoryConfiguration struct {
	ReviewersTeam       string
	AutoApprove         bool
	AutoChangesRequired bool
	AutoMerge           bool
	AutoReviewAssign    bool
	UpdateTitle         bool
	CheckTitle          bool
	CheckDescription    bool
	CheckTimeSpent      bool
	CheckLinearLink     bool
	CheckLinearProject  bool
	CheckChangelog      bool
	AutoAssignee        bool
	UpdateLinearLink    bool
	SlackNotify         bool
	AutoCapReport       bool
	GithubToSlack       map[string]string
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

type optionFunc func(c *GitHubRepositoryConfiguration)

var configToFunc = map[string]optionFunc{
	"auto_approve":          withAutoApprove,
	"auto_changes_required": withAutoChangesRequired,
	"auto_merge":            withAutoMerge,
	"auto_review_assign":    withAutoReviewAssign,
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
}

// GitHubRepositoryConfigurations maps the repository with its configuration to enable/disable features
type GitHubRepositoryConfigurations map[string]*GitHubRepositoryConfiguration

func GetGitHubRepositoryConfigurations(cfg config.Marvin) GitHubRepositoryConfigurations {
	configs := make(map[string]*GitHubRepositoryConfiguration, len(cfg.MarvinRepositories))
	logrus.Infof("reviewers teams: %+v", cfg.MarvinReviewersTeams)

	for repoName, featuresStr := range cfg.MarvinRepositories {
		features := strings.Split(featuresStr, ";")
		repoName = strings.TrimSpace(repoName)
		logrus.Infof("got %+v features for repository: %s", features, repoName)
		repoConfig := &GitHubRepositoryConfiguration{}
		if team, ok := cfg.MarvinReviewersTeams[repoName]; ok {
			logrus.Infof("repository %q will use team %q to find its reviewers", repoName, team)
			repoConfig.ReviewersTeam = team
		}

		for _, featureName := range features {
			opt, ok := configToFunc[featureName]
			if !ok {
				logrus.Criticalf("unknown feature: %q. Known features are: %q", featureName, strings.Join(maputil.Keys(configToFunc), ","))
			} else {
				opt(repoConfig)
			}
		}
		if repoConfig.SlackNotify {
			repoConfig.GithubToSlack = cfg.MarvinGithubToSlack
		}

		config.PrintConfig(repoConfig)
		configs[repoName] = repoConfig
	}

	return configs
}
