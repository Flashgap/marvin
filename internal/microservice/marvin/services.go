package marvin

import (
	"context"
	"fmt"
	"net/http"

	"cloud.google.com/go/errorreporting"
	"github.com/bradleyfalzon/ghinstallation/v2"
	gogithub "github.com/google/go-github/v63/github"

	"github.com/Flashgap/marvin/internal/migrations"
	"github.com/Flashgap/marvin/internal/service/github"
	"github.com/Flashgap/marvin/internal/service/jira"
	"github.com/Flashgap/marvin/internal/service/lock"
	"github.com/Flashgap/marvin/internal/service/marvin"
	slacksvc "github.com/Flashgap/marvin/internal/service/slack"
	"github.com/Flashgap/marvin/pkg/database"
	pkggithub "github.com/Flashgap/marvin/pkg/github"
	pkgjira "github.com/Flashgap/marvin/pkg/jira"
	"github.com/Flashgap/marvin/pkg/linear"
	"github.com/Flashgap/marvin/pkg/logger"
	"github.com/Flashgap/marvin/pkg/slack"
)

type Services struct {
	errorClient   *errorreporting.Client
	DB            database.Client
	SlackService  slacksvc.Service
	GithubService github.Service
	JiraService   jira.Service
	MarvinService marvin.Service
	LockService   lock.Service
}

func (s *Services) initialize(ctx context.Context, cfg *Config) error {
	if cfg.EnableErrorReporting {
		errorClient, err := logger.NewErrorReportingService(ctx, cfg.ErrorReportingConfig())
		if err != nil {
			return fmt.Errorf("cannot init error reporting client: %w", err)
		}
		s.errorClient = errorClient
	}

	if s.DB == nil && cfg.Enabled() {
		dbClient, err := database.NewClient(ctx, cfg.DatabaseConfig())
		if err != nil {
			return fmt.Errorf("cannot init database client: %w", err)
		}
		s.DB = dbClient
	}

	// One Slack client shared across services. The thin pkg/slack client is
	// wrapped once in the higher-level slack service so future commands have a
	// single integration point.
	slackClient := slack.NewClient(cfg.SlackBotToken)
	if s.SlackService == nil {
		s.SlackService = slacksvc.NewService(slackClient)
	}

	if s.LockService == nil && s.DB != nil {
		lockSvc, err := lock.NewService(ctx, s.DB, s.SlackService, migrations.FS)
		if err != nil {
			return fmt.Errorf("cannot init lock service: %w", err)
		}
		s.LockService = lockSvc
	}

	if s.GithubService == nil {
		itr, err := ghinstallation.NewKeyFromFile(http.DefaultTransport, cfg.GithubAppID, cfg.GithubInstallID, cfg.GithubSecretKey)
		if err != nil {
			return fmt.Errorf("failed creating Github client: %w", err)
		}
		ghClient := pkggithub.NewClient(gogithub.NewClient(&http.Client{Transport: itr}))
		s.GithubService = github.NewService(ghClient)
	}

	if s.JiraService == nil {
		jiraClient, err := pkgjira.NewClient(cfg.JiraHost, cfg.JiraAPIKey)
		if err != nil {
			return fmt.Errorf("error creating Jira client: %w", err)
		}

		s.JiraService, err = jira.NewService(&cfg.Jira, jiraClient)
		if err != nil {
			return fmt.Errorf("error creating Jira service: %w", err)
		}
	}

	if s.MarvinService == nil {
		linearClient := linear.NewClient(ctx, cfg.LinearOAuthToken, cfg.LinearWorkspaceSlug)

		if cfg.LinearPrefixRefreshInterval <= 0 && len(cfg.LinearIssuePrefixes) == 0 {
			return fmt.Errorf("either LINEAR_ISSUE_PREFIXES or LINEAR_PREFIX_REFRESH_INTERVAL must be set")
		}

		repoConfigs := marvin.GetGitHubRepositoryConfigurations(cfg.Marvin)
		prefixCache := github.NewPrefixCache(cfg.LinearWorkspaceSlug, cfg.LinearIssuePrefixes, linearClient.Teams)
		prefixCache.Start(ctx, cfg.LinearPrefixRefreshInterval)
		prParserConfig := github.PRParserConfig{Prefixes: prefixCache}
		s.MarvinService = marvin.NewService(s.GithubService, s.JiraService, linearClient, slackClient, repoConfigs, prParserConfig)
	}

	return nil
}
