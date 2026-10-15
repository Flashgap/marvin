//go:generate mockgen --source=$GOFILE --destination=mock/mock.go --package mock_reviewload
package reviewload

import (
	"context"
	"fmt"

	gogithub "github.com/google/go-github/v90/github"

	pkggithub "github.com/Flashgap/marvin/pkg/github"
	"github.com/Flashgap/marvin/pkg/linguist"
)

// Service ranks reviewers by review load.
type Service interface {
	// RepoReviewLoad ranks everyone reviewing an open PR of the installed repository matching name, by
	// name or full name ignoring case. It returns an *UnknownRepositoryError when none matches.
	RepoReviewLoad(ctx context.Context, name string) (*gogithub.Repository, []Reviewer, error)

	// ReviewLoad ranks, for each non-archived installed repository someone is reviewing an open PR of,
	// its reviewers by their review load in that repository. Repositories are sorted by name.
	ReviewLoad(ctx context.Context) ([]RepositoryReviewers, error)

	// Rank scores reviewers of the repository's open PRs by review load, lowest first, ties broken by
	// login. With nil members, everyone reviewing an open PR is ranked. Otherwise only members are,
	// including those reviewing nothing.
	Rank(ctx context.Context, webhook pkggithub.RepoSenderGetter, members []string) ([]Reviewer, error)
}

// Gitattributes holds the last .gitattributes files read off each repository's default branch, never off a
// PR's head, which would let a PR author shrink their own PR's score. Reading them is left to a poller: ranking
// makes no call for them.
type Gitattributes interface {
	// Gitattributes returns the matcher of the files repo's .gitattributes files mark linguist-generated or
	// linguist-vendored, or nil when they exclude none or none were read: every file then counts.
	Gitattributes(repo *gogithub.Repository) *linguist.Matcher
}

// Reviewer is a login with its review load score: the sum of the lines added by the open PRs it reviews,
// leaving out the files the .gitattributes files of the repository's default branch mark linguist-generated
// or linguist-vendored.
type Reviewer struct {
	Login string
	Score int
	PRs   []PullRequest // Open PRs making up Score, sorted by number
}

// RepositoryReviewers is a repository with its reviewers ranked by review load in it.
type RepositoryReviewers struct {
	Repo      *gogithub.Repository
	Reviewers []Reviewer
}

// PullRequest identifies an open PR counted in a Reviewer's score.
type PullRequest struct {
	Number int
	URL    string
}

// UnknownRepositoryError is returned when no installed repository matches the requested name.
type UnknownRepositoryError struct {
	Name         string
	Repositories []string // Names of the installed repositories, sorted
}

func (e *UnknownRepositoryError) Error() string {
	return fmt.Sprintf("unknown repository %q", e.Name)
}
