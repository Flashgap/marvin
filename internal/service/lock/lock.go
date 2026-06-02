//go:generate mockgen --source=$GOFILE --destination=mock/mock.go --package mock_lock
package lock

import (
	"context"
	"errors"
)

// ResponseType matches Slack's response_type field but is defined here so the
// service stays free of HTTP/Slack JSON knowledge. The controller maps it on
// the wire.
type ResponseType string

const (
	ResponseEphemeral ResponseType = "ephemeral"
	ResponseInChannel ResponseType = "in_channel"
)

// Response is the domain payload returned to whoever ran the slash command.
type Response struct {
	Type ResponseType
	Text string
}

// SlashPayload is the subset of Slack's slash-command form we care about.
// The caller (UserID) is the victim of the prank; Text contains the mention
// of the finder, e.g. "<@U12345|alice>".
type SlashPayload struct {
	UserID   string
	UserName string
	Text     string
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
	// Lock applies the point change parsed from payload.Text (the finder).
	// The caller (payload.UserID) is the victim.
	Lock(ctx context.Context, payload SlashPayload) (*Response, error)
	// Leaderboard returns the top-3 / bottom-3 view as an ephemeral Response.
	Leaderboard(ctx context.Context) (*Response, error)
}
