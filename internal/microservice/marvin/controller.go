package marvin

import (
	"path"

	"github.com/gin-gonic/gin"

	"github.com/Flashgap/marvin/internal/middlewares"
	marvinroute "github.com/Flashgap/marvin/internal/route/marvin"
	"github.com/Flashgap/marvin/internal/service/lock"
	"github.com/Flashgap/marvin/internal/service/marvin"
	"github.com/Flashgap/marvin/internal/service/reviewload"
	"github.com/Flashgap/marvin/internal/service/standup"
	"github.com/Flashgap/marvin/internal/web"
	"github.com/Flashgap/marvin/pkg/googleid"
)

type Controller struct {
	web.BaseController
	configuration     *Config
	marvinService     marvin.Service
	lockService       lock.Service
	reviewLoad        reviewload.Service
	standupService    standup.Service
	taskTokenVerifier *googleid.Verifier
}

func NewController(configuration *Config, marvinService marvin.Service, lockService lock.Service, reviewLoad reviewload.Service, standupService standup.Service, taskTokenVerifier *googleid.Verifier) *Controller {
	return &Controller{
		configuration:     configuration,
		marvinService:     marvinService,
		lockService:       lockService,
		reviewLoad:        reviewLoad,
		standupService:    standupService,
		taskTokenVerifier: taskTokenVerifier,
	}
}

// RouteEndpoints registers the module endpoints handlers
func (ctrl *Controller) RouteEndpoints(router gin.IRouter) {
	if ctrl.configuration.IsDevEnv {
		// This route is only required in the case where we'd need to refresh the linear token.
		// In that case Marvin would be running locally
		router.GET(path.Join(marvinroute.Linear, marvinroute.LinearLogin), ctrl.linearOAuthLogin)
	}
}

func (ctrl *Controller) RouteTasks(router gin.IRouter) {
	// Every task route requires a task token: checked on the group, so a route
	// added below can't miss it.
	router.Use(middlewares.ValidateTaskToken(ctrl.configuration.IsDevEnv, ctrl.taskTokenVerifier.Verify))

	// Daily standup reminder, triggered by a cron (e.g. Cloud Scheduler). The
	// handler returns 501 when no DB-backed standup service is wired in.
	router.POST(path.Join(marvinroute.Standup, marvinroute.StandupRemind), ctrl.standupRemindHandler)
}

func (ctrl *Controller) RouteWebhooks(router gin.IRouter) {
	// https://docs.github.com/en/webhooks-and-events/webhooks/securing-your-webhooks
	router.POST(path.Join(marvinroute.Github, marvinroute.GithubWebhook), middlewares.ValidateGithubWebhook(ctrl.configuration.Github, ctrl.configuration.IsDevEnv), ctrl.githubHandler)

	// Slack /lock slash command — gated on a configured database; the handler
	// itself returns 501 when no DB-backed lock service is wired in.
	router.POST(
		path.Join(marvinroute.Slack, marvinroute.SlackLock),
		middlewares.ValidateSlackWebhook(ctrl.configuration.Slack, ctrl.configuration.IsDevEnv),
		ctrl.lockHandler,
	)

	// Slack /review-load slash command
	router.POST(
		path.Join(marvinroute.Slack, marvinroute.SlackReviewLoad),
		middlewares.ValidateSlackWebhook(ctrl.configuration.Slack, ctrl.configuration.IsDevEnv),
		ctrl.reviewLoadHandler,
	)

	if ctrl.configuration.IsDevEnv {
		// This route is only required in the case where we'd need to refresh the linear token.
		// In that case Marvin would be running locally
		router.GET(path.Join(marvinroute.Linear, marvinroute.LinearCallback), ctrl.linearOAuthCallback)
	}
}
