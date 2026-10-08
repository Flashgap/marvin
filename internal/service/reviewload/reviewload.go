//go:generate mockgen --source=$GOFILE --destination=mock/mock.go --package mock_reviewload
package reviewload

import (
	"context"

	"github.com/slack-go/slack"

	pkggithub "github.com/Flashgap/marvin/pkg/github"
)

// Service performs the business logic for the /review-load command and returns a slack.Msg ready to be
// serialized as the slash-command response.
//
// Usage outcomes (missing or unknown repository) are returned as ephemeral slack.Msg values with a nil
// error — only genuine failures (GitHub API error) surface through the error return.
type Service interface {
	// ReviewLoad lists, for the repository named in cmd.Text, every reviewer of its open PRs with the
	// score auto_review_assign ranks them by, lowest first.
	ReviewLoad(ctx context.Context, cmd slack.SlashCommand) (*slack.Msg, error)
	// Rank scores reviewers of the repository's open PRs by review load, lowest first, ties broken by
	// login. With nil members, everyone reviewing an open PR is ranked. Otherwise only members are,
	// including those reviewing nothing.
	Rank(ctx context.Context, webhook pkggithub.RepoSenderGetter, members []string) ([]Reviewer, error)
}
