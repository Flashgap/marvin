package marvin

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// standupRemindHandler doesn't do anything yet: it lets the daily standup
// reminder's Cloud Scheduler job, and its authentication, be set up and checked
// before the reminder itself lands.
func (*Controller) standupRemindHandler(c *gin.Context) {
	c.Status(http.StatusOK)
}
