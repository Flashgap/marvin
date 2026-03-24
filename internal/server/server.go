package server

import (
	"context"
	"errors"
	"net"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/Flashgap/marvin/internal/constants"
	"github.com/Flashgap/marvin/pkg/logger"
	"github.com/Flashgap/marvin/pkg/option"
)

// NewServerContext creates a cancellable context to be used for servers.
func NewServerContext(mainCtx context.Context) (context.Context, context.CancelFunc) {
	return signal.NotifyContext(mainCtx, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
}

// NewShutdownContext creates a cancellable context to be used for graceful shutdown.
func NewShutdownContext(mainCtx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(mainCtx, constants.ShutdownTimeout)
}

type Options struct {
	listener net.Listener
}

type Option = option.Option[Options]

// HTTP is an HTTP server that can be run and gracefully shutdown.
type HTTP struct {
	*http.Server
	options Options
}

// NewServer creates a new server with the given http.Server and options.
func NewServer(httpServer *http.Server, o ...option.Option[Options]) *HTTP {
	opts := option.New(o)

	return &HTTP{
		Server:  httpServer,
		options: opts,
	}
}

// Run runs an http server and takes care to listen for
// termination signals to gracefully close it.
func (s *HTTP) Run(ctx context.Context, stop context.CancelFunc) {
	defer stop()

	log := logger.WithContext(ctx).WithPrefix("[Server.Run]")
	log.Infof("Running server on %s", s.Addr)

	// Initializing the server in a goroutine so that
	// it won't block the graceful shutdown handling below
	go func() {
		ln := s.options.listener
		if ln == nil {
			var err error
			ln, err = net.Listen("tcp", s.Addr)
			if err != nil {
				log.Criticalf("Failed to listen on %s: %v", s.Addr, err)
				return
			}
		}

		if err := s.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Criticalf("Server shutdown with error: %v", err)
		}
	}()

	// Listen for the interrupt signal.
	<-ctx.Done()

	// Restore default behavior on the interrupt signal and notify user of shutdown.
	stop()
	log.Info("shutting down gracefully, press Ctrl+C again to force")

	// The context is used to inform the server it has 30 seconds to finish
	// the request it is currently handling
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := s.Shutdown(ctx); err != nil {
		log.Critical("Server forced to shutdown: ", err)
	}

	log.Info("Server exiting")
}
