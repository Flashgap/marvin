package slack

import (
	"context"

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
	return &User{ID: u.ID, Name: name, IsBot: u.IsBot}, nil
}
