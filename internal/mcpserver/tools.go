package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/apresai/podcaster/internal/ingest"
	"github.com/apresai/podcaster/internal/tts"
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
			Name:        "server_info",
			Description: "Returns server runtime information and diagnostics. Useful for debugging.",
			InputSchema: mcp.ToolInputSchema{
				Type:       "object",
				Properties: map[string]any{},
			},
		},
		{
			Name:        "generate_podcast",
			Description: "Generate a podcast episode from a URL or text input. Starts async pipeline (content ingestion, script generation, text-to-speech synthesis, audio assembly) and returns a podcast_id immediately. Use get_podcast to poll for progress and the completed result with an audio_url link to the MP3 file. Generation takes 3-8 minutes depending on duration setting. Always poll get_podcast until status is 'complete', then show the audio_url link to the user. Use list_voices to discover available voice IDs and list_options to see all formats, styles, and providers.",
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
						"description": "Script generation LLM that writes the conversation. Always use haiku unless the user specifically asks for a different model. Options: haiku (default, Claude Haiku 4.5), sonnet (Claude Sonnet 4.5), gemini-flash (Gemini 2.5 Flash), gemini-pro (Gemini 2.5 Pro)",
						"default":     "haiku",
					},
					"tts": map[string]any{
						"type":        "string",
						"description": "Text-to-speech provider that synthesizes audio: gemini (default), gemini-vertex, vertex-express, elevenlabs, google",
						"default":     "gemini",
					},
					"tone": map[string]any{
						"type":        "string",
						"description": "Conversation tone: casual, technical, educational",
						"default":     "casual",
					},
					"duration": map[string]any{
						"type":        "string",
						"description": "Episode length: short (~3-4min), standard (~8-10min), long (~15min), deep (~30-35min)",
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
					"style": map[string]any{
						"type":        "string",
						"description": "Conversation styles (comma-separated): humor, wow, serious, debate, storytelling",
					},
					"voice1": map[string]any{
						"type":        "string",
						"description": "Voice ID for host 1. Use list_voices to see available IDs. Format: plain ID (e.g. 'Kore') or 'provider:ID' for cross-provider mixing (e.g. 'elevenlabs:rachel').",
					},
					"voice2": map[string]any{
						"type":        "string",
						"description": "Voice ID for host 2. Same format as voice1.",
					},
					"voice3": map[string]any{
						"type":        "string",
						"description": "Voice ID for host 3. Same format as voice1.",
					},
					"tts_model": map[string]any{
						"type":        "string",
						"description": "TTS model override (e.g. eleven_v3, gemini-2.5-pro-tts). Leave empty for provider default.",
					},
					"tts_speed": map[string]any{
						"type":        "number",
						"description": "Speech speed (ElevenLabs: 0.7-1.2, Google: 0.25-2.0). Not supported by Gemini providers.",
					},
					"tts_stability": map[string]any{
						"type":        "number",
						"description": "Voice stability, ElevenLabs only (0.0-1.0, default 0.5).",
					},
					"tts_pitch": map[string]any{
						"type":        "number",
						"description": "Pitch in semitones, Google Cloud TTS only (-20.0 to 20.0).",
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
			Description: "Get the status and details of a podcast by ID. Use this to check on a running generation or retrieve a completed podcast. Completed podcasts include an audio_url with a direct MP3 link — always show this link to the user.",
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
			Description: "List all generated podcasts, newest first. Each completed podcast includes an audio_url field with a direct link to the MP3 file that users can click to listen. Always show the audio_url link for completed podcasts.",
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
		{
			Name:        "list_voices",
			Description: "List available TTS voices for a provider. Returns voice IDs that can be used with voice1/voice2/voice3 params in generate_podcast.",
			InputSchema: mcp.ToolInputSchema{
				Type: "object",
				Properties: map[string]any{
					"provider": map[string]any{
						"type":        "string",
						"description": "TTS provider name: gemini, vertex-express, gemini-vertex, elevenlabs, google",
					},
				},
				Required: []string{"provider"},
			},
		},
		{
			Name:        "list_options",
			Description: "List all available options for podcast generation: show formats, conversation styles, TTS providers, script models, and durations.",
			InputSchema: mcp.ToolInputSchema{
				Type:       "object",
				Properties: map[string]any{},
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

	// Resolve user identity from either:
	// 1. HTTP auth context (direct access with Authorization header)
	// 2. Proxy-injected _user_id/_key_id in tool arguments (Lambda proxy flow)
	auth := AuthFromContext(ctx)
	userID := ""
	keyID := ""

	if auth.Authenticated {
		userID = auth.UserID
		keyID = auth.KeyID
	} else {
		// Check for proxy-injected auth in arguments
		args := req.GetArguments()
		if uid, ok := args["_user_id"].(string); ok && uid != "" {
			userID = uid
		}
		if kid, ok := args["_key_id"].(string); ok && kid != "" {
			keyID = kid
		}
	}

	// Require auth when running on AWS (SECRET_PREFIX is set)
	if userID == "" && os.Getenv("SECRET_PREFIX") != "" {
		if auth.Error != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Authentication failed: %v. Provide your API key as: Authorization: Bearer <your-api-key>. Get an API key at https://podcasts.apresai.dev", auth.Error)), nil
		}
		return mcp.NewToolResultError("Authentication required. Provide your API key as: Authorization: Bearer <your-api-key>. Get an API key at https://podcasts.apresai.dev"), nil
	}

	_ = keyID // used for logging if needed
	owner := "anonymous"
	if userID != "" {
		owner = userID
	}

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
		Style:            mcp.ParseString(req, "style", ""),
		Voice1:           mcp.ParseString(req, "voice1", ""),
		Voice2:           mcp.ParseString(req, "voice2", ""),
		Voice3:           mcp.ParseString(req, "voice3", ""),
		TTSModel:         mcp.ParseString(req, "tts_model", ""),
		TTSSpeed:         parseFloatParam(req, "tts_speed", 0),
		TTSStability:     parseFloatParam(req, "tts_stability", 0),
		TTSPitch:         parseFloatParam(req, "tts_pitch", 0),
		AnthropicAPIKey:  mcp.ParseString(req, "anthropic_api_key", ""),
		GeminiAPIKey:     mcp.ParseString(req, "gemini_api_key", ""),
		ElevenLabsAPIKey: mcp.ParseString(req, "elevenlabs_api_key", ""),
		Owner:            owner,
		UserID:           userID,
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

	// Validate URL content synchronously before starting async task.
	// This catches unfetchable URLs and insufficient content immediately,
	// so the LLM client can ask the user for input_text or a different URL.
	if genReq.InputURL != "" {
		valCtx, valCancel := context.WithTimeout(ctx, 60*time.Second)
		defer valCancel()
		if err := ingest.ValidateURL(valCtx, genReq.InputURL); err != nil {
			span.SetStatus(codes.Error, "url validation failed")
			span.RecordError(err)
			h.log.WarnContext(ctx, "URL validation failed", "url", genReq.InputURL, "error", err)
			return mcp.NewToolResultError(fmt.Sprintf(
				"Could not use this URL for podcast generation. %v. "+
					"Please provide the content directly using input_text, or try a different URL.",
				err,
			)), nil
		}
	}

	h.log.InfoContext(ctx, "Starting podcast generation", "model", genReq.Model, "tts", genReq.TTS)

	id, err := h.tasks.StartTask(ctx, genReq)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "start task failed")
		return mcp.NewToolResultError(fmt.Sprintf("Failed to start generation: %v", err)), nil
	}

	span.SetAttributes(attribute.String("podcast_id", id))
	h.log.InfoContext(ctx, "Podcast generation started", "podcast_id", id)

	result := map[string]any{
		"podcast_id": id,
		"status":     "submitted",
		"message":    "Podcast generation started. Use get_podcast to check progress.",
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
	if item.ScriptURL != "" {
		result["script_url"] = item.ScriptURL
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
		if item.ScriptURL != "" {
			p["script_url"] = item.ScriptURL
		}
		if item.Duration != "" {
			p["duration"] = item.Duration
		}
		if item.Model != "" {
			p["model"] = item.Model
		}
		if item.TTSProvider != "" {
			p["tts_provider"] = item.TTSProvider
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

// HandleServerInfo returns runtime diagnostics.
func (h *Handlers) HandleServerInfo(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Collect OTEL-related env vars (redact sensitive values)
	otelVars := map[string]string{}
	for _, env := range os.Environ() {
		parts := strings.SplitN(env, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := parts[0]
		if strings.HasPrefix(key, "OTEL_") || strings.HasPrefix(key, "AWS_") ||
			key == "SECRET_PREFIX" ||
			key == "S3_BUCKET" || key == "DYNAMODB_TABLE" ||
			key == "CDN_BASE_URL" || key == "DISABLE_ADOT_OBSERVABILITY" ||
			key == "HOME" || key == "PORT" || key == "PATH" {
			otelVars[key] = parts[1]
		}
	}

	// Check local OTEL collector connectivity
	otelPorts := map[string]string{
		"grpc_4317": "localhost:4317",
		"http_4318": "localhost:4318",
	}
	portStatus := map[string]string{}
	for name, addr := range otelPorts {
		conn, err := net.DialTimeout("tcp", addr, 2*time.Second)
		if err != nil {
			portStatus[name] = fmt.Sprintf("CLOSED (%v)", err)
		} else {
			conn.Close()
			portStatus[name] = "OPEN"
		}
	}

	result := map[string]any{
		"go_version":    runtime.Version(),
		"arch":          runtime.GOARCH,
		"os":            runtime.GOOS,
		"num_goroutine": runtime.NumGoroutine(),
		"env_vars":      otelVars,
		"otel_ports":    portStatus,
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

func parseFloatParam(req mcp.CallToolRequest, key string, defaultVal float64) float64 {
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
		return v
	case int:
		return float64(v)
	default:
		return defaultVal
	}
}

// HandleListVoices returns available voices for a TTS provider.
func (h *Handlers) HandleListVoices(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	provider := mcp.ParseString(req, "provider", "")
	if provider == "" {
		return mcp.NewToolResultError("provider is required"), nil
	}

	voices, err := tts.AvailableVoices(provider)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("unknown provider %q: must be gemini, vertex-express, gemini-vertex, elevenlabs, or google", provider)), nil
	}

	voiceList := make([]map[string]any, 0, len(voices))
	for _, v := range voices {
		entry := map[string]any{
			"id":          v.ID,
			"name":        v.Name,
			"gender":      v.Gender,
			"description": v.Description,
		}
		if v.DefaultFor != "" {
			entry["default_for"] = v.DefaultFor
		}
		voiceList = append(voiceList, entry)
	}

	result := map[string]any{
		"provider": provider,
		"voices":   voiceList,
		"count":    len(voiceList),
	}
	return jsonResult(result)
}

// HandleListOptions returns all available generation options.
func (h *Handlers) HandleListOptions(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	result := map[string]any{
		"formats": []map[string]any{
			{"name": "conversation", "description": "Casual back-and-forth discussion"},
			{"name": "interview", "description": "Structured Q&A with interviewer and expert(s)"},
			{"name": "deep-dive", "description": "Investigative deep dive, layered evidence"},
			{"name": "explainer", "description": "Educational explainer, progressive complexity"},
			{"name": "debate", "description": "Point-counterpoint with opposing positions"},
			{"name": "news", "description": "News briefing, single-story deep coverage"},
			{"name": "storytelling", "description": "Narrative arc with tension and resolution"},
			{"name": "challenger", "description": "Devil's advocate stress-testing ideas"},
		},
		"styles": []map[string]any{
			{"name": "humor", "description": "Witty banter, clever one-liners, running jokes"},
			{"name": "wow", "description": "Build-up to dramatic reveals, surprise reactions"},
			{"name": "serious", "description": "Measured, analytical, gravity-weighted tone"},
			{"name": "debate", "description": "Push-back, challenge assumptions, dialectical"},
			{"name": "storytelling", "description": "Narrative threads, callbacks, scene-setting"},
		},
		"tts_providers": []map[string]any{
			{"name": "gemini", "auth": "API key (GEMINI_API_KEY)", "rate_limit": "10 RPM, 100 RPD", "voices": "30 Gemini voices"},
			{"name": "vertex-express", "auth": "API key (VERTEX_AI_API_KEY)", "rate_limit": "Higher than AI Studio", "voices": "Same 30 Gemini voices"},
			{"name": "gemini-vertex", "auth": "GCP ADC/service account", "rate_limit": "30,000 RPM", "voices": "Same 30 Gemini voices"},
			{"name": "elevenlabs", "auth": "API key (ELEVENLABS_API_KEY)", "rate_limit": "Varies by plan", "voices": "10+ ElevenLabs voices"},
			{"name": "google", "auth": "GCP ADC/service account", "rate_limit": "150 RPM", "voices": "8 Chirp 3 HD voices"},
		},
		"models": []map[string]any{
			{"name": "haiku", "provider": "Anthropic", "description": "Claude Haiku 4.5 (fastest, default)"},
			{"name": "sonnet", "provider": "Anthropic", "description": "Claude Sonnet 4.5"},
			{"name": "gemini-flash", "provider": "Google", "description": "Gemini 2.5 Flash"},
			{"name": "gemini-pro", "provider": "Google", "description": "Gemini 2.5 Pro"},
		},
		"durations": []map[string]any{
			{"name": "short", "description": "~3-4 minutes, ~15 segments"},
			{"name": "standard", "description": "~8-10 minutes, ~40 segments"},
			{"name": "long", "description": "~15 minutes, ~65 segments"},
			{"name": "deep", "description": "~30-35 minutes, ~150 segments"},
		},
	}
	return jsonResult(result)
}
