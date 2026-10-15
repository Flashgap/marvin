package config

// Standup configuration. The daily standup reminder is optional: it requires
// the database and is disabled when StandupChannelID is empty.
type Standup struct {
	// StandupChannelID is the ID (not the name) of the Slack channel the team
	// posts its standup in, e.g. C0123456789. When empty, the standup reminder
	// is disabled.
	StandupChannelID string `envconfig:"MARVIN_STANDUP_CHANNEL_ID"`
}

// StandupEnabled reports whether the standup reminder should be initialized.
func (c Standup) StandupEnabled() bool {
	return c.StandupChannelID != ""
}
