package middlewares

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/gin-gonic/gin"
	"google.golang.org/api/idtoken"

	"github.com/Flashgap/marvin/internal/config"
	"github.com/Flashgap/marvin/internal/web"
	stderror "github.com/Flashgap/marvin/pkg/stderr"
)

// googleIssuers are the two values Google uses for the iss claim of its ID tokens.
var googleIssuers = []string{"https://accounts.google.com", "accounts.google.com"}

// ValidateTaskToken verifies that inbound task requests (e.g. from Cloud
// Scheduler) carry "Authorization: Bearer <token>", where token is a Google
// OIDC token minted for cfg.TasksServiceAccount with cfg.TasksAudience as
// audience.
//
// Marvin checks the token itself; it doesn't rely on anything in front of it,
// which must pass the Authorization header through unchanged. validator checks
// the signature against Google's public keys, the audience and the expiry; the
// issuer, email and email_verified are checked here. The email check is what
// pins the caller: any service account can get a Google-signed token for our
// audience.
//
// In dev (IsDevEnv), verification is skipped, same escape hatch as
// ValidateGithubWebhook. Everywhere else it always runs, and a missing field
// rejects every request.
func ValidateTaskToken(cfg config.Tasks, isDevEnv bool, validator *idtoken.Validator) gin.HandlerFunc {
	if isDevEnv {
		return func(c *gin.Context) { c.Next() }
	}

	return func(c *gin.Context) {
		if err := verifyTaskToken(c.Request.Context(), c.GetHeader("Authorization"), cfg, validator); err != nil {
			web.DefaultController.Error(c, fmt.Errorf("%w: ValidateTaskToken: %w", stderror.ErrUnauthorized, err))
			return
		}

		c.Next()
	}
}

func verifyTaskToken(ctx context.Context, authorization string, cfg config.Tasks, validator *idtoken.Validator) error {
	if !cfg.TasksConfigured() {
		return errors.New("MARVIN_TASKS_AUDIENCE and MARVIN_TASKS_SERVICE_ACCOUNT must both be set")
	}

	token, ok := strings.CutPrefix(authorization, "Bearer ")
	if !ok {
		return errors.New("missing bearer token")
	}

	payload, err := validator.Validate(ctx, token, cfg.TasksAudience)
	if err != nil {
		return err
	}

	if !slices.Contains(googleIssuers, payload.Issuer) {
		return fmt.Errorf("unexpected issuer %q", payload.Issuer)
	}
	if email, _ := payload.Claims["email"].(string); email != cfg.TasksServiceAccount {
		return fmt.Errorf("unexpected email %q", email)
	}
	if verified, _ := payload.Claims["email_verified"].(bool); !verified {
		return errors.New("email not verified")
	}

	return nil
}
