package marvin

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/slack-go/slack"

	"github.com/Flashgap/marvin/internal/service/lock"
	weberrors "github.com/Flashgap/marvin/internal/web/errors"
	stderror "github.com/Flashgap/marvin/pkg/stderr"
)

func (ctrl *Controller) lockHandler(c *gin.Context) {
	if ctrl.lockService == nil {
		c.AbortWithStatusJSON(http.StatusNotImplemented, weberrors.GenericNotImplementedError)
		return
	}

	cmd, err := slack.SlashCommandParse(c.Request)
	if err != nil {
		ctrl.Error(c, fmt.Errorf("%w: parsing slash command: %w", stderror.ErrParsing, err))
		return
	}
	cmd.Text = strings.TrimSpace(cmd.Text)

	var resp *lock.Response
	if cmd.Text == "" {
		resp, err = ctrl.lockService.Leaderboard(c.Request.Context())
	} else {
		resp, err = ctrl.lockService.Lock(c.Request.Context(), cmd)
	}
	if ctrl.Error(c, err) {
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"response_type": resp.Type,
		"text":          resp.Text,
	})
}
