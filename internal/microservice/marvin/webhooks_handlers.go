package marvin

import (
	"context"
	"fmt"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
	gogithub "github.com/google/go-github/v63/github"
	"golang.org/x/oauth2"

	"github.com/Flashgap/marvin/internal/middlewares"
	"github.com/Flashgap/marvin/pkg/linear"
	"github.com/Flashgap/marvin/pkg/logger"
	stderror "github.com/Flashgap/marvin/pkg/stderr"
)

func (ctrl *Controller) handleGithubRequest(ctx context.Context, req *http.Request) ([]byte, error) {
	log := logger.WithContext(ctx).WithPrefix("[handleGithubRequest]")

	body, err := io.ReadAll(req.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading request body: %w", err)
	}

	webhookType := gogithub.WebHookType(req)
	log.Infof("Got a webhook event of type %q", webhookType)

	eventPayload, err := gogithub.ParseWebHook(webhookType, body)
	if err != nil {
		return nil, fmt.Errorf("error reading parsing body: %w", err)
	}

	switch event := eventPayload.(type) {
	case *gogithub.PullRequestEvent:
		middlewares.EnrichGHContext(ctx, event.GetRepo().GetName(), event.GetPullRequest().GetNumber())
		err = ctrl.marvinService.OnPullRequest(ctx, event)
	case *gogithub.PullRequestReviewEvent:
		middlewares.EnrichGHContext(ctx, event.GetRepo().GetName(), event.GetPullRequest().GetNumber())
		err = ctrl.marvinService.OnPullRequestReview(ctx, event)
	case *gogithub.CheckRunEvent:
		middlewares.EnrichGHContext(ctx, event.GetRepo().GetName(), int(event.GetCheckRun().GetID()))
		err = ctrl.marvinService.OnCheckRun(ctx, event)
	}

	if err != nil {
		log.Errorf("error during callback: %v", err)
		return nil, fmt.Errorf("error during callback: %w", err)
	}

	return body, nil
}

func (ctrl *Controller) githubHandler(c *gin.Context) {
	payload, err := ctrl.handleGithubRequest(c, c.Request)
	if ctrl.Error(c, err) {
		return
	}

	// Returning the payload for debugging purposes.
	// The GitHub UI allows you to recover this payload so you can feed it to a local instance of your bot for example.
	c.JSON(http.StatusOK, payload)
}

// linearOAuthLogin is only made available in a local environment and run in the specific case where we'd need to re-generate a linear OAuth token
// ClientID, ClientSecret and RedirectURL need to be filled with the information given by linear
// linearOAuthState needs to be filled with a random string that matches the one in the corresponding endpoint handler linearOAuthLogin
func (ctrl *Controller) linearOAuthCallback(c *gin.Context) {
	log := logger.WithContext(c).WithPrefix("[linearOAuthCallback]")

	var (
		linearOAuthConfig = &oauth2.Config{
			ClientID:     "",
			ClientSecret: "",
			Endpoint: oauth2.Endpoint{
				AuthURL:  linear.AuthURL,
				TokenURL: linear.TokenURL,
			},
			RedirectURL: "",
			Scopes:      []string{linear.ScopeRead},
		}
		linearOAuthState = ""
	)

	if state := c.Query("state"); state != linearOAuthState {
		ctrl.Error(c, fmt.Errorf("%w: invalid OAuth state %q", stderror.ErrUnauthorized, state))
	}

	token, err := linearOAuthConfig.Exchange(c, c.Query("code"))
	if err != nil {
		ctrl.Error(c, fmt.Errorf("error exchanging token: %w", err))
	}

	log.Infof("Got token %+v", token)
}
