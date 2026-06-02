package marvin

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"

	"github.com/Flashgap/marvin/internal/service/lock"
	apperrors "github.com/Flashgap/marvin/internal/web/errors"
)

// slackSlashCommand is the subset of Slack's slash-command form payload that
// the lock controller cares about.
type slackSlashCommand struct {
	UserID   string `form:"user_id"`
	UserName string `form:"user_name"`
	Text     string `form:"text"`
}

func (ctrl *Controller) lockHandler(c *gin.Context) {
	if ctrl.lockService == nil {
		c.AbortWithStatusJSON(http.StatusNotImplemented, apperrors.GenericNotImplementedError)
		return
	}

	var cmd slackSlashCommand
	if !ctrl.Bind(c, &cmd, binding.Form) {
		return
	}

	payload := lock.SlashPayload{
		UserID:   cmd.UserID,
		UserName: cmd.UserName,
		Text:     strings.TrimSpace(cmd.Text),
	}

	var (
		resp *lock.Response
		err  error
	)
	if payload.Text == "" {
		resp, err = ctrl.lockService.Leaderboard(c.Request.Context())
	} else {
		resp, err = ctrl.lockService.Lock(c.Request.Context(), payload)
	}
	if ctrl.Error(c, err) {
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"response_type": string(resp.Type),
		"text":          resp.Text,
	})
}
