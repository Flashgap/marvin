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
// config.DefaultTeam when no rule matches any changed file. Returns an empty slice when there is
// nothing to resolve (no rules and no default team configured).
func (s *service) resolveReviewTeams(ctx context.Context, webhook pkggithub.RepoSenderGetter, prNumber int, config *GitHubRepositoryConfiguration) ([]string, error) {
	if len(config.ReviewRules) == 0 {
		return defaultTeamOrNone(config), nil
	}

	matchedTeams := make(map[string]struct{})
	err := pkggithub.ConsumePaginatedResource(pkggithub.MaxPerPage, func(opts *gogithub.ListOptions) (*gogithub.Response, bool, error) {
		files, res, err := s.githubService.ListPRFiles(ctx, webhook, prNumber, opts)
		if err != nil {
			return res, false, fmt.Errorf("cannot list pr files: %w", err)
		}

		for _, file := range files {
			for _, rule := range config.ReviewRules {
				matched, err := doublestar.Match(rule.Pattern, file.GetFilename())
				if err != nil {
					return res, false, fmt.Errorf("invalid review rule pattern %q: %w", rule.Pattern, err)
				}
				if matched {
					matchedTeams[rule.Team] = struct{}{}
				}
			}
		}

		return res, true, nil
	})
	if err != nil {
		return nil, err
	}

	if len(matchedTeams) == 0 {
		return defaultTeamOrNone(config), nil
	}

	teams := make([]string, 0, len(matchedTeams))
	for team := range matchedTeams {
		teams = append(teams, team)
	}

	return teams, nil
}

func defaultTeamOrNone(config *GitHubRepositoryConfiguration) []string {
	if config.DefaultTeam == "" {
		return nil
	}

	return []string{config.DefaultTeam}
}
