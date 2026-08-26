package config

import "time"

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
}

// Slack configuration.
type Slack struct {
	// SlackBotToken holds the secret for the Slack bot allowing services to talk as a Slack app
	SlackBotToken string `envconfig:"MARVIN_SLACK_BOT_TOKEN" required:"true" secret:"true"`

	// SlackSigningSecret authenticates inbound slash-command requests using
	// the X-Slack-Signature HMAC scheme. Required to enable the /lock command;
	// when empty (and not in dev), inbound requests are rejected.
	SlackSigningSecret string `envconfig:"MARVIN_SLACK_SIGNING_SECRET" secret:"true"`
}

// Linear configuration.
type Linear struct {
	// LinearOAuthToken holds a long-living token for linear API queries
	LinearOAuthToken string `envconfig:"LINEAR_OAUTH_TOKEN" required:"true" secret:"true"`

	// LinearWorkspaceSlug is the slug of the Linear workspace, visible in issue URLs.
	// ex: https://linear.app/<workspace-slug>/issue/ENG-123
	LinearWorkspaceSlug string `envconfig:"LINEAR_WORKSPACE_SLUG" required:"true"`

	// LinearIssuePrefixes is the seed/fallback list of issue shorthand prefixes.
	// If LinearPrefixRefreshInterval is unset, this list is used as-is and never refreshed.
	// If LinearPrefixRefreshInterval is set, this list seeds the cache until the first successful
	// fetch and is also used as a fallback if Linear becomes unreachable. It can be left empty in
	// that mode if you're fine with a brief startup window where no prefixes are known.
	// ex: ENG,APP,BUG
	LinearIssuePrefixes []string `envconfig:"LINEAR_ISSUE_PREFIXES"`

	// LinearPrefixRefreshInterval, when set, enables periodic refresh of the team prefix list
	// from Linear. When unset (zero), the static LinearIssuePrefixes list is used.
	// ex: 1h, 30m
	LinearPrefixRefreshInterval time.Duration `envconfig:"LINEAR_PREFIX_REFRESH_INTERVAL"`
}

// Marvin configuration.
type Marvin struct {
	// Deprecated: MarvinLegacyRepositories only seeds the one-time .marvin.yaml migration PR (see
	// internal/service/marvin/migration.go) that Marvin opens for a repository it previously had
	// an entry for here. It no longer has any effect on Marvin's actual per-repository behavior,
	// which is driven entirely by each repository's own .marvin.yaml. Safe to unset once every
	// repository has been migrated.
	// ex: repo-name:auto_merge;auto_assign,other-repo:check_title
	MarvinLegacyRepositories map[string]string `envconfig:"MARVIN_REPOSITORIES"`

	// Deprecated: migration-only, see MarvinLegacyRepositories.
	// ex: repo-name:team-name
	MarvinLegacyReviewersTeams map[string]string `envconfig:"MARVIN_REVIEWERS_TEAMS"`

	// MarvinRepoConfigCacheTTL controls how long a repository's .marvin.yaml file is cached
	// before being re-fetched from its default branch (defaults to 5m).
	MarvinRepoConfigCacheTTL time.Duration `envconfig:"MARVIN_REPO_CONFIG_CACHE_TTL" default:"5m"`

	// MarvinGithubToSlack is a mapping of GitHub handles to Slack IDs
	// ex: octocat:U043AC1234,bob:U043BC1234
	MarvinGithubToSlack map[string]string `envconfig:"MARVIN_GITHUB_TO_SLACK"`

	// MarvinAIReviewerLogins extends the built-in list of AI code-review bot logins
	// (coderabbitai[bot], graphite-app[bot], copilot-pull-request-reviewer[bot]) recognized by
	// require_ai_review. Matches are an exact, case-insensitive match against the review author's
	// GitHub login.
	// ex: my-custom-ai-bot[bot]
	MarvinAIReviewerLogins []string `envconfig:"MARVIN_AI_REVIEWER_LOGINS"`

	// MarvinAIReviewStatusContexts extends the built-in list of commit-status contexts (CodeRabbit)
	// that count as a completed AI review when no formal review is found on the PR. Used by
	// require_ai_review. Matches are an exact, case-insensitive match against the status context.
	// ex: MyAIReviewer
	MarvinAIReviewStatusContexts []string `envconfig:"MARVIN_AI_REVIEW_STATUS_CONTEXTS"`
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
