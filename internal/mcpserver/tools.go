package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/mark3labs/mcp-go/mcp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
)

var tracer = otel.Tracer("podcaster-mcp")

// ToolDefs returns the MCP tool definitions.
func ToolDefs() []mcp.Tool {
	return []mcp.Tool{
		{
			Name:        "generate_podcast",
			Description: "Generate a podcast episode from a URL or text input. Starts an async task and returns a task ID. Use get_podcast to check progress.",
			InputSchema: mcp.ToolInputSchema{
				Type: "object",
				Properties: map[string]any{
					"input_url": map[string]any{
						"type":        "string",
						"description": "URL of content to convert into a podcast",
					},
					"input_text": map[string]any{
						"type":        "string",
						"description": "Raw text to convert into a podcast (alternative to input_url)",
					},
					"model": map[string]any{
						"type":        "string",
						"description": "Script generation model: haiku, sonnet, gemini-flash, gemini-pro",
						"default":     "haiku",
					},
					"tts": map[string]any{
						"type":        "string",
						"description": "Text-to-speech provider: gemini, elevenlabs, google",
						"default":     "gemini",
					},
					"tone": map[string]any{
						"type":        "string",
						"description": "Conversation tone: casual, technical, educational",
						"default":     "casual",
					},
					"duration": map[string]any{
						"type":        "string",
						"description": "Episode length: short (~8min), standard (~18min), long (~35min), deep (~55min)",
						"default":     "standard",
					},
					"format": map[string]any{
						"type":        "string",
						"description": "Show format: conversation, interview, deep-dive, explainer, debate, news, storytelling, challenger",
						"default":     "conversation",
					},
					"voices": map[string]any{
						"type":        "integer",
						"description": "Number of hosts (1-3)",
						"default":     2,
					},
					"topic": map[string]any{
						"type":        "string",
						"description": "Focus topic to emphasize in the conversation",
					},
					"anthropic_api_key": map[string]any{
						"type":        "string",
						"description": "Your Anthropic API key (required for haiku/sonnet models if server has no default key)",
					},
					"gemini_api_key": map[string]any{
						"type":        "string",
						"description": "Your Gemini API key (required for gemini-flash/pro models or gemini TTS if server has no default key)",
					},
					"elevenlabs_api_key": map[string]any{
						"type":        "string",
						"description": "Your ElevenLabs API key (required for elevenlabs TTS if server has no default key)",
					},
				},
			},
		},
		{
			Name:        "get_podcast",
			Description: "Get the status and details of a podcast by ID. Use this to check on a running generation or retrieve a completed podcast's audio URL.",
			InputSchema: mcp.ToolInputSchema{
				Type: "object",
				Properties: map[string]any{
					"podcast_id": map[string]any{
						"type":        "string",
						"description": "The podcast ID returned from generate_podcast",
					},
				},
				Required: []string{"podcast_id"},
			},
		},
		{
			Name:        "list_podcasts",
			Description: "List all generated podcasts, newest first. Returns podcast IDs, titles, status, and audio URLs.",
			InputSchema: mcp.ToolInputSchema{
				Type: "object",
				Properties: map[string]any{
					"limit": map[string]any{
						"type":        "integer",
						"description": "Maximum number of results (default 20)",
						"default":     20,
					},
					"cursor": map[string]any{
						"type":        "string",
						"description": "Pagination cursor from a previous list_podcasts call",
					},
				},
			},
		},
	}
}

// Handlers contains tool handler implementations.
type Handlers struct {
	tasks *TaskManager
	store *Store
	log   *slog.Logger
}

// NewHandlers creates tool handlers.
func NewHandlers(tasks *TaskManager, store *Store, logger *slog.Logger) *Handlers {
	return &Handlers{tasks: tasks, store: store, log: logger}
}

// HandleGeneratePodcast starts a podcast generation task.
func (h *Handlers) HandleGeneratePodcast(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	ctx, span := tracer.Start(ctx, "tool.generate_podcast")
	defer span.End()

	genReq := GenerateRequest{
		InputURL:         mcp.ParseString(req, "input_url", ""),
		InputText:        mcp.ParseString(req, "input_text", ""),
		Model:            mcp.ParseString(req, "model", "haiku"),
		TTS:              mcp.ParseString(req, "tts", "gemini"),
		Tone:             mcp.ParseString(req, "tone", "casual"),
		Duration:         mcp.ParseString(req, "duration", "standard"),
		Format:           mcp.ParseString(req, "format", "conversation"),
		Voices:           parseIntParam(req, "voices", 2),
		Topic:            mcp.ParseString(req, "topic", ""),
		AnthropicAPIKey:  mcp.ParseString(req, "anthropic_api_key", ""),
		GeminiAPIKey:     mcp.ParseString(req, "gemini_api_key", ""),
		ElevenLabsAPIKey: mcp.ParseString(req, "elevenlabs_api_key", ""),
		Owner:            "mcp-server",
	}

	span.SetAttributes(
		attribute.String("input_url", genReq.InputURL),
		attribute.String("model", genReq.Model),
		attribute.String("tts", genReq.TTS),
		attribute.String("duration", genReq.Duration),
		attribute.String("format", genReq.Format),
		attribute.Int("voices", genReq.Voices),
	)

	if genReq.InputURL == "" && genReq.InputText == "" {
		span.SetStatus(codes.Error, "missing input")
		return mcp.NewToolResultError("either input_url or input_text is required"), nil
	}

	id, err := h.tasks.StartTask(ctx, genReq)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "start task failed")
		return mcp.NewToolResultError(fmt.Sprintf("failed to start task: %v", err)), nil
	}

	span.SetAttributes(attribute.String("podcast_id", id))
	h.log.InfoContext(ctx, "Podcast generation started", "podcast_id", id, "model", genReq.Model, "tts", genReq.TTS)

	result := map[string]any{
		"podcast_id": id,
		"status":     "submitted",
		"message":    "Podcast generation started. Use get_podcast with this podcast_id to check progress.",
	}
	return jsonResult(result)
}

// HandleGetPodcast returns podcast details.
func (h *Handlers) HandleGetPodcast(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	ctx, span := tracer.Start(ctx, "tool.get_podcast")
	defer span.End()

	id := mcp.ParseString(req, "podcast_id", "")
	if id == "" {
		span.SetStatus(codes.Error, "missing podcast_id")
		return mcp.NewToolResultError("podcast_id is required"), nil
	}

	span.SetAttributes(attribute.String("podcast_id", id))

	item, err := h.store.GetPodcast(ctx, id)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "get podcast failed")
		return mcp.NewToolResultError(fmt.Sprintf("failed to get podcast: %v", err)), nil
	}
	if item == nil {
		span.SetStatus(codes.Error, "not found")
		return mcp.NewToolResultError(fmt.Sprintf("podcast %s not found", id)), nil
	}

	result := map[string]any{
		"podcast_id":       item.PodcastID,
		"status":           item.Status,
		"progress_percent": item.ProgressPercent,
		"stage_message":    item.StageMessage,
		"created_at":       item.CreatedAt,
	}

	if item.Title != "" {
		result["title"] = item.Title
	}
	if item.Summary != "" {
		result["summary"] = item.Summary
	}
	if item.AudioURL != "" {
		result["audio_url"] = item.AudioURL
	}
	if item.Duration != "" {
		result["duration"] = item.Duration
	}
	if item.FileSizeMB > 0 {
		result["file_size_mb"] = item.FileSizeMB
	}
	if item.ErrorMessage != "" {
		result["error"] = item.ErrorMessage
	}
	if item.Model != "" {
		result["model"] = item.Model
	}
	if item.TTSProvider != "" {
		result["tts_provider"] = item.TTSProvider
	}
	if item.Format != "" {
		result["format"] = item.Format
	}
	if item.PlayCount > 0 {
		result["play_count"] = item.PlayCount
	}

	return jsonResult(result)
}

// HandleListPodcasts returns a paginated list of podcasts.
func (h *Handlers) HandleListPodcasts(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	ctx, span := tracer.Start(ctx, "tool.list_podcasts")
	defer span.End()

	limit := parseIntParam(req, "limit", 20)
	cursor := mcp.ParseString(req, "cursor", "")

	span.SetAttributes(
		attribute.Int("limit", limit),
		attribute.String("cursor", cursor),
	)

	items, nextCursor, err := h.store.ListPodcasts(ctx, limit, cursor)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "list podcasts failed")
		return mcp.NewToolResultError(fmt.Sprintf("failed to list podcasts: %v", err)), nil
	}

	span.SetAttributes(attribute.Int("result_count", len(items)))

	podcasts := make([]map[string]any, 0, len(items))
	for _, item := range items {
		p := map[string]any{
			"podcast_id": item.PodcastID,
			"status":     item.Status,
			"created_at": item.CreatedAt,
		}
		if item.Title != "" {
			p["title"] = item.Title
		}
		if item.AudioURL != "" {
			p["audio_url"] = item.AudioURL
		}
		if item.Duration != "" {
			p["duration"] = item.Duration
		}
		if item.PlayCount > 0 {
			p["play_count"] = item.PlayCount
		}
		podcasts = append(podcasts, p)
	}

	result := map[string]any{
		"podcasts": podcasts,
		"count":    len(podcasts),
	}
	if nextCursor != "" {
		result["next_cursor"] = nextCursor
	}

	return jsonResult(result)
}

func jsonResult(v any) (*mcp.CallToolResult, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("marshal result: %v", err)), nil
	}
	return mcp.NewToolResultText(string(data)), nil
}

func parseIntParam(req mcp.CallToolRequest, key string, defaultVal int) int {
	args := req.GetArguments()
	if args == nil {
		return defaultVal
	}
	raw, ok := args[key]
	if !ok {
		return defaultVal
	}
	switch v := raw.(type) {
	case float64:
		return int(v)
	case int:
		return v
	default:
		return defaultVal
	}
}
