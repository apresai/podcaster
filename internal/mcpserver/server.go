package mcpserver

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/mark3labs/mcp-go/server"
)

// Config holds server configuration.
type Config struct {
	Port         int
	TableName    string
	S3Bucket     string
	CDNBaseURL   string
	AWSRegion    string
	MaxTasks     int
	SecretPrefix string // e.g. "/podcaster/mcp/"
}

// DefaultConfig returns a Config populated from environment variables.
func DefaultConfig() Config {
	cfg := Config{
		Port:         8000,
		TableName:    envOr("DYNAMODB_TABLE", "apresai-podcasts-prod"),
		S3Bucket:     envOr("S3_BUCKET", ""),
		CDNBaseURL:   envOr("CDN_BASE_URL", "https://podcasts.apresai.dev"),
		AWSRegion:    envOr("AWS_REGION", "us-east-1"),
		MaxTasks:     5,
		SecretPrefix: envOr("SECRET_PREFIX", "/podcaster/mcp/"),
	}
	return cfg
}

// Server is the MCP server for podcast generation.
type Server struct {
	cfg      Config
	mcp      *server.MCPServer
	handlers *Handlers
	log      *slog.Logger
}

// New creates and configures the MCP server.
func New(ctx context.Context, cfg Config, logger *slog.Logger) (*Server, error) {
	// Load AWS config
	awsCfg, err := awsconfig.LoadDefaultConfig(ctx,
		awsconfig.WithRegion(cfg.AWSRegion),
	)
	if err != nil {
		return nil, fmt.Errorf("load aws config: %w", err)
	}

	// Fetch secrets if running in AWS
	if cfg.SecretPrefix != "" {
		if err := loadSecrets(ctx, awsCfg, cfg.SecretPrefix, logger); err != nil {
			logger.Warn("Failed to load secrets from Secrets Manager, falling back to env vars",
				"error", err)
		}
	}

	if cfg.S3Bucket == "" {
		return nil, fmt.Errorf("S3_BUCKET environment variable is required")
	}

	// Create AWS clients
	ddbClient := dynamodb.NewFromConfig(awsCfg)
	s3Client := s3.NewFromConfig(awsCfg)

	// Create store, storage, task manager
	store := NewStore(ddbClient, cfg.TableName)
	storage := NewStorage(s3Client, cfg.S3Bucket, cfg.CDNBaseURL)
	taskMgr := NewTaskManager(store, storage, cfg.MaxTasks, logger)
	handlers := NewHandlers(taskMgr, store, logger)

	// Create MCP server
	mcpServer := server.NewMCPServer(
		"podcaster",
		"1.0.0",
		server.WithToolCapabilities(true),
	)

	// Register tools
	tools := ToolDefs()
	mcpServer.AddTool(tools[0], handlers.HandleGeneratePodcast)
	mcpServer.AddTool(tools[1], handlers.HandleGetPodcast)
	mcpServer.AddTool(tools[2], handlers.HandleListPodcasts)

	return &Server{
		cfg:      cfg,
		mcp:      mcpServer,
		handlers: handlers,
		log:      logger,
	}, nil
}

// Start runs the HTTP MCP server.
func (s *Server) Start() error {
	addr := fmt.Sprintf(":%d", s.cfg.Port)
	s.log.Info("Starting MCP server", "addr", addr)

	httpServer := server.NewStreamableHTTPServer(s.mcp,
		server.WithStateLess(true), // AgentCore manages session IDs
	)
	return httpServer.Start(addr)
}

// loadSecrets fetches API keys from Secrets Manager and sets them as env vars.
func loadSecrets(ctx context.Context, cfg aws.Config, prefix string, logger *slog.Logger) error {
	client := secretsmanager.NewFromConfig(cfg)

	secrets := map[string]string{
		"ANTHROPIC_API_KEY":  prefix + "ANTHROPIC_API_KEY",
		"GEMINI_API_KEY":     prefix + "GEMINI_API_KEY",
		"ELEVENLABS_API_KEY": prefix + "ELEVENLABS_API_KEY",
	}

	for envVar, secretID := range secrets {
		// Skip if already set in environment
		if os.Getenv(envVar) != "" {
			continue
		}

		result, err := client.GetSecretValue(ctx, &secretsmanager.GetSecretValueInput{
			SecretId: &secretID,
		})
		if err != nil {
			logger.Info("Secret not found", "secret_id", secretID, "error", err)
			continue
		}
		if result.SecretString != nil {
			os.Setenv(envVar, *result.SecretString)
			logger.Info("Loaded secret", "secret_id", secretID)
		}
	}

	return nil
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
