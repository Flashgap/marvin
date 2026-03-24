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
)

type Client interface {
	SendMessage(ctx context.Context, userID, message string) error
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
