//go:generate mockgen --source=$GOFILE --destination=mock/mock.go --package mock_slack
package slack

import "context"

// User is the subset of Slack user metadata Marvin needs.
type User struct {
	ID    string
	Name  string // display name when present, real name fallback
	IsBot bool
}

// Service centralizes Marvin's Slack interactions. It wraps the low-level
// pkg/slack client so that future commands have one place to add new
// operations (formatters, async responses, lookups, etc.).
type Service interface {
	// SendDM posts message to the given user via a direct conversation.
	SendDM(ctx context.Context, userID, message string) error
	// GetUser fetches metadata for the given Slack user ID.
	GetUser(ctx context.Context, userID string) (*User, error)
}
