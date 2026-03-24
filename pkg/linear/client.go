//go:generate mockgen --source=$GOFILE --destination=mock/mock.go --package mock_linear
package linear

import (
	"context"
	"time"

	"github.com/shurcooL/graphql"
	"golang.org/x/oauth2"
)

const (
	baseURL       = "https://api.linear.app"
	graphQLAPIURL = baseURL + "/graphql"

	AuthURL   = "https://linear.app/oauth/authorize"
	TokenURL  = baseURL + "/oauth/token"
	ScopeRead = "read"
)

type Client interface {
	Issue(ctx context.Context, id string) (*Issue, error)
}

type client struct {
	*graphql.Client
	workspaceSlug string
}

// NewClient performs the second part of the OAuth handshake and returns a Client that is ready to perform
// In order to retrieve an OAuth Code, see ClientLogin and implement the necessary callback
func NewClient(ctx context.Context, oAuthToken string, workspaceSlug string) Client {
	httpClient := oauth2.NewClient(ctx, oauth2.StaticTokenSource(&oauth2.Token{
		AccessToken: oAuthToken,
		// The token was generated some time in 2023, and is supposed to last 10 years. This should be fine and
		// provide us with a bit of a margin.
		Expiry: time.Date(2030, 0, 0, 0, 0, 0, 0, time.UTC),
	}))

	return &client{
		Client:        graphql.NewClient(graphQLAPIURL, httpClient),
		workspaceSlug: workspaceSlug,
	}
}
