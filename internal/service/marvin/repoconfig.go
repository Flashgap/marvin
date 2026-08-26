package marvin

import (
	"context"
	"fmt"
	"net/http"
	"slices"
	"sync"
	"time"

	"github.com/Flashgap/logrus"
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
	// Get returns the resolved configuration for the repository targeted by webhook, or nil if the
	// repository has no .marvin.yaml. A nil configuration means Marvin is disabled for that
	// repository: callers must treat it exactly like the "repository not configured" case did
	// before .marvin.yaml existed, i.e. no-op rather than error.
	//
	// If refreshing the config failed (invalid YAML, or a GitHub API error), Get falls back to the
	// last known-good configuration and returns a non-nil ConfigWarning describing why, so callers
	// can surface it (e.g. as a PR comment) instead of silently disabling Marvin on a typo or a
	// transient GitHub outage. If no known-good configuration exists yet, Get returns a nil
	// configuration alongside the warning.
	Get(ctx context.Context, webhook pkggithub.RepoSenderGetter) (*GitHubRepositoryConfiguration, *ConfigWarning, error)
}

// ConfigWarning describes a non-fatal problem encountered while refreshing a repository's
// .marvin.yaml. It is not a fatal error: RepoConfigProvider.Get always pairs it with a usable
// (possibly nil) configuration.
type ConfigWarning struct {
	// Message is a human-readable explanation of what went wrong, suitable for logging or posting
	// as a PR comment.
	Message string
	// UsedFallback is true when Message describes why the returned configuration is a previously
	// cached one rather than freshly fetched.
	UsedFallback bool
}

// StaticRepoConfigProvider is a RepoConfigProvider backed by a fixed, pre-resolved map of
// repository name to configuration, with no fetching or caching. Useful for tests and for any
// caller that already has repo configs resolved by other means.
type StaticRepoConfigProvider map[string]*GitHubRepositoryConfiguration

func (p StaticRepoConfigProvider) Get(_ context.Context, webhook pkggithub.RepoSenderGetter) (*GitHubRepositoryConfiguration, *ConfigWarning, error) {
	return p[webhook.GetRepo().GetName()], nil, nil
}

type repoConfigCacheEntry struct {
	config    *GitHubRepositoryConfiguration
	warning   *ConfigWarning
	fetchedAt time.Time
}

type repoConfigProvider struct {
	githubClient pkggithub.Client
	orgConfig    config.Marvin
	ttl          time.Duration

	mu    sync.Mutex
	cache map[string]repoConfigCacheEntry
}

// NewRepoConfigProvider builds a RepoConfigProvider backed by the given GitHub client and cached
// for orgConfig.MarvinRepoConfigCacheTTL between fetches of a given repository's .marvin.yaml.
func NewRepoConfigProvider(githubClient pkggithub.Client, orgConfig config.Marvin) RepoConfigProvider {
	ttl := orgConfig.MarvinRepoConfigCacheTTL
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}

	return &repoConfigProvider{
		githubClient: githubClient,
		orgConfig:    orgConfig,
		ttl:          ttl,
		cache:        make(map[string]repoConfigCacheEntry),
	}
}

func (p *repoConfigProvider) Get(ctx context.Context, webhook pkggithub.RepoSenderGetter) (*GitHubRepositoryConfiguration, *ConfigWarning, error) {
	repoKey := webhook.GetRepo().GetFullName()

	p.mu.Lock()
	entry, ok := p.cache[repoKey]
	p.mu.Unlock()
	if ok && time.Since(entry.fetchedAt) < p.ttl {
		return entry.config, entry.warning, nil
	}

	repoConfig, err := p.fetch(ctx, webhook)
	if err != nil {
		var warning *ConfigWarning
		if ok {
			repoConfig = entry.config
			warning = &ConfigWarning{
				Message:      fmt.Sprintf("failed to refresh %s (%v), using the last known-good configuration", RepoConfigFileName, err),
				UsedFallback: true,
			}
		} else {
			warning = &ConfigWarning{
				Message: fmt.Sprintf("failed to load %s (%v), Marvin is disabled for this repository", RepoConfigFileName, err),
			}
		}

		p.mu.Lock()
		p.cache[repoKey] = repoConfigCacheEntry{config: repoConfig, warning: warning, fetchedAt: time.Now()}
		p.mu.Unlock()

		return repoConfig, warning, nil
	}

	p.mu.Lock()
	p.cache[repoKey] = repoConfigCacheEntry{config: repoConfig, fetchedAt: time.Now()}
	p.mu.Unlock()

	return repoConfig, nil, nil
}

// fetch always reads .marvin.yaml off the repository's default branch, never off the webhook's PR
// head ref: fetching from the PR branch would let a PR author edit their own review rules (e.g.
// drop the reviewer requirement) inside the very PR being reviewed.
func (p *repoConfigProvider) fetch(ctx context.Context, webhook pkggithub.RepoSenderGetter) (*GitHubRepositoryConfiguration, error) {
	defaultBranch := webhook.GetRepo().GetDefaultBranch()

	content, res, err := p.githubClient.GetFileContent(ctx, webhook, RepoConfigFileName, defaultBranch)
	if err != nil {
		if res != nil && res.StatusCode == http.StatusNotFound {
			logrus.Infof("no %s found in %s, Marvin is disabled for this repository", RepoConfigFileName, webhook.GetRepo().GetFullName())
			return nil, nil
		}
		return nil, fmt.Errorf("error fetching %s from %s: %w", RepoConfigFileName, webhook.GetRepo().GetFullName(), err)
	}

	var file repoConfigFile
	if err := yaml.Unmarshal([]byte(content), &file); err != nil {
		return nil, fmt.Errorf("invalid %s in %s: %w", RepoConfigFileName, webhook.GetRepo().GetFullName(), err)
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
		repoConfig.GithubToSlack = p.orgConfig.MarvinGithubToSlack
	}

	var repoAIReviewerLogins, repoAIReviewStatusContexts []string
	if file.AIReview != nil {
		repoAIReviewerLogins = file.AIReview.ReviewerLogins
		repoAIReviewStatusContexts = file.AIReview.StatusContexts
	}

	if repoConfig.RequireAIReview || repoConfig.AutoChangesRequired {
		repoConfig.AIReviewerLogins = slices.Concat(DefaultAIReviewerLogins, p.orgConfig.MarvinAIReviewerLogins, repoAIReviewerLogins)
	}
	if repoConfig.RequireAIReview {
		repoConfig.AIReviewStatusContexts = slices.Concat(DefaultAIReviewStatusContexts, p.orgConfig.MarvinAIReviewStatusContexts, repoAIReviewStatusContexts)
	}

	config.PrintConfig(repoConfig)

	return repoConfig, nil
}
