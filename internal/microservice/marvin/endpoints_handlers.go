package marvin

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"golang.org/x/oauth2"

	"github.com/Flashgap/marvin/pkg/linear"
)

// linearOAuthLogin is only made available in a local environment and run in the specific case where we'd need to re-generate a linear OAuth token
// ClientID, ClientSecret and RedirectURL need to be filled with the information given by linear
// linearOAuthState needs to be filled with a random string that matches the one in the corresponding webhook handler linearOAuthCallback
func (*Controller) linearOAuthLogin(c *gin.Context) {
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

	c.Redirect(http.StatusTemporaryRedirect, linearOAuthConfig.AuthCodeURL(linearOAuthState, oauth2.SetAuthURLParam("actor", "application")))
}
