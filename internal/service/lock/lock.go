//go:generate mockgen --source=$GOFILE --destination=mock/mock.go --package mock_lock
package lock

import (
	"context"
	"errors"

	"github.com/slack-go/slack"
)

// Response is the domain payload returned to whoever ran the slash command.
// Type carries Slack's response_type value — callers should use
// slack.ResponseTypeEphemeral / slack.ResponseTypeInChannel from slack-go.
type Response struct {
	Type string
	Text string
}

// Validation sentinels — translated into ephemeral Response values internally.
// External callers don't see them.
var (
	ErrInvalidMention = errors.New("invalid mention")
	ErrSelfLock       = errors.New("self-lock")
	ErrTargetIsBot    = errors.New("target is a bot")
	ErrTooSoon        = errors.New("cooldown")
)

// Service is the controller's single dependency. It owns the DB pool and the
// SlackService, runs migrations at construction, and performs all business
// logic for the /lock command.
type Service interface {
	// Lock applies the point change parsed from cmd.Text (the finder mention).
	// The caller (cmd.UserID) is the victim.
	Lock(ctx context.Context, cmd slack.SlashCommand) (*Response, error)
	// Leaderboard returns the top-3 / bottom-3 view as an ephemeral Response.
	Leaderboard(ctx context.Context) (*Response, error)
}
