package config

// Github configuration.
type Github struct {
	// GithubAppID is the app ID, find it: GitHub app > General
	GithubAppID int64 `envconfig:"GH_APP_ID" required:"true"`

	// GithubInstallID ID is a part of WebHook request.
	// find it: GitHub App > Advanced > Payload in Request tab
	GithubInstallID int64 `envconfig:"GH_INSTALL_ID" required:"true"`

	// GithubSecretKey is the path to the secret key used to generate GitHub JWT
	GithubSecretKey string `envconfig:"GH_SECRET_KEY" required:"true"`

	// GithubWebhookSecret holds the secret that GitHub should be sending to authenticate calls
	GithubWebhookSecret string `envconfig:"GH_WEBHOOK_SECRET" required:"true" secret:"true"`

	// GithubLabelsNamingVersion is the version of the naming convention for labels
	GithubLabelsNamingVersion int `envconfig:"GH_LABELS_NAMING_VERSION" default:"1"`
}

// Slack configuration.
type Slack struct {
	// SlackBotToken holds the secret for the Slack bot allowing services to talk as a Slack app
	SlackBotToken string `envconfig:"MARVIN_SLACK_BOT_TOKEN" required:"true" secret:"true"`
}

// Linear configuration.
type Linear struct {
	// LinearOAuthToken holds a long-living token for linear API queries
	LinearOAuthToken string `envconfig:"LINEAR_OAUTH_TOKEN" required:"true" secret:"true"`

	// LinearWorkspaceSlug is the slug of the Linear workspace, visible in issue URLs.
	// ex: https://linear.app/<workspace-slug>/issue/ENG-123
	LinearWorkspaceSlug string `envconfig:"LINEAR_WORKSPACE_SLUG" required:"true"`

	// LinearIssuePrefixes is the list of issue shorthand prefixes used by the team.
	// ex: ENG,APP,BUG
	LinearIssuePrefixes []string `envconfig:"LINEAR_ISSUE_PREFIXES" required:"true"`
}

// Marvin configuration.
type Marvin struct {
	// MarvinRepositories the list of repositories where Marvin is enabled with its features
	// ex: repo-name:auto_merge;auto_assign,another-repo:check_title
	MarvinRepositories map[string]string `envconfig:"MARVIN_REPOSITORIES" required:"true"`

	// MarvinReviewersTeams is a mapping of repository to their respective owner teams
	// ex: repo-name:team-name
	MarvinReviewersTeams map[string]string `envconfig:"MARVIN_REVIEWERS_TEAMS"`

	// MarvinGithubToSlack is a mapping of GitHub handles to Slack IDs
	// ex: octocat:U043AC1234,bob:U043BC1234
	MarvinGithubToSlack map[string]string `envconfig:"MARVIN_GITHUB_TO_SLACK"`
}

// Jira configuration.
type Jira struct {
	// JiraHost holds the target URL for Jira
	JiraHost string `envconfig:"JIRA_HOST"`
	// JiraAPIKey holds the API key for Jira Authentication
	// If you need to generate a personal token to have Marvin act on your behalf
	// refer to https://support.atlassian.com/atlassian-account/docs/manage-api-tokens-for-your-atlassian-account/.
	// You can now base64 encode a concatenation of your Jira user ID, which should be your corporate email, and the
	// token you got by following the above documentation. The resulting base64 string is your JIRA_API_KEY, to put
	// into ../../config/local/marvin.env
	JiraAPIKey string `envconfig:"JIRA_API_KEY" secret:"true"`
	// JiraFields holds mappings for custom fields names and other configurations that need to be fetched
	// ex: "projectKey:CIR,projectID:10407,epicIssueType:10413,taskIssueType:10412,transitionToDoID:11"
	// All keys needed are gettable from the following URLs :
	// <JiraHost>/rest/api/latest/project
	// * ProjectKey
	// * ProjectID
	// <JiraHost>/rest/api/latest/issuetype - prioritise IDs scoped to the target project
	// * TaskIssueTypeID
	// * EpicIssueTypeID
	// <JiraHost>/rest/api/latest/field - Look for "name": "Start Date" and get the associated customfield_XXXXX
	// * StartDateCustomFieldKey
	// <JiraHost>/rest/api/latest/issue/<AnyIsssueKey>/transitions
	// * InProgressTransitionID
	// * DoneTransitionID
	// doc: https://developer.atlassian.com/cloud/jira/platform/rest/v2/api-group-issues/#api-rest-api-2-issue-issueidorkey-transitions-get
	JiraFields map[string]string `envconfig:"JIRA_FIELDS"`
}
