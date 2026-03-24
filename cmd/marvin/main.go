package main

import (
	"context"

	"github.com/Flashgap/logrus"

	"github.com/Flashgap/marvin/internal/config"
	"github.com/Flashgap/marvin/internal/microservice/marvin"
	"github.com/Flashgap/marvin/internal/server"
	"github.com/Flashgap/marvin/pkg/logger"
)

func main() {
	mainCtx := context.Background()
	ctx, stop := server.NewServerContext(mainCtx)
	cfg := marvin.NewConfig()
	logger.Init(cfg.LoggerConfig())
	config.PrintConfig(cfg)

	srv, err := marvin.NewServer(ctx, cfg)
	if err != nil {
		logrus.Fatal(err)
	}
	defer func() {
		shutdownCtx, _ := server.NewShutdownContext(mainCtx)
		srv.Close(shutdownCtx)
	}()
	srv.Run(ctx, stop)
}
