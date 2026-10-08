package reviewload

import (
	"cmp"
	"context"
	"fmt"
	"slices"
	"sort"
	"strings"

	gogithub "github.com/google/go-github/v90/github"
	"golang.org/x/sync/errgroup"

	pkggithub "github.com/Flashgap/marvin/pkg/github"
)

// maxConcurrentRepos caps the GitHub queries ReviewLoad runs at once.
const maxConcurrentRepos = 8

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

func (s *service) ReviewLoad(ctx context.Context) ([]RepositoryReviewers, error) {
	repos, err := s.listInstalledRepos(ctx)
	if err != nil {
		return nil, fmt.Errorf("listing installed repositories: %w", err)
	}

	// One query per repository: run them concurrently
	perRepo := make([][]Reviewer, len(repos))
	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(maxConcurrentRepos)
	for i, repo := range repos {
		if repo.GetArchived() {
			continue
		}
		g.Go(func() error {
			reviewers, err := s.Rank(gctx, installedRepo{repo: repo}, nil)
			perRepo[i] = reviewers
			return err
		})
	}
	if err := g.Wait(); err != nil {
		return nil, err
	}

	var result []RepositoryReviewers
	for i, reviewers := range perRepo {
		if len(reviewers) > 0 {
			result = append(result, RepositoryReviewers{Repo: repos[i], Reviewers: reviewers})
		}
	}
	slices.SortFunc(result, func(a, b RepositoryReviewers) int {
		return cmp.Compare(a.Repo.GetName(), b.Repo.GetName())
	})

	return result, nil
}

func (s *service) Rank(ctx context.Context, webhook pkggithub.RepoSenderGetter, members []string) ([]Reviewer, error) {
	prs, err := s.openPRs(ctx, webhook)
	if err != nil {
		return nil, err
	}

	return rank(prs, members), nil
}

// openPR is an open PR with its size and reviewers.
type openPR struct {
	PullRequest
	additions int
	reviewers map[string]struct{}
}

func (s *service) openPRs(ctx context.Context, webhook pkggithub.RepoSenderGetter) ([]openPR, error) {
	loads, err := s.githubClient.ListOpenPRsWithReviewers(ctx, webhook)
	if err != nil {
		return nil, fmt.Errorf("listing open pull requests with reviewers of %s: %w", webhook.GetRepo().GetFullName(), err)
	}

	repo := webhook.GetRepo()
	prs := make([]openPR, 0, len(loads))
	for _, load := range loads {
		prs = append(prs, openPR{
			PullRequest: PullRequest{
				Number: load.Number,
				URL:    fmt.Sprintf("%s/pull/%d", repo.GetHTMLURL(), load.Number),
			},
			additions: load.Additions,
			reviewers: load.Reviewers,
		})
	}

	return prs, nil
}

// rank scores reviewers by review load, lowest first, ties broken by login. With nil members, everyone
// reviewing one of prs is ranked. Otherwise only members are, including those reviewing nothing.
func rank(prs []openPR, members []string) []Reviewer {
	byLogin := make(map[string]*Reviewer, len(members))
	for _, login := range members {
		byLogin[login] = &Reviewer{Login: login}
	}

	for _, pr := range prs {
		for login := range pr.reviewers {
			r, ok := byLogin[login]
			if !ok {
				if members != nil {
					continue
				}
				r = &Reviewer{Login: login}
				byLogin[login] = r
			}
			r.Score += pr.additions
			r.PRs = append(r.PRs, pr.PullRequest)
		}
	}

	reviewers := make([]Reviewer, 0, len(byLogin))
	for _, r := range byLogin {
		slices.SortFunc(r.PRs, func(a, b PullRequest) int {
			return cmp.Compare(a.Number, b.Number)
		})
		reviewers = append(reviewers, *r)
	}
	slices.SortFunc(reviewers, func(a, b Reviewer) int {
		return cmp.Or(cmp.Compare(a.Score, b.Score), cmp.Compare(a.Login, b.Login))
	})

	return reviewers
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
