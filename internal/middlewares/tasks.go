package middlewares

import (
	"context"
	"fmt"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/Flashgap/marvin/internal/web"
	stderror "github.com/Flashgap/marvin/pkg/stderr"
)

// ValidateTaskToken verifies that inbound task requests (e.g. from Cloud
// Scheduler) carry "Authorization: Bearer <token>", where verify accepts token.
//
// Marvin checks the token itself; it doesn't rely on anything in front of it,
// which must pass the Authorization header through unchanged.
//
// In dev (IsDevEnv), verification is skipped, same escape hatch as
// ValidateGithubWebhook.
func ValidateTaskToken(isDevEnv bool, verify func(ctx context.Context, token string) error) gin.HandlerFunc {
	if isDevEnv {
		return func(c *gin.Context) { c.Next() }
	}

	return func(c *gin.Context) {
		token, ok := strings.CutPrefix(c.GetHeader("Authorization"), "Bearer ")
		if !ok {
			web.DefaultController.Error(c, fmt.Errorf("%w: ValidateTaskToken: missing bearer token", stderror.ErrUnauthorized))
			return
		}

		if err := verify(c.Request.Context(), token); err != nil {
			web.DefaultController.Error(c, fmt.Errorf("%w: ValidateTaskToken: %w", stderror.ErrUnauthorized, err))
			return
		}

		c.Next()
	}
}
