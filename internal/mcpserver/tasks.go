package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"time"

	"github.com/apresai/podcaster/internal/pipeline"
	"github.com/apresai/podcaster/internal/progress"
	"github.com/apresai/podcaster/internal/script"
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

	mu       sync.Mutex
	cancels  map[string]context.CancelFunc
	maxTasks int
	running  int
}

// NewTaskManager creates a task manager.
func NewTaskManager(store *Store, storage *Storage, maxTasks int, logger *slog.Logger) *TaskManager {
	if maxTasks <= 0 {
		maxTasks = 5
	}
	return &TaskManager{
		store:    store,
		storage:  storage,
		log:      logger,
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

	// Use a detached context so the goroutine isn't cancelled when the HTTP request ends.
	taskCtx, cancel := context.WithCancel(context.Background())
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
	defer func() {
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

		status := mapStage(evt.Stage)
		if err := tm.store.UpdateProgress(ctx, id, status, evt.Percent, evt.Message); err != nil {
			log.Warn("Update progress failed", "error", err)
		}
		lastWrite = now
		lastStage = evt.Stage
	}

	// Set up a temp working directory for this task
	workDir, err := os.MkdirTemp("", "podcaster-mcp-*")
	if err != nil {
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
			tm.store.FailJob(ctx, id, fmt.Sprintf("write input text: %v", err))
			return
		}
		input = inputPath
	}
	if input == "" {
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

	opts := pipeline.Options{
		Input:            input,
		Output:           outputPath,
		Topic:            req.Topic,
		Tone:             req.Tone,
		Duration:         duration,
		Format:           format,
		Voices:           voices,
		DefaultTTS:       ttsProvider,
		Voice1Provider:   ttsProvider,
		Voice2Provider:   ttsProvider,
		Voice3Provider:   ttsProvider,
		Model:            model,
		OnProgress:       progressCb,
		AnthropicAPIKey:  req.AnthropicAPIKey,
		GeminiAPIKey:     req.GeminiAPIKey,
		ElevenLabsAPIKey: req.ElevenLabsAPIKey,
	}

	// Run the pipeline
	log.Info("Pipeline starting", "model", model, "tts", ttsProvider, "duration", duration)
	if err := pipeline.Run(ctx, opts); err != nil {
		log.Error("Pipeline failed", "error", err)
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
		log.Error("S3 upload failed", "error", err)
		tm.store.FailJob(ctx, id, fmt.Sprintf("upload to S3: %v", err))
		return
	}

	// Mark complete
	if err := tm.store.CompleteJob(ctx, id, title, summary, audioKey, audioURL, audioDuration, scriptJSON, fileSizeMB); err != nil {
		log.Error("Complete job failed", "error", err)
	}

	log.Info("Pipeline complete", "title", title, "audio_url", audioURL)
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
