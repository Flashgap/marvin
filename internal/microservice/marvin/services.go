package marvin

import (
	"context"
	"fmt"
	"net/http"

	"cloud.google.com/go/errorreporting"
	"github.com/bradleyfalzon/ghinstallation/v2"
	gogithub "github.com/google/go-github/v63/github"

	"github.com/Flashgap/marvin/internal/service/github"
	"github.com/Flashgap/marvin/internal/service/jira"
	"github.com/Flashgap/marvin/internal/service/marvin"
	pkggithub "github.com/Flashgap/marvin/pkg/github"
	pkgjira "github.com/Flashgap/marvin/pkg/jira"
	"github.com/Flashgap/marvin/pkg/linear"
	"github.com/Flashgap/marvin/pkg/logger"
	"github.com/Flashgap/marvin/pkg/slack"
)

type Services struct {
	errorClient   *errorreporting.Client
	GithubService github.Service
	JiraService   jira.Service
	MarvinService marvin.Service
}

func (s *Services) initialize(ctx context.Context, cfg *Config) error {
	if cfg.EnableErrorReporting {
		errorClient, err := logger.NewErrorReportingService(ctx, cfg.ErrorReportingConfig())
		if err != nil {
			return fmt.Errorf("cannot init error reporting client: %w", err)
		}
		s.errorClient = errorClient
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
		slackClient := slack.NewClient(cfg.SlackBotToken)
		linearClient := linear.NewClient(ctx, cfg.LinearOAuthToken, cfg.LinearWorkspaceSlug)

		repoConfigs := marvin.GetGitHubRepositoryConfigurations(cfg.Marvin)
		prParserConfig := github.PRParserConfig{
			WorkspaceSlug: cfg.LinearWorkspaceSlug,
			IssuePrefixes: cfg.LinearIssuePrefixes,
		}
		s.MarvinService = marvin.NewService(s.GithubService, s.JiraService, linearClient, slackClient, repoConfigs, prParserConfig)
	}

	return nil
}
