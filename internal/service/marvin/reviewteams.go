package marvin

import (
	"context"
	"fmt"

	"github.com/bmatcuk/doublestar/v4"
	gogithub "github.com/google/go-github/v90/github"

	pkggithub "github.com/Flashgap/marvin/pkg/github"
)

// resolveReviewTeams returns the deduped union of every team whose ReviewRules pattern matches at
// least one file changed in the PR (doublestar syntax, e.g. "go/**"), falling back to
// config.DefaultTeam when no rule with a team matches any changed file. Returns an empty slice when
// there is nothing to resolve (no rules and no default team configured).
func (s *service) resolveReviewTeams(ctx context.Context, webhook pkggithub.RepoSenderGetter, prNumber int, config *GitHubRepositoryConfiguration) ([]string, error) {
	return s.resolveFromRules(ctx, webhook, prNumber, config, func(rule PathReviewRule) string { return rule.Team }, config.DefaultTeam)
}

// resolvePathLabels returns the deduped union of every label whose ReviewRules pattern matches at
// least one file changed in the PR, falling back to config.DefaultLabel when no rule with a label
// matches any changed file.
func (s *service) resolvePathLabels(ctx context.Context, webhook pkggithub.RepoSenderGetter, prNumber int, config *GitHubRepositoryConfiguration) ([]string, error) {
	return s.resolveFromRules(ctx, webhook, prNumber, config, func(rule PathReviewRule) string { return rule.Label }, config.DefaultLabel)
}

// resolveFromRules matches config.ReviewRules against the files changed in the PR and returns the
// deduped, non-empty values picked by field from the matched rules, or fallback when none is found.
func (s *service) resolveFromRules(ctx context.Context, webhook pkggithub.RepoSenderGetter, prNumber int, config *GitHubRepositoryConfiguration, field func(PathReviewRule) string, fallback string) ([]string, error) {
	var rules []PathReviewRule
	for _, rule := range config.ReviewRules {
		if field(rule) != "" {
			rules = append(rules, rule)
		}
	}
	if len(rules) == 0 {
		return valueOrNone(fallback), nil
	}

	seen := make(map[string]struct{})
	var values []string
	err := pkggithub.ConsumePaginatedResource(pkggithub.MaxPerPage, func(opts *gogithub.ListOptions) (*gogithub.Response, bool, error) {
		files, res, err := s.githubService.ListPRFiles(ctx, webhook, prNumber, opts)
		if err != nil {
			return res, false, fmt.Errorf("cannot list pr files: %w", err)
		}

		for _, file := range files {
			for _, rule := range rules {
				matched, err := doublestar.Match(rule.Pattern, file.GetFilename())
				if err != nil {
					return res, false, fmt.Errorf("invalid review rule pattern %q: %w", rule.Pattern, err)
				}
				if _, ok := seen[field(rule)]; matched && !ok {
					seen[field(rule)] = struct{}{}
					values = append(values, field(rule))
				}
			}
		}

		return res, true, nil
	})
	if err != nil {
		return nil, err
	}

	if len(values) == 0 {
		return valueOrNone(fallback), nil
	}

	return values, nil
}

func valueOrNone(value string) []string {
	if value == "" {
		return nil
	}

	return []string{value}
}
