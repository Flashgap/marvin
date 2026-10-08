package reviewload

import (
	"context"
	"fmt"
	"sort"
	"strings"

	gogithub "github.com/google/go-github/v90/github"
	"github.com/slack-go/slack"

	pkggithub "github.com/Flashgap/marvin/pkg/github"
)

type service struct {
	githubClient pkggithub.Client
}

// NewService wires the review load service on the GitHub App's client.
func NewService(githubClient pkggithub.Client) Service {
	return &service{githubClient: githubClient}
}

func (s *service) ReviewLoad(ctx context.Context, cmd slack.SlashCommand) (*slack.Msg, error) {
	repos, err := s.listInstalledRepos(ctx)
	if err != nil {
		return nil, fmt.Errorf("listing installed repositories: %w", err)
	}

	name := strings.TrimSpace(cmd.Text)
	repo := findRepo(repos, name)
	if repo == nil {
		return ephemeral(usage(name, repos)), nil
	}

	prs, err := s.githubClient.ListOpenPRsWithReviewers(ctx, installedRepo{repo: repo})
	if err != nil {
		return nil, fmt.Errorf("listing open pull requests with reviewers: %w", err)
	}

	return ephemeral(format(repo, Rank(prs, nil))), nil
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

// Reviewer is a login with its review load score: the sum of the additions of the open PRs it reviews.
type Reviewer struct {
	Login string
	Score int
	PRs   []int // Numbers of the open PRs making up Score, sorted
}

// Rank scores reviewers by review load, lowest first, ties broken by login. With nil members, everyone
// reviewing an open PR is ranked. Otherwise only members are, including those reviewing nothing.
func Rank(prs []pkggithub.OpenPRReviewLoad, members []string) []Reviewer {
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

	return reviewers
}

func format(repo *gogithub.Repository, reviewers []Reviewer) string {
	if len(reviewers) == 0 {
		return fmt.Sprintf("Nobody is reviewing an open PR of *%s*.", repo.GetFullName())
	}

	var b strings.Builder
	fmt.Fprintf(&b, "*Review load of %s* (additions of the open PRs each person reviews, lowest is picked first)\n", repo.GetFullName())
	for _, r := range reviewers {
		links := make([]string, 0, len(r.PRs))
		for _, number := range r.PRs {
			links = append(links, fmt.Sprintf("<%s/pull/%d|#%d>", repo.GetHTMLURL(), number, number))
		}
		fmt.Fprintf(&b, "• *%s* — %d — %s\n", r.Login, r.Score, strings.Join(links, ", "))
	}

	return b.String()
}

func usage(name string, repos []*gogithub.Repository) string {
	names := make([]string, 0, len(repos))
	for _, repo := range repos {
		names = append(names, "`"+repo.GetName()+"`")
	}
	sort.Strings(names)

	prefix := "Usage: `/review-load <repository>`."
	if name != "" {
		prefix = fmt.Sprintf("Unknown repository `%s`.", name)
	}

	return fmt.Sprintf("%s Repositories: %s", prefix, strings.Join(names, ", "))
}

func ephemeral(text string) *slack.Msg {
	return &slack.Msg{ResponseType: slack.ResponseTypeEphemeral, Text: text}
}

// installedRepo adapts a repository from the installation listing into a pkggithub.RepoSenderGetter.
// It carries no sender since a slash command isn't attributable to any GitHub user.
type installedRepo struct {
	repo *gogithub.Repository
}

func (r installedRepo) GetRepo() *gogithub.Repository { return r.repo }
func (installedRepo) GetSender() *gogithub.User       { return nil }
