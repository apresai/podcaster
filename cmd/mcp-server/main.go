package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/apresai/podcaster/internal/mcpserver"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.Println("Podcaster MCP Server starting...")

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	cfg := mcpserver.DefaultConfig()

	srv, err := mcpserver.New(ctx, cfg)
	if err != nil {
		log.Fatalf("Failed to create server: %v", err)
	}

	go func() {
		<-ctx.Done()
		log.Println("Shutdown signal received")
		os.Exit(0)
	}()

	if err := srv.Start(); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
