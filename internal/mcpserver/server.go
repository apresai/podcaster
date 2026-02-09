package mcpserver

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/mark3labs/mcp-go/server"
	"go.opentelemetry.io/contrib/instrumentation/github.com/aws/aws-sdk-go-v2/otelaws"
)

// Config holds server configuration.
type Config struct {
	Port                 int
	TableName            string
	S3Bucket             string
	CDNBaseURL           string
	AWSRegion            string
	MaxTasks     int
	SecretPrefix string // e.g. "/podcaster/mcp/"
}

// DefaultConfig returns a Config populated from environment variables.
func DefaultConfig() Config {
	cfg := Config{
		Port:                 8000,
		TableName:            envOr("DYNAMODB_TABLE", "apresai-podcasts-prod"),
		S3Bucket:             envOr("S3_BUCKET", ""),
		CDNBaseURL:           envOr("CDN_BASE_URL", "https://podcasts.apresai.dev"),
		AWSRegion:            envOr("AWS_REGION", "us-east-1"),
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
// Secrets are loaded asynchronously to minimize cold-start latency on AgentCore,
// where the container must have port 8000 listening before AgentCore sends the
// first request. The HTTP listener starts immediately; secrets finish loading
// in the background (typically <1s).
func New(ctx context.Context, cfg Config, logger *slog.Logger) (*Server, error) {
	// Load AWS config
	awsCfg, err := awsconfig.LoadDefaultConfig(ctx,
		awsconfig.WithRegion(cfg.AWSRegion),
	)
	if err != nil {
		return nil, fmt.Errorf("load aws config: %w", err)
	}

	// Auto-instrument AWS SDK calls (DynamoDB, S3, Secrets Manager)
	otelaws.AppendMiddlewares(&awsCfg.APIOptions)

	// Fetch secrets asynchronously — don't block server startup.
	// AgentCore sends the first HTTP request immediately after the container
	// starts, so we must be listening on :8000 ASAP. Secrets are only needed
	// when generate_podcast actually runs the pipeline.
	if cfg.SecretPrefix != "" {
		go func() {
			if err := loadSecrets(ctx, awsCfg, cfg.SecretPrefix, logger); err != nil {
				logger.Warn("Failed to load secrets from Secrets Manager, falling back to env vars",
					"error", err)
			}
		}()
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
	taskMgr := NewTaskManager(store, storage, cfg.MaxTasks, logger, ctx)

	handlers := NewHandlers(taskMgr, store, logger)

	// Create MCP server
	mcpServer := server.NewMCPServer(
		"podcaster",
		"1.0.0",
		server.WithToolCapabilities(true),
	)

	// Register tools
	tools := ToolDefs()
	mcpServer.AddTool(tools[0], handlers.HandleServerInfo)
	mcpServer.AddTool(tools[1], handlers.HandleGeneratePodcast)
	mcpServer.AddTool(tools[2], handlers.HandleGetPodcast)
	mcpServer.AddTool(tools[3], handlers.HandleListPodcasts)

	return &Server{
		cfg:      cfg,
		mcp:      mcpServer,
		handlers: handlers,
		log:      logger,
	}, nil
}

// Start runs the HTTP MCP server.
// Uses a custom mux with request logging so we can debug AgentCore request
// routing. The StreamableHTTPServer is mounted at /mcp and used as a handler.
func (s *Server) Start() error {
	addr := fmt.Sprintf(":%d", s.cfg.Port)
	s.log.Info("Starting MCP server", "addr", addr)

	store := s.handlers.store

	mcpHandler := server.NewStreamableHTTPServer(s.mcp,
		server.WithStateLess(true), // AgentCore manages session IDs
		server.WithHTTPContextFunc(func(ctx context.Context, r *http.Request) context.Context {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				// No auth header — anonymous mode (local dev)
				return WithAuthResult(ctx, AuthResult{Authenticated: false})
			}

			token := strings.TrimPrefix(authHeader, "Bearer ")
			if token == authHeader {
				// No "Bearer " prefix
				return WithAuthResult(ctx, AuthResult{Authenticated: false, Error: fmt.Errorf("invalid authorization format, expected: Bearer <api-key>")})
			}

			info, err := store.ValidateAPIKey(ctx, authHeader)
			if err != nil {
				s.log.WarnContext(ctx, "API key validation failed", "error", err)
				return WithAuthResult(ctx, AuthResult{Authenticated: false, Error: err})
			}

			s.log.InfoContext(ctx, "Authenticated request", "user_id", info.UserID, "key_id", info.KeyID)
			return WithAuthResult(ctx, AuthResult{
				Authenticated: true,
				UserID:        info.UserID,
				Role:          info.Role,
				KeyID:         info.KeyID,
			})
		}),
	)

	mux := http.NewServeMux()
	// Register both /mcp and /mcp/ — AgentCore sends POST to /mcp/ (trailing
	// slash) and Go's ServeMux won't match /mcp for /mcp/ POST requests.
	mux.Handle("/mcp", mcpHandler)
	mux.Handle("/mcp/", mcpHandler)

	// Wrap with middleware that ensures Content-Type is set. AgentCore may not
	// send Content-Type: application/json, which causes mcp-go to reject with
	// 400 Bad Request. Also logs requests for debugging.
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.log.Info("HTTP request",
			"method", r.Method,
			"path", r.URL.Path,
			"content_type", r.Header.Get("Content-Type"),
		)
		// Ensure Content-Type is set for POST requests — mcp-go requires
		// application/json and rejects requests without it.
		if r.Method == http.MethodPost && r.Header.Get("Content-Type") == "" {
			r.Header.Set("Content-Type", "application/json")
		}
		mux.ServeHTTP(w, r)
	})

	httpSrv := &http.Server{
		Addr:    addr,
		Handler: handler,
	}
	return httpSrv.ListenAndServe()
}

// loadSecrets fetches API keys from Secrets Manager and sets them as env vars.
func loadSecrets(ctx context.Context, cfg aws.Config, prefix string, logger *slog.Logger) error {
	client := secretsmanager.NewFromConfig(cfg)

	secrets := map[string]string{
		"ANTHROPIC_API_KEY":  prefix + "ANTHROPIC_API_KEY",
		"GEMINI_API_KEY":     prefix + "GEMINI_API_KEY",
		"ELEVENLABS_API_KEY": prefix + "ELEVENLABS_API_KEY",
		"VERTEX_AI_API_KEY":  prefix + "VERTEX_AI_API_KEY",
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
