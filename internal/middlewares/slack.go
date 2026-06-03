package middlewares

import (
	"bytes"
	"fmt"
	"io"

	"github.com/gin-gonic/gin"
	"github.com/slack-go/slack"

	"github.com/Flashgap/marvin/internal/config"
	"github.com/Flashgap/marvin/internal/web"
	stderror "github.com/Flashgap/marvin/pkg/stderr"
)

// ValidateSlackWebhook verifies inbound slash-command requests using slack-go's
// SecretsVerifier — which handles X-Slack-Signature parsing, the 5-minute
// timestamp replay window, and the HMAC-SHA256 comparison.
//
// The middleware reads the request body and restores it for downstream handlers
// so they can re-bind the form payload.
//
// In dev (IsDevEnv) when SlackSigningSecret is empty, signature verification is
// skipped — same escape hatch as ValidateGithubWebhook so local dev doesn't
// need a real Slack app.
func ValidateSlackWebhook(cfg config.Slack, isDevEnv bool) gin.HandlerFunc {
	bypass := isDevEnv && cfg.SlackSigningSecret == ""

	return func(c *gin.Context) {
		body, err := io.ReadAll(c.Request.Body)
		if err != nil {
			web.DefaultController.Error(c, fmt.Errorf("%w: ValidateSlackWebhook read body: %w", stderror.ErrUnauthorized, err))
			return
		}
		c.Request.Body = io.NopCloser(bytes.NewReader(body))

		if bypass {
			c.Next()
			return
		}

		sv, err := slack.NewSecretsVerifier(c.Request.Header, cfg.SlackSigningSecret)
		if err != nil {
			web.DefaultController.Error(c, fmt.Errorf("%w: ValidateSlackWebhook: %w", stderror.ErrUnauthorized, err))
			return
		}
		if _, err := sv.Write(body); err != nil {
			web.DefaultController.Error(c, fmt.Errorf("%w: ValidateSlackWebhook: %w", stderror.ErrUnauthorized, err))
			return
		}
		if err := sv.Ensure(); err != nil {
			web.DefaultController.Error(c, fmt.Errorf("%w: ValidateSlackWebhook: %w", stderror.ErrUnauthorized, err))
			return
		}

		c.Next()
	}
}
