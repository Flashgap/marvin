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
// In dev (IsDevEnv) when TasksSecret is empty, the check is skipped — same
// escape hatch as ValidateSlackWebhook.
func ValidateTaskSecret(cfg config.Tasks, isDevEnv bool) gin.HandlerFunc {
	bypass := isDevEnv && cfg.TasksSecret == ""
	want := []byte("Bearer " + cfg.TasksSecret)

	return func(c *gin.Context) {
		if bypass {
			c.Next()
			return
		}

		got := []byte(c.GetHeader("Authorization"))
		if cfg.TasksSecret == "" || subtle.ConstantTimeCompare(got, want) != 1 {
			web.DefaultController.Error(c, fmt.Errorf("%w: ValidateTaskSecret", stderror.ErrUnauthorized))
			return
		}

		c.Next()
	}
}
