//go:generate mockgen --source=$GOFILE --destination=mock/mock.go --package mock_standup
package standup

import "context"

// Service is the controller's single dependency for the daily standup
// reminder. It owns the DB pool and the SlackService and runs migrations at
// construction.
type Service interface {
	// Remind runs one reminder pass over the standup channel: every member who
	// hasn't posted their standup today (UTC) gets a DM quoting their last
	// update. It is idempotent per person per UTC day. A non-nil error means at
	// least one person still needs a reminder, so the caller should retry.
	Remind(ctx context.Context) (*Report, error)
}

// Report counts what one Remind pass did with the channel members.
type Report struct {
	Members         int `json:"members"`
	Reminded        int `json:"reminded"`
	AlreadyPosted   int `json:"already_posted"`
	AlreadyReminded int `json:"already_reminded"`
	Ignored         int `json:"ignored"` // bots, deactivated users, permanent DM errors
}
