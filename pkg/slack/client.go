//go:generate mockgen --source=$GOFILE --destination=mock/mock.go --package mock_slack
package slack

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/slack-go/slack"
)

var (
	ErrOpenConversation  = errors.New("error opening conversation")
	ErrPostMessage       = errors.New("error posting message")
	ErrGetUser           = errors.New("error fetching user")
	ErrGetChannelMembers = errors.New("error fetching channel members")
	ErrGetChannelHistory = errors.New("error fetching channel history")
)

// pageSize is the number of items requested per page from Slack's paginated methods.
const pageSize = 200

type Client interface {
	SendMessage(ctx context.Context, userID, message string) error
	GetUser(ctx context.Context, userID string) (*slack.User, error)
	// GetChannelMembers returns every member ID of channelID, following pagination.
	GetChannelMembers(ctx context.Context, channelID string) ([]string, error)
	// GetChannelHistory returns channelID's messages with ts > oldest, newest first, following pagination.
	// conversations.history only returns top-level messages (plus thread_broadcast copies), never thread replies.
	GetChannelHistory(ctx context.Context, channelID string, oldest time.Time) ([]slack.Message, error)
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

func (s *slackClient) GetChannelMembers(ctx context.Context, channelID string) ([]string, error) {
	params := &slack.GetUsersInConversationParameters{ChannelID: channelID, Limit: pageSize}
	var members []string
	for {
		page, cursor, err := s.GetUsersInConversationContext(ctx, params)
		if err != nil {
			return nil, fmt.Errorf("%w: %w", ErrGetChannelMembers, err)
		}
		members = append(members, page...)
		if cursor == "" {
			return members, nil
		}
		params.Cursor = cursor
	}
}

func (s *slackClient) GetChannelHistory(ctx context.Context, channelID string, oldest time.Time) ([]slack.Message, error) {
	params := &slack.GetConversationHistoryParameters{
		ChannelID: channelID,
		Oldest:    strconv.FormatInt(oldest.Unix(), 10),
		Limit:     pageSize,
	}
	var messages []slack.Message
	for {
		resp, err := s.GetConversationHistoryContext(ctx, params)
		if err != nil {
			return nil, fmt.Errorf("%w: %w", ErrGetChannelHistory, err)
		}
		messages = append(messages, resp.Messages...)
		if !resp.HasMore || resp.ResponseMetaData.NextCursor == "" {
			return messages, nil
		}
		params.Cursor = resp.ResponseMetaData.NextCursor
	}
}

// IsPermanentDMError reports whether err, as returned by SendMessage, is a Slack error that no retry
// can fix: the user is a bot, deactivated, or not visible to Marvin.
func IsPermanentDMError(err error) bool {
	var slackErr slack.SlackErrorResponse
	if !errors.As(err, &slackErr) {
		return false
	}
	switch slackErr.Err {
	case "cannot_dm_bot", "user_not_found", "user_not_visible", "user_disabled":
		return true
	default:
		return false
	}
}
