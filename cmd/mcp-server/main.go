package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/apresai/podcaster/internal/mcpserver"
	"github.com/apresai/podcaster/internal/observability"
)

func main() {
	logger := observability.InitLogger()

	logger.Info("Podcaster MCP Server starting...")

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	tp, err := observability.InitTracer(ctx, "podcaster-mcp", "1.0.0")
	if err != nil {
		logger.Warn("Failed to init tracer, continuing without tracing", "error", err)
	} else {
		defer func() {
			if err := tp.Shutdown(context.Background()); err != nil {
				logger.Error("Tracer shutdown error", "error", err)
			}
		}()
	}

	cfg := mcpserver.DefaultConfig()

	srv, err := mcpserver.New(ctx, cfg, logger)
	if err != nil {
		logger.Error("Failed to create server", "error", err)
		os.Exit(1)
	}

	go func() {
		<-ctx.Done()
		logger.Info("Shutdown signal received")
		os.Exit(0)
	}()

	if err := srv.Start(); err != nil {
		logger.Error("Server error", "error", err)
		os.Exit(1)
	}
}
