//go:generate mockgen --source=$GOFILE --destination=mock/mock.go --package mock_lock
package lock

import (
	"context"

	"github.com/slack-go/slack"
)

// Service is the controller's single dependency. It owns the DB pool and the
// SlackService, runs migrations at construction, and performs all business
// logic for the /lock command. Both methods return a slack.Msg ready to be
// serialized as the slash-command response (controller writes it as JSON).
//
// Validation outcomes (self-lock, bot target, cooldown, malformed mention) are
// returned as ephemeral slack.Msg values with a nil error — only genuine
// failures (DB outage, Slack API error) surface through the error return.
type Service interface {
	// Lock applies the point change parsed from cmd.Text (the finder mention).
	// The caller (cmd.UserID) is the victim.
	Lock(ctx context.Context, cmd slack.SlashCommand) (*slack.Msg, error)
	// Leaderboard returns the top-3 / bottom-3 view as an ephemeral message.
	Leaderboard(ctx context.Context) (*slack.Msg, error)
}
