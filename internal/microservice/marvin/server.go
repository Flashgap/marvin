package marvin

import (
	"context"
	"io"
	"net/http"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"

	"github.com/Flashgap/marvin/internal/constants"
	"github.com/Flashgap/marvin/internal/middlewares"
	"github.com/Flashgap/marvin/internal/middlewares/errorhandler"
	"github.com/Flashgap/marvin/internal/route"
	marvinroute "github.com/Flashgap/marvin/internal/route/marvin"
	"github.com/Flashgap/marvin/internal/server"
	"github.com/Flashgap/marvin/internal/web"
	apperrors "github.com/Flashgap/marvin/internal/web/errors"
	"github.com/Flashgap/marvin/pkg/logger"
)

type Server struct {
	*server.HTTP
	ginEngine *gin.Engine
	cfg       *Config
	ctrls     []web.Controller
	services  *Services
}

func NewServer(ctx context.Context, cfg *Config, optServices ...*Services) (*Server, error) {
	if len(optServices) > 1 {
		panic("only one Services is required")
	}
	var services *Services
	if len(optServices) == 0 {
		services = &Services{}
	} else {
		services = optServices[0]
	}

	if !cfg.IsDevEnv {
		gin.SetMode(gin.ReleaseMode)
	}
	router := gin.New()

	if services == nil {
		services = &Services{}
	}
	if err := services.initialize(ctx, cfg); err != nil {
		return nil, err
	}

	httpServer := server.NewServer(&http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           router,
		ReadHeaderTimeout: constants.ReadHeaderTimeout,
	})
	srv := &Server{
		HTTP:      httpServer,
		ginEngine: router,
		cfg:       cfg,
		services:  services,
	}

	srv.initializeControllers()
	srv.initializeRouter()

	return srv, nil
}

// initializeRouter initializes Gin router related things.
func (ms *Server) initializeRouter() {
	// Disable default logger
	gin.DefaultWriter = io.Discard
	ms.ginEngine.MaxMultipartMemory = 1 << 20 // ~1MB

	// Register default middlewares
	ms.ginEngine.Use(middlewares.Recovery(ms.services.errorClient))

	// Global error handler.
	ms.ginEngine.Use(errorhandler.Middleware(errorhandler.DefaultErrorMapping, errorhandler.WithFallback(apperrors.GenericInternalServerError)))

	corsConfig := cors.DefaultConfig()
	corsConfig.AddAllowHeaders("Authorization")
	corsConfig.AllowMethods = []string{"POST"}
	corsConfig.AllowOrigins = []string{"*"}

	if !ms.cfg.IsDevEnv {
		corsConfig.AllowOrigins = []string{"https://api.github.com"}
	}

	ms.ginEngine.Use(
		cors.New(corsConfig),
		logger.LogContextMiddleware(),
	)

	// Enables context propagation to the http handlers, to allow
	// proper telemetry span nesting (also important when not using telemetry so that logs are in the
	// same trace)
	ms.ginEngine.ContextWithFallback = true

	authRouter := ms.ginEngine.Group(marvinroute.BasePath)

	ms.cfg.MountProbes(ms.ginEngine,
		func(c *gin.Context) { c.Status(http.StatusOK) },
		func(c *gin.Context) { c.Status(http.StatusOK) })

	for _, controller := range ms.ctrls {
		controller.RouteEndpoints(authRouter.Group(route.RoutingPathEndpointsPrefix))
		controller.RouteWebhooks(authRouter.Group(route.RoutingPathWebHooksPrefix))
	}
}

// initializeControllers initializes all HTTP controllers.
func (ms *Server) initializeControllers() {
	ms.ctrls = []web.Controller{
		NewController(ms.cfg, ms.services.MarvinService),
	}
}

func (ms *Server) Close(ctx context.Context) {
	log := logger.WithContext(ctx).WithPrefix("[marvin.Close]")

	// It might be nil because this service is optional
	if ms.services.errorClient != nil {
		if err := ms.services.errorClient.Close(); err != nil {
			log.Criticalf("error shutting down error reporting client: %v", err)
		}
	}
}
