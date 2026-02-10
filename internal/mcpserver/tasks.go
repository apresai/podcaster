package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/apresai/podcaster/internal/observability"
	"github.com/apresai/podcaster/internal/pipeline"
	"github.com/apresai/podcaster/internal/progress"
	"github.com/apresai/podcaster/internal/script"
	"github.com/apresai/podcaster/internal/tts"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// GenerateRequest holds parameters for a podcast generation task.
type GenerateRequest struct {
	InputURL  string
	InputText string
	Model     string
	TTS       string
	Tone      string
	Duration  string
	Format    string
	Voices    int
	Topic     string
	Owner     string
	UserID    string // authenticated user ID (empty for anonymous)

	// Voice and style options
	Style        string  // comma-separated styles: humor, wow, serious, debate, storytelling
	Voice1       string  // voice spec: plain ID or "provider:ID"
	Voice2       string
	Voice3       string
	TTSModel     string  // TTS model override (e.g. eleven_v3, gemini-2.5-pro-tts)
	TTSSpeed     float64 // speech speed (ElevenLabs: 0.7-1.2, Google: 0.25-2.0)
	TTSStability float64 // voice stability, ElevenLabs only (0.0-1.0)
	TTSPitch     float64 // pitch in semitones, Google only (-20.0 to 20.0)

	// Per-request API key overrides (BYOK). Empty = use server defaults.
	AnthropicAPIKey  string
	GeminiAPIKey     string
	ElevenLabsAPIKey string
}

// TaskManager manages async podcast generation tasks.
type TaskManager struct {
	store   *Store
	storage *Storage
	log     *slog.Logger
	baseCtx context.Context // cancelled on SIGTERM for graceful shutdown

	mu       sync.Mutex
	cancels  map[string]context.CancelFunc
	maxTasks int
	running  int
}

// NewTaskManager creates a task manager.
// baseCtx should be cancelled on SIGTERM so pipeline goroutines can clean up.
func NewTaskManager(store *Store, storage *Storage, maxTasks int, logger *slog.Logger, baseCtx context.Context) *TaskManager {
	if maxTasks <= 0 {
		maxTasks = 5
	}
	return &TaskManager{
		store:    store,
		storage:  storage,
		log:      logger,
		baseCtx:  baseCtx,
		cancels:  make(map[string]context.CancelFunc),
		maxTasks: maxTasks,
	}
}

// StartTask creates a DynamoDB record and starts pipeline.Run in a goroutine.
// Returns the podcast ID immediately.
func (tm *TaskManager) StartTask(ctx context.Context, req GenerateRequest) (string, error) {
	id, err := NewPodcastID()
	if err != nil {
		return "", err
	}

	tm.mu.Lock()
	if tm.running >= tm.maxTasks {
		tm.mu.Unlock()
		return "", fmt.Errorf("max concurrent tasks reached (%d)", tm.maxTasks)
	}
	tm.running++

	// Derive goroutine context from baseCtx (cancelled on SIGTERM) rather than
	// the HTTP request context (cancelled when the response is sent).
	// Carry trace span from the HTTP request for observability linking.
	taskCtx := observability.DetachTraceContextFrom(ctx, tm.baseCtx)
	taskCtx, cancel := context.WithCancel(taskCtx)
	tm.cancels[id] = cancel
	tm.mu.Unlock()

	if err := tm.store.CreateJob(ctx, id, req.Owner, req.InputURL, req.Model, req.TTS, req.Format); err != nil {
		cancel()
		tm.mu.Lock()
		delete(tm.cancels, id)
		tm.running--
		tm.mu.Unlock()
		return "", fmt.Errorf("create job: %w", err)
	}

	go tm.runPipeline(taskCtx, id, req)

	return id, nil
}

// CancelTask cancels a running task.
func (tm *TaskManager) CancelTask(id string) {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	if cancel, ok := tm.cancels[id]; ok {
		cancel()
	}
}

func (tm *TaskManager) runPipeline(ctx context.Context, id string, req GenerateRequest) {
	ctx, span := tracer.Start(ctx, "pipeline.run",
		trace.WithAttributes(attribute.String("podcast_id", id)),
	)
	defer span.End()

	defer func() {
		// On shutdown (SIGTERM), mark any in-progress job as failed so it doesn't
		// appear stuck in "synthesizing" forever.
		if ctx.Err() != nil {
			failCtx, failCancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer failCancel()
			tm.store.FailJob(failCtx, id, "server shutdown during processing")
			tm.log.Info("Marked job as failed due to shutdown", "podcast_id", id)
		}
		tm.mu.Lock()
		delete(tm.cancels, id)
		tm.running--
		tm.mu.Unlock()
	}()

	log := tm.log.With("podcast_id", id)

	// Throttle DynamoDB writes: max 1 per 2 seconds except on stage transitions.
	var lastWrite time.Time
	var lastStage progress.Stage

	progressCb := func(evt progress.Event) {
		now := time.Now()
		stageChanged := evt.Stage != lastStage
		throttled := now.Sub(lastWrite) < 2*time.Second

		if throttled && !stageChanged {
			return
		}

		if stageChanged {
			fmt.Fprintf(os.Stderr, "[%s] stage=%s msg=%s pct=%.2f\n", id, evt.Stage, evt.Message, evt.Percent)
			span.AddEvent("stage_transition",
				trace.WithAttributes(
					attribute.String("stage", evt.Message),
					attribute.Float64("percent", evt.Percent),
				),
			)
		}

		status := mapStage(evt.Stage)
		if err := tm.store.UpdateProgress(ctx, id, status, evt.Percent, evt.Message); err != nil {
			log.WarnContext(ctx, "Update progress failed", "error", err)
		}
		lastWrite = now
		lastStage = evt.Stage
	}

	// Set up a temp working directory for this task
	workDir, err := os.MkdirTemp("", "podcaster-mcp-*")
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "create work dir failed")
		tm.store.FailJob(ctx, id, fmt.Sprintf("create work dir: %v", err))
		return
	}
	defer os.RemoveAll(workDir)

	// Determine input
	input := req.InputURL
	if input == "" && req.InputText != "" {
		// Write input text to a temp file
		inputPath := workDir + "/input.txt"
		if err := os.WriteFile(inputPath, []byte(req.InputText), 0644); err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, "write input failed")
			tm.store.FailJob(ctx, id, fmt.Sprintf("write input text: %v", err))
			return
		}
		input = inputPath
	}
	if input == "" {
		span.SetStatus(codes.Error, "no input")
		tm.store.FailJob(ctx, id, "no input provided")
		return
	}

	outputPath := workDir + "/" + id + ".mp3"
	scriptPath := workDir + "/" + id + ".json"

	model := req.Model
	if model == "" {
		model = "haiku"
	}
	ttsProvider := req.TTS
	if ttsProvider == "" {
		ttsProvider = "gemini"
	}
	duration := req.Duration
	if duration == "" {
		duration = "standard"
	}
	format := req.Format
	if format == "" {
		format = "conversation"
	}
	voices := req.Voices
	if voices == 0 {
		voices = 2
	}

	// Parse voice specs (provider:voiceID or plain voiceID)
	v1Provider, v1ID := tts.ParseVoiceSpec(req.Voice1)
	v2Provider, v2ID := tts.ParseVoiceSpec(req.Voice2)
	v3Provider, v3ID := tts.ParseVoiceSpec(req.Voice3)
	if v1Provider == "" {
		v1Provider = ttsProvider
	}
	if v2Provider == "" {
		v2Provider = ttsProvider
	}
	if v3Provider == "" {
		v3Provider = ttsProvider
	}

	// Parse comma-separated styles
	var styles []string
	if req.Style != "" {
		for _, s := range strings.Split(req.Style, ",") {
			s = strings.TrimSpace(s)
			if s != "" {
				styles = append(styles, s)
			}
		}
	}

	opts := pipeline.Options{
		Input:            input,
		Output:           outputPath,
		Topic:            req.Topic,
		Tone:             req.Tone,
		Duration:         duration,
		Format:           format,
		Styles:           styles,
		Voice1:           v1ID,
		Voice1Provider:   v1Provider,
		Voice2:           v2ID,
		Voice2Provider:   v2Provider,
		Voice3:           v3ID,
		Voice3Provider:   v3Provider,
		Voices:           voices,
		DefaultTTS:       ttsProvider,
		Model:            model,
		TTSModel:         req.TTSModel,
		TTSSpeed:         req.TTSSpeed,
		TTSStability:     req.TTSStability,
		TTSPitch:         req.TTSPitch,
		OnProgress:       progressCb,
		DisableBatch:     true, // Per-segment with rate limiting for AI Studio Gemini TTS 10 RPM limit
		AnthropicAPIKey:  req.AnthropicAPIKey,
		GeminiAPIKey:     req.GeminiAPIKey,
		ElevenLabsAPIKey: req.ElevenLabsAPIKey,
	}

	// Run the pipeline
	pipelineStart := time.Now()
	fmt.Fprintf(os.Stderr, "[%s] Pipeline starting: model=%s tts=%s duration=%s batch=%v voices=%d\n",
		id, model, ttsProvider, duration, !opts.DisableBatch, voices)
	log.InfoContext(ctx, "Pipeline starting",
		"model", model, "tts", ttsProvider, "duration", duration,
		"batch", !opts.DisableBatch, "voices", voices, "input_url", opts.Input)
	if err := pipeline.Run(ctx, opts); err != nil {
		elapsed := time.Since(pipelineStart).Round(time.Second)
		fmt.Fprintf(os.Stderr, "[%s] Pipeline FAILED after %s: %v\n", id, elapsed, err)
		span.RecordError(err)
		span.SetStatus(codes.Error, "pipeline failed")
		log.ErrorContext(ctx, "Pipeline failed", "error", err, "elapsed", elapsed.String())
		tm.store.FailJob(ctx, id, err.Error())
		return
	}

	// Read script metadata
	var title, summary, scriptJSON string
	if data, err := os.ReadFile(pipeline.ScriptPath(outputPath)); err == nil {
		scriptJSON = string(data)
		var s script.Script
		if json.Unmarshal(data, &s) == nil {
			title = s.Title
			summary = s.Summary
		}
	}
	// Fallback: try the workdir script path
	if title == "" {
		if data, err := os.ReadFile(scriptPath); err == nil {
			scriptJSON = string(data)
			var s script.Script
			if json.Unmarshal(data, &s) == nil {
				title = s.Title
				summary = s.Summary
			}
		}
	}

	// Get file size and duration
	var fileSizeMB float64
	if info, err := os.Stat(outputPath); err == nil {
		fileSizeMB = float64(info.Size()) / (1024 * 1024)
	}
	audioDuration := pipeline.ProbeDuration(outputPath)

	// Upload to S3
	tm.store.UpdateProgress(ctx, id, JobStatusUploading, 0.95, "Uploading to S3...")
	audioKey, audioURL, err := tm.storage.Upload(ctx, id, outputPath)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "upload failed")
		log.ErrorContext(ctx, "S3 upload failed", "error", err)
		tm.store.FailJob(ctx, id, fmt.Sprintf("upload to S3: %v", err))
		return
	}

	// Mark complete
	if err := tm.store.CompleteJob(ctx, id, title, summary, audioKey, audioURL, audioDuration, scriptJSON, fileSizeMB); err != nil {
		log.ErrorContext(ctx, "Complete job failed", "error", err)
	}

	// Record usage metrics if authenticated
	if req.UserID != "" {
		inputChars := len(req.InputText)
		if inputChars == 0 && req.InputURL != "" {
			inputChars = 5000 // estimate for URL-sourced content
		}

		// Calculate TTS chars from script segments
		ttsChars := 0
		if scriptJSON != "" {
			var s script.Script
			if json.Unmarshal([]byte(scriptJSON), &s) == nil {
				for _, seg := range s.Segments {
					ttsChars += len(seg.Text)
				}
			}
		}

		// Parse duration to seconds
		durationSec := parseDurationSec(audioDuration)

		if err := tm.store.RecordUsage(ctx, id, req.UserID, req.Model, req.TTS, inputChars, ttsChars, durationSec); err != nil {
			log.WarnContext(ctx, "Record usage failed", "error", err)
		} else {
			cost := EstimateCost(req.Model, req.TTS, inputChars, ttsChars, durationSec)
			log.InfoContext(ctx, "Usage recorded", "user_id", req.UserID, "cost_usd", cost)
		}
	}

	elapsed := time.Since(pipelineStart).Round(time.Second)
	fmt.Fprintf(os.Stderr, "[%s] Pipeline COMPLETE in %s: title=%s url=%s size=%.1fMB\n", id, elapsed, title, audioURL, fileSizeMB)
	span.SetAttributes(
		attribute.String("title", title),
		attribute.String("audio_url", audioURL),
		attribute.Float64("file_size_mb", fileSizeMB),
	)
	span.SetStatus(codes.Ok, "complete")
	log.InfoContext(ctx, "Pipeline complete", "title", title, "audio_url", audioURL)
}

// parseDurationSec converts a duration string like "12m34s" or "12:34" to seconds.
func parseDurationSec(d string) int {
	if d == "" {
		return 0
	}
	// Try Go duration format first
	if parsed, err := time.ParseDuration(d); err == nil {
		return int(parsed.Seconds())
	}
	// Try MM:SS format
	parts := strings.SplitN(d, ":", 2)
	if len(parts) == 2 {
		var m, s int
		fmt.Sscanf(parts[0], "%d", &m)
		fmt.Sscanf(parts[1], "%d", &s)
		return m*60 + s
	}
	return 0
}

// mapStage maps a pipeline progress stage to a job status.
func mapStage(stage progress.Stage) JobStatus {
	switch stage {
	case progress.StageIngest:
		return JobStatusIngesting
	case progress.StageScript:
		return JobStatusScripting
	case progress.StageTTS:
		return JobStatusSynthesizing
	case progress.StageAssembly:
		return JobStatusAssembling
	case progress.StageComplete:
		return JobStatusComplete
	default:
		return JobStatusSubmitted
	}
}
