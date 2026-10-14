package marvin

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"cloud.google.com/go/errorreporting"
	"github.com/bradleyfalzon/ghinstallation/v2"
	gogithub "github.com/google/go-github/v90/github"

	"github.com/Flashgap/marvin/internal/migrations"
	"github.com/Flashgap/marvin/internal/service/github"
	"github.com/Flashgap/marvin/internal/service/jira"
	"github.com/Flashgap/marvin/internal/service/lock"
	"github.com/Flashgap/marvin/internal/service/marvin"
	"github.com/Flashgap/marvin/internal/service/reviewload"
	slacksvc "github.com/Flashgap/marvin/internal/service/slack"
	"github.com/Flashgap/marvin/internal/service/standup"
	"github.com/Flashgap/marvin/pkg/database"
	pkggithub "github.com/Flashgap/marvin/pkg/github"
	"github.com/Flashgap/marvin/pkg/googleid"
	pkgjira "github.com/Flashgap/marvin/pkg/jira"
	"github.com/Flashgap/marvin/pkg/linear"
	"github.com/Flashgap/marvin/pkg/logger"
	"github.com/Flashgap/marvin/pkg/slack"
)

type Services struct {
	errorClient    *errorreporting.Client
	DB             database.Client
	SlackService   slacksvc.Service
	GithubService  github.Service
	JiraService    jira.Service
	MarvinService  marvin.Service
	LockService    lock.Service
	ReviewLoad     reviewload.Service
	StandupService standup.Service
	// TaskTokenVerifier verifies the Google ID tokens of task requests.
	TaskTokenVerifier *googleid.Verifier
}

func (s *Services) initialize(ctx context.Context, cfg *Config) error {
	if cfg.EnableErrorReporting {
		errorClient, err := logger.NewErrorReportingService(ctx, cfg.ErrorReportingConfig())
		if err != nil {
			return fmt.Errorf("cannot init error reporting client: %w", err)
		}
		s.errorClient = errorClient
	}

	if s.DB == nil && cfg.DBEnabled() {
		dbClient, err := database.NewClient(ctx, cfg.DatabaseConfig())
		if err != nil {
			return fmt.Errorf("cannot init database client: %w", err)
		}
		s.DB = dbClient
	}

	// Migrations cover every table, whichever service uses it: apply them once,
	// before any service uses the database.
	if s.DB != nil {
		if err := s.DB.Migrate(ctx, migrations.FS); err != nil {
			return fmt.Errorf("cannot apply database migrations: %w", err)
		}
	}

	// SlackService is the single integration point for Slack. It wraps the
	// thin pkg/slack client and is shared across every service that needs to
	// talk to Slack (currently MarvinService, LockService and StandupService).
	if s.SlackService == nil {
		s.SlackService = slacksvc.NewService(slack.NewClient(cfg.SlackBotToken))
	}

	if s.LockService == nil && s.DB != nil {
		lockSvc, err := lock.NewService(s.DB, s.SlackService)
		if err != nil {
			return fmt.Errorf("cannot init lock service: %w", err)
		}
		s.LockService = lockSvc
	}

	if s.StandupService == nil && s.DB != nil && cfg.StandupEnabled() {
		standupSvc, err := standup.NewService(s.DB, s.SlackService, cfg.StandupChannelID, time.Now)
		if err != nil {
			return fmt.Errorf("cannot init standup service: %w", err)
		}
		s.StandupService = standupSvc
	}
	if s.DB == nil && cfg.StandupEnabled() {
		logger.WithContext(ctx).Warn("MARVIN_STANDUP_CHANNEL_ID is set but the database is disabled (no DB_HOST): the standup reminder is off")
	}

	if s.TaskTokenVerifier == nil {
		verifier, err := googleid.NewVerifier(ctx, http.DefaultTransport, cfg.TasksAudience, cfg.TasksServiceAccount)
		if err != nil {
			return fmt.Errorf("cannot init task token verifier: %w", err)
		}
		s.TaskTokenVerifier = verifier
	}

	if s.GithubService == nil {
		itr, err := ghinstallation.NewKeyFromFile(http.DefaultTransport, cfg.GithubAppID, cfg.GithubInstallID, cfg.GithubSecretKey)
		if err != nil {
			return fmt.Errorf("failed creating Github client: %w", err)
		}
		gogithubClient, err := gogithub.NewClient(gogithub.WithTransport(itr))
		if err != nil {
			return fmt.Errorf("failed creating Github client: %w", err)
		}
		s.GithubService = github.NewService(pkggithub.NewClient(gogithubClient))
	}

	if s.ReviewLoad == nil {
		s.ReviewLoad = reviewload.NewService(s.GithubService)
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

		repoConfigProvider := marvin.NewRepoConfigProvider(s.GithubService, cfg.Marvin)
		repoConfigProvider.Start(ctx, cfg.MarvinRepoConfigPollInterval)
		prefixCache := github.NewPrefixCache(cfg.LinearWorkspaceSlug, cfg.LinearIssuePrefixes, linearClient.Teams)
		prefixCache.Start(ctx, cfg.LinearPrefixRefreshInterval)
		prParserConfig := github.PRParserConfig{Prefixes: prefixCache}
		s.MarvinService = marvin.NewService(s.GithubService, s.JiraService, linearClient, s.SlackService, repoConfigProvider, prParserConfig)
	}

	return nil
}
