package reviewload

import (
	"context"
	"fmt"
	"sort"
	"strings"

	gogithub "github.com/google/go-github/v90/github"

	pkggithub "github.com/Flashgap/marvin/pkg/github"
)

type service struct {
	githubClient pkggithub.Client
}

// NewService wires the review load service on the GitHub App's client.
func NewService(githubClient pkggithub.Client) Service {
	return &service{githubClient: githubClient}
}

func (s *service) RepoReviewLoad(ctx context.Context, name string) (*gogithub.Repository, []Reviewer, error) {
	repos, err := s.listInstalledRepos(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("listing installed repositories: %w", err)
	}

	repo := findRepo(repos, name)
	if repo == nil {
		names := make([]string, 0, len(repos))
		for _, r := range repos {
			names = append(names, r.GetName())
		}
		sort.Strings(names)

		return nil, nil, &UnknownRepositoryError{Name: name, Repositories: names}
	}

	reviewers, err := s.Rank(ctx, installedRepo{repo: repo}, nil)
	if err != nil {
		return nil, nil, err
	}

	return repo, reviewers, nil
}

func (s *service) Rank(ctx context.Context, webhook pkggithub.RepoSenderGetter, members []string) ([]Reviewer, error) {
	prs, err := s.githubClient.ListOpenPRsWithReviewers(ctx, webhook)
	if err != nil {
		return nil, fmt.Errorf("listing open pull requests with reviewers: %w", err)
	}

	byLogin := make(map[string]*Reviewer, len(members))
	for _, login := range members {
		byLogin[login] = &Reviewer{Login: login}
	}

	for _, pr := range prs {
		for login := range pr.Reviewers {
			r, ok := byLogin[login]
			if !ok {
				if members != nil {
					continue
				}
				r = &Reviewer{Login: login}
				byLogin[login] = r
			}
			r.Score += pr.Additions
			r.PRs = append(r.PRs, pr.Number)
		}
	}

	reviewers := make([]Reviewer, 0, len(byLogin))
	for _, r := range byLogin {
		sort.Ints(r.PRs)
		reviewers = append(reviewers, *r)
	}

	sort.Slice(reviewers, func(i, j int) bool {
		if reviewers[i].Score != reviewers[j].Score {
			return reviewers[i].Score < reviewers[j].Score
		}
		return reviewers[i].Login < reviewers[j].Login
	})

	return reviewers, nil
}

func (s *service) listInstalledRepos(ctx context.Context) ([]*gogithub.Repository, error) {
	var all []*gogithub.Repository
	opts := &gogithub.ListOptions{PerPage: pkggithub.MaxPerPage}

	for {
		result, res, err := s.githubClient.ListInstalledRepos(ctx, opts)
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

// findRepo matches name against each repository's name or full name, ignoring case.
func findRepo(repos []*gogithub.Repository, name string) *gogithub.Repository {
	if name == "" {
		return nil
	}
	for _, repo := range repos {
		if strings.EqualFold(repo.GetName(), name) || strings.EqualFold(repo.GetFullName(), name) {
			return repo
		}
	}

	return nil
}

// installedRepo adapts a repository from the installation listing into a pkggithub.RepoSenderGetter.
// It carries no sender since a review load lookup isn't attributable to any GitHub user.
type installedRepo struct {
	repo *gogithub.Repository
}

func (r installedRepo) GetRepo() *gogithub.Repository { return r.repo }
func (installedRepo) GetSender() *gogithub.User       { return nil }
