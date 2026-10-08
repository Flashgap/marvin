//go:generate mockgen --source=$GOFILE --destination=mock/mock.go --package mock_reviewload
package reviewload

import (
	"context"

	"github.com/slack-go/slack"
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
}
