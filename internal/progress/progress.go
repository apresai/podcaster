package progress

import "time"

// Stage identifies which pipeline stage is active.
type Stage string

const (
	StageIngest   Stage = "ingest"
	StageScript   Stage = "script"
	StageTTS      Stage = "tts"
	StageAssembly Stage = "assembly"
	StageComplete Stage = "complete"
)

// Event carries progress information from the pipeline to the renderer.
type Event struct {
	Stage        Stage
	Message      string
	Percent      float64 // 0.0–1.0
	SegmentNum   int
	SegmentTotal int
	Elapsed      time.Duration
	Error        error
	// OutputFile is set on StageComplete with the final file path.
	OutputFile string
	// Duration is the episode duration string (e.g. "12:34"), set on StageComplete.
	Duration string
	// SizeMB is the output file size in MB, set on StageComplete.
	SizeMB float64
	// LogFile is the log file path, set on StageComplete.
	LogFile string
}

// Callback is the function signature for progress event handlers.
type Callback func(Event)

// NopCallback is a no-op progress callback for tests and silent mode.
func NopCallback(Event) {}

// NewEvent creates an Event with common fields populated.
func NewEvent(stage Stage, msg string, pct float64, start time.Time) Event {
	return Event{
		Stage:   stage,
		Message: msg,
		Percent: pct,
		Elapsed: time.Since(start),
	}
}
