package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/apresai/podcaster/internal/mcpserver"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	logger.Info("Podcaster MCP Server starting...")

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

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
