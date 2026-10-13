package middlewares

import (
	"crypto/subtle"
	"fmt"

	"github.com/gin-gonic/gin"

	"github.com/Flashgap/marvin/internal/config"
	"github.com/Flashgap/marvin/internal/web"
	stderror "github.com/Flashgap/marvin/pkg/stderr"
)

// ValidateTaskSecret verifies that inbound task requests (e.g. from Cloud
// Scheduler) carry "Authorization: Bearer <TasksSecret>".
//
// Unlike the GitHub and Slack webhooks, there is no dev bypass: a bearer
// secret is trivial to send by hand, so an empty TasksSecret rejects every
// request, in dev too.
func ValidateTaskSecret(cfg config.Tasks) gin.HandlerFunc {
	want := []byte("Bearer " + cfg.TasksSecret)

	return func(c *gin.Context) {
		got := []byte(c.GetHeader("Authorization"))
		if cfg.TasksSecret == "" || subtle.ConstantTimeCompare(got, want) != 1 {
			web.DefaultController.Error(c, fmt.Errorf("%w: ValidateTaskSecret", stderror.ErrUnauthorized))
			return
		}

		c.Next()
	}
}
