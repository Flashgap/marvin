//go:generate mockgen --source=$GOFILE --destination=mock/mock.go --package mock_slack
package slack

import (
	"context"
	"errors"
	"fmt"

	"github.com/slack-go/slack"
)

var (
	ErrOpenConversation = errors.New("error opening conversation")
	ErrPostMessage      = errors.New("error posting message")
	ErrGetUser          = errors.New("error fetching user")
)

type Client interface {
	SendMessage(ctx context.Context, userID, message string) error
	GetUser(ctx context.Context, userID string) (*slack.User, error)
}

type slackClient struct {
	*slack.Client
}

func NewClient(token string) Client {
	return &slackClient{slack.New(token)}
}

func (s *slackClient) SendMessage(ctx context.Context, userID, message string) error {
	ch, _, _, err := s.OpenConversationContext(ctx, &slack.OpenConversationParameters{
		Users: []string{userID},
	})
	if err != nil {
		return fmt.Errorf("%w: %w", ErrOpenConversation, err)
	}

	if _, _, err := s.PostMessageContext(ctx, ch.ID, slack.MsgOptionText(message, false)); err != nil {
		return fmt.Errorf("%w: %w", ErrPostMessage, err)
	}
	return nil
}

func (s *slackClient) GetUser(ctx context.Context, userID string) (*slack.User, error) {
	u, err := s.GetUserInfoContext(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrGetUser, err)
	}
	return u, nil
}
