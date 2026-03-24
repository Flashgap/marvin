//go:generate mockgen --source=$GOFILE --destination=mock/mock.go --package mock_marvin
package marvin

import (
	"context"

	gogithub "github.com/google/go-github/v63/github"
)

type Service interface {
	OnPullRequest(context.Context, *gogithub.PullRequestEvent) error
	OnPullRequestReview(context.Context, *gogithub.PullRequestReviewEvent) error
	OnCheckRun(context.Context, *gogithub.CheckRunEvent) error
}
