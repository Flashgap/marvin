package marvin

import (
	"net/http"

	"github.com/gin-gonic/gin"

	weberrors "github.com/Flashgap/marvin/internal/web/errors"
)

func (ctrl *Controller) standupRemindHandler(c *gin.Context) {
	if ctrl.standupService == nil {
		c.AbortWithStatusJSON(http.StatusNotImplemented, weberrors.GenericNotImplementedError)
		return
	}

	report, err := ctrl.standupService.Remind(c.Request.Context())
	if ctrl.Error(c, err) {
		return
	}

	c.JSON(http.StatusOK, report)
}
