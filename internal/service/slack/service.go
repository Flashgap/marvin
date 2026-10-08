package slack

import (
	"context"
	"strconv"
	"strings"
	"time"

	pkgslack "github.com/Flashgap/marvin/pkg/slack"
)

type service struct {
	client pkgslack.Client
}

// NewService returns a Service backed by the given low-level client.
func NewService(client pkgslack.Client) Service {
	return &service{client: client}
}

func (s *service) SendDM(ctx context.Context, userID, message string) error {
	return s.client.SendMessage(ctx, userID, message)
}

func (s *service) GetUser(ctx context.Context, userID string) (*User, error) {
	u, err := s.client.GetUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	name := u.Profile.DisplayName
	if name == "" {
		name = u.RealName
	}
	if name == "" {
		name = u.Name
	}
	return &User{ID: u.ID, Name: name, IsBot: u.IsBot, Deleted: u.Deleted}, nil
}

func (s *service) ChannelMembers(ctx context.Context, channelID string) ([]string, error) {
	return s.client.GetChannelMembers(ctx, channelID)
}

func (s *service) ChannelHistory(ctx context.Context, channelID string, oldest time.Time) ([]Message, error) {
	msgs, err := s.client.GetChannelHistory(ctx, channelID, oldest)
	if err != nil {
		return nil, err
	}
	out := make([]Message, 0, len(msgs))
	for _, m := range msgs {
		out = append(out, Message{
			UserID:   m.User,
			TS:       m.Timestamp,
			PostedAt: ParseTS(m.Timestamp),
			Text:     m.Text,
			SubType:  m.SubType,
			BotID:    m.BotID,
		})
	}
	return out, nil
}

// ParseTS returns the UTC time of a Slack ts such as "1759820000.123456",
// truncated to the second. A malformed ts gives the zero time.
func ParseTS(ts string) time.Time {
	sec, _, _ := strings.Cut(ts, ".")
	unix, err := strconv.ParseInt(sec, 10, 64)
	if err != nil {
		return time.Time{}
	}
	return time.Unix(unix, 0).UTC()
}
