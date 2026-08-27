package marvin

import (
	"context"
	"fmt"
	"net/http"
	"slices"
	"sync"
	"time"

	"github.com/Flashgap/logrus"
	gogithub "github.com/google/go-github/v63/github"
	"gopkg.in/yaml.v3"

	"github.com/Flashgap/marvin/internal/config"
	pkggithub "github.com/Flashgap/marvin/pkg/github"
)

// RepoConfigFileName is the name of the per-repository Marvin configuration file, committed at
// the root of each monitored repository's default branch.
const RepoConfigFileName = ".marvin.yaml"

// repoConfigFile is the YAML schema of a repository's .marvin.yaml file.
type repoConfigFile struct {
	Features       []string              `yaml:"features"`
	Reviewers      *reviewersConfig      `yaml:"reviewers"`
	CheckChangelog *checkChangelogConfig `yaml:"check_changelog"`
	AIReview       *aiReviewConfig       `yaml:"ai_review"`
}

type checkChangelogConfig struct {
	File string `yaml:"file"`
}

type reviewersConfig struct {
	DefaultTeam string             `yaml:"default_team"`
	Rules       []reviewRuleConfig `yaml:"rules"`
}

type reviewRuleConfig struct {
	Path string `yaml:"path"`
	Team string `yaml:"team"`
}

type aiReviewConfig struct {
	ReviewerLogins []string `yaml:"reviewer_logins"`
	StatusContexts []string `yaml:"status_contexts"`
}

// RepoConfigProvider resolves a repository's Marvin configuration from the .marvin.yaml file
// committed on that repository's default branch, so that repo owners can self-serve their own
// Marvin settings (including which team reviews which subtree) via a normal PR to their own repo.
type RepoConfigProvider interface {
	// Get returns the last-polled configuration for the repository targeted by webhook, or nil if
	// the repository has no .marvin.yaml. A nil configuration means Marvin is disabled for that
	// repository: callers must treat it exactly like the "repository not configured" case did
	// before .marvin.yaml existed, i.e. no-op rather than error. Get only ever reads from an
	// in-memory cache maintained by Start: it never calls the GitHub API itself, so it's safe to
	// call from the webhook request path without risking a slow response or a GitHub outage
	// breaking webhook delivery.
	//
	// If the last poll of this repository failed (invalid YAML, or a GitHub API error), Get returns
	// the last known-good configuration alongside a non-nil ConfigWarning describing why, so callers
	// can surface it (e.g. as a PR comment) instead of silently disabling Marvin on a typo or a
	// transient GitHub outage. If no known-good configuration exists yet, Get returns a nil
	// configuration alongside the warning.
	Get(webhook pkggithub.RepoSenderGetter) (*GitHubRepositoryConfiguration, *ConfigWarning)

	// Start performs an initial poll of every repository accessible to the GitHub App installation,
	// populating the cache Get reads from, then repeats the poll every interval until ctx is
	// cancelled. If interval is zero, only the initial poll runs. Start blocks until the initial
	// poll completes.
	Start(ctx context.Context, interval time.Duration)
}

// ConfigWarning describes a non-fatal problem encountered while polling a repository's
// .marvin.yaml. It is not a fatal error: RepoConfigProvider.Get always pairs it with a usable
// (possibly nil) configuration.
type ConfigWarning struct {
	// Message is a human-readable explanation of what went wrong, suitable for logging or posting
	// as a PR comment.
	Message string
	// UsedFallback is true when Message describes why the returned configuration is a previously
	// cached one rather than freshly polled.
	UsedFallback bool
}

// StaticRepoConfigProvider is a RepoConfigProvider backed by a fixed, pre-resolved map of
// repository name to configuration, with no polling or caching. Useful for tests and for any
// caller that already has repo configs resolved by other means such as legacy config.
type StaticRepoConfigProvider map[string]*GitHubRepositoryConfiguration

func (p StaticRepoConfigProvider) Get(webhook pkggithub.RepoSenderGetter) (*GitHubRepositoryConfiguration, *ConfigWarning) {
	return p[webhook.GetRepo().GetName()], nil
}

func (p StaticRepoConfigProvider) Start(_ context.Context, _ time.Duration) {}

type repoConfigCacheEntry struct {
	config  *GitHubRepositoryConfiguration
	warning *ConfigWarning
}

type repoConfigProvider struct {
	githubClient pkggithub.Client
	orgConfig    config.Marvin

	mu    sync.RWMutex
	cache map[string]repoConfigCacheEntry
}

// NewRepoConfigProvider builds a RepoConfigProvider backed by the given GitHub client. Call Start
// to begin polling; until then, Get returns a nil configuration for every repository.
func NewRepoConfigProvider(githubClient pkggithub.Client, orgConfig config.Marvin) RepoConfigProvider {
	return &repoConfigProvider{
		githubClient: githubClient,
		orgConfig:    orgConfig,
		cache:        make(map[string]repoConfigCacheEntry),
	}
}

func (p *repoConfigProvider) Get(webhook pkggithub.RepoSenderGetter) (*GitHubRepositoryConfiguration, *ConfigWarning) {
	p.mu.RLock()
	entry := p.cache[webhook.GetRepo().GetFullName()]
	p.mu.RUnlock()

	return entry.config, entry.warning
}

func (p *repoConfigProvider) Start(ctx context.Context, interval time.Duration) {
	p.poll(ctx)

	if interval <= 0 {
		return
	}

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				logrus.Info("stopping repo config poll loop")
				return
			case <-ticker.C:
				p.poll(ctx)
			}
		}
	}()
}

// poll refreshes every repository accessible to the GitHub App installation. A repository whose
// refresh fails keeps its last known-good cached configuration (see pollRepo); a failure to even
// list the installed repositories leaves the whole cache untouched until the next tick.
func (p *repoConfigProvider) poll(ctx context.Context) {
	repos, err := p.listInstalledRepos(ctx)
	if err != nil {
		logrus.Errorf("repo config poll: failed to list installed repositories, keeping previous configurations: %v", err)
		return
	}

	for _, repo := range repos {
		p.pollRepo(ctx, repo)
	}

	logrus.Infof("repo config poll: refreshed %d repositories", len(repos))
}

func (p *repoConfigProvider) listInstalledRepos(ctx context.Context) ([]*gogithub.Repository, error) {
	var all []*gogithub.Repository
	opts := &gogithub.ListOptions{PerPage: pkggithub.MaxPerPage}

	for {
		result, res, err := p.githubClient.ListInstalledRepos(ctx, opts)
		if err != nil {
			return nil, err
		}

		all = append(all, result.Repositories...)

		if res.NextPage == 0 {
			return all, nil
		}
		opts.Page = res.NextPage
	}
}

// installedRepoWebhook adapts a *gogithub.Repository from the installation repo listing into a
// pkggithub.RepoSenderGetter, so the poller can reuse fetch exactly as webhook-triggered lookups
// do. It carries no sender since polling isn't attributable to any GitHub user.
type installedRepoWebhook struct {
	repo *gogithub.Repository
}

func (w installedRepoWebhook) GetRepo() *gogithub.Repository { return w.repo }
func (w installedRepoWebhook) GetSender() *gogithub.User     { return nil }

// pollRepo refreshes a single repository's cached configuration. On failure it keeps the
// previously cached configuration (if any) and attaches a ConfigWarning describing why, so a
// broken or unreachable .marvin.yaml doesn't silently disable Marvin.
func (p *repoConfigProvider) pollRepo(ctx context.Context, repo *gogithub.Repository) {
	webhook := installedRepoWebhook{repo: repo}
	repoKey := repo.GetFullName()

	repoConfig, err := p.fetch(ctx, webhook)

	if err != nil {
		p.mu.Lock()
		prev, hadPrev := p.cache[repoKey]

		var warning *ConfigWarning
		if hadPrev {
			warning = &ConfigWarning{
				Message:      fmt.Sprintf("failed to refresh %s (%v), using the last known-good configuration", RepoConfigFileName, err),
				UsedFallback: true,
			}
			repoConfig = prev.config
		} else {
			warning = &ConfigWarning{
				Message: fmt.Sprintf("failed to load %s (%v), Marvin is disabled for this repository", RepoConfigFileName, err),
			}
		}

		p.cache[repoKey] = repoConfigCacheEntry{config: repoConfig, warning: warning}
		p.mu.Unlock()

		logrus.Warnf("%s: %s", repoKey, warning.Message)
		return
	}

	if repoConfig == nil {
		// Genuinely no .marvin.yaml. Fall back to the repository's legacy config:
		if fallback := legacyFallbackConfig(p.orgConfig, repo.GetName()); fallback != nil {
			logrus.Infof("no %s yet for %s, running off legacy config pending migration", RepoConfigFileName, repoKey)
			repoConfig = fallback
		} else {
			logrus.Infof("Marvin disabled for %s: no %s and no legacy entry", repoKey, RepoConfigFileName)
		}
		p.attemptConfigMigration(ctx, webhook)
	}

	p.mu.Lock()
	p.cache[repoKey] = repoConfigCacheEntry{config: repoConfig}
	p.mu.Unlock()
}

// fetch always reads .marvin.yaml off the repository's default branch, never off the webhook's PR
// head ref: fetching from the PR branch would let a PR author edit their own review rules (e.g.
// drop the reviewer requirement) inside the very PR being reviewed.
func (p *repoConfigProvider) fetch(ctx context.Context, webhook pkggithub.RepoSenderGetter) (*GitHubRepositoryConfiguration, error) {
	defaultBranch := webhook.GetRepo().GetDefaultBranch()

	content, res, err := p.githubClient.GetFileContent(ctx, webhook, RepoConfigFileName, defaultBranch)
	if err != nil {
		if res != nil && res.StatusCode == http.StatusNotFound {
			logrus.Infof("no %s found in %s", RepoConfigFileName, webhook.GetRepo().GetFullName())
			return nil, nil
		}
		return nil, fmt.Errorf("error fetching %s from %s: %w", RepoConfigFileName, webhook.GetRepo().GetFullName(), err)
	}

	repoConfig, err := parseRepoConfig(content, p.orgConfig)
	if err != nil {
		return nil, fmt.Errorf("invalid %s in %s: %w", RepoConfigFileName, webhook.GetRepo().GetFullName(), err)
	}

	return repoConfig, nil
}

// parseRepoConfig builds a GitHubRepositoryConfiguration from raw .marvin.yaml content.
func parseRepoConfig(content string, orgConfig config.Marvin) (*GitHubRepositoryConfiguration, error) {
	var file repoConfigFile
	if err := yaml.Unmarshal([]byte(content), &file); err != nil {
		return nil, err
	}

	repoConfig := &GitHubRepositoryConfiguration{}
	applyFeatures(repoConfig, file.Features)

	repoConfig.ChangelogFile = DefaultChangelogFile
	if file.CheckChangelog != nil && file.CheckChangelog.File != "" {
		repoConfig.ChangelogFile = file.CheckChangelog.File
	}

	if file.Reviewers != nil {
		repoConfig.DefaultTeam = file.Reviewers.DefaultTeam
		for _, rule := range file.Reviewers.Rules {
			repoConfig.ReviewRules = append(repoConfig.ReviewRules, PathReviewRule{Pattern: rule.Path, Team: rule.Team})
		}
	}

	if repoConfig.SlackNotify {
		repoConfig.GithubToSlack = orgConfig.MarvinGithubToSlack
	}

	var repoAIReviewerLogins, repoAIReviewStatusContexts []string
	if file.AIReview != nil {
		repoAIReviewerLogins = file.AIReview.ReviewerLogins
		repoAIReviewStatusContexts = file.AIReview.StatusContexts
	}

	if repoConfig.RequireAIReview || repoConfig.AutoChangesRequired {
		repoConfig.AIReviewerLogins = slices.Concat(DefaultAIReviewerLogins, orgConfig.MarvinAIReviewerLogins, repoAIReviewerLogins)
	}
	if repoConfig.RequireAIReview {
		repoConfig.AIReviewStatusContexts = slices.Concat(DefaultAIReviewStatusContexts, orgConfig.MarvinAIReviewStatusContexts, repoAIReviewStatusContexts)
	}

	config.PrintConfig(repoConfig)

	return repoConfig, nil
}
