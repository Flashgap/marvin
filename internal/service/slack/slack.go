//go:generate mockgen --source=$GOFILE --destination=mock/mock.go --package mock_slack
package slack

import (
	"context"
	"time"
)

// User is the subset of Slack user metadata Marvin needs.
type User struct {
	ID      string
	Name    string // display name when present, real name fallback
	IsBot   bool
	Deleted bool // deactivated account
}

// Message is the subset of a Slack channel message Marvin needs.
type Message struct {
	UserID   string
	TS       string    // Slack ts, e.g. "1759820000.123456" (also the message ID)
	PostedAt time.Time // parsed from TS, UTC
	Text     string
	SubType  string
	BotID    string
}

// Service centralizes Marvin's Slack interactions. It wraps the low-level
// pkg/slack client so that future commands have one place to add new
// operations (formatters, async responses, lookups, etc.).
type Service interface {
	// SendDM posts message to the given user via a direct conversation.
	SendDM(ctx context.Context, userID, message string) error
	// GetUser fetches metadata for the given Slack user ID.
	GetUser(ctx context.Context, userID string) (*User, error)
	// ChannelMembers returns the user ID of every member of the given channel.
	ChannelMembers(ctx context.Context, channelID string) ([]string, error)
	// ChannelHistory returns the channel's top-level messages posted after
	// oldest, newest first.
	ChannelHistory(ctx context.Context, channelID string, oldest time.Time) ([]Message, error)
}
