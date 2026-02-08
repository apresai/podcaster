package pipeline

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"time"

	"github.com/apresai/podcaster/internal/assembly"
	"github.com/apresai/podcaster/internal/ingest"
	"github.com/apresai/podcaster/internal/progress"
	"github.com/apresai/podcaster/internal/script"
	"github.com/apresai/podcaster/internal/tts"
)

// OutputBaseDir is the root directory for all podcaster output.
const OutputBaseDir = "podcaster-output"

type Options struct {
	Input          string
	Output         string
	Topic          string
	Tone           string
	Duration       string
	Format         string // show format: conversation, interview, debate, etc.
	Styles         []string
	Voice1         string // voice ID (without provider prefix)
	Voice1Provider string // "elevenlabs", "gemini", "google"
	Voice2         string
	Voice2Provider string
	Voice3         string
	Voice3Provider string
	Voices         int // 1-3, default 2
	ScriptOnly     bool
	FromScript     string
	Verbose        bool
	DefaultTTS     string // --tts value, for logging/defaults
	Model          string
	LogFile        string
	TTSModel       string  // --tts-model
	TTSSpeed       float64 // --tts-speed
	TTSStability   float64 // --tts-stability (ElevenLabs)
	TTSPitch       float64 // --tts-pitch (Google)
	OnProgress     progress.Callback
}

// CLICommand returns a reproducible CLI command for the current options.
func (o Options) CLICommand() string {
	var parts []string
	parts = append(parts, "podcaster generate")
	if o.Input != "" {
		parts = append(parts, fmt.Sprintf("-i %q", o.Input))
	}
	if o.FromScript != "" {
		parts = append(parts, fmt.Sprintf("--from-script %q", o.FromScript))
	}
	if o.Output != "" {
		parts = append(parts, fmt.Sprintf("-o %q", o.Output))
	}
	if o.Model != "" && o.Model != "haiku" {
		parts = append(parts, "--model", o.Model)
	}
	if o.DefaultTTS != "" && o.DefaultTTS != "gemini" {
		parts = append(parts, "--tts", o.DefaultTTS)
	}
	if o.TTSModel != "" {
		parts = append(parts, "--tts-model", o.TTSModel)
	}
	if o.Format != "" && o.Format != "conversation" {
		parts = append(parts, "--format", o.Format)
	}
	if o.Tone != "" && o.Tone != "casual" {
		parts = append(parts, "--tone", o.Tone)
	}
	if o.Duration != "" && o.Duration != "standard" {
		parts = append(parts, "--duration", o.Duration)
	}
	if len(o.Styles) > 0 {
		parts = append(parts, "--style", strings.Join(o.Styles, ","))
	}
	if o.Topic != "" {
		parts = append(parts, fmt.Sprintf("--topic %q", o.Topic))
	}
	if o.Voices != 0 && o.Voices != 2 {
		parts = append(parts, fmt.Sprintf("--voices %d", o.Voices))
	}
	if o.Voice1 != "" {
		v := o.Voice1
		if o.Voice1Provider != "" {
			v = o.Voice1Provider + ":" + v
		}
		parts = append(parts, "--voice1", v)
	}
	if o.Voice2 != "" {
		v := o.Voice2
		if o.Voice2Provider != "" {
			v = o.Voice2Provider + ":" + v
		}
		parts = append(parts, "--voice2", v)
	}
	if o.Voice3 != "" {
		v := o.Voice3
		if o.Voice3Provider != "" {
			v = o.Voice3Provider + ":" + v
		}
		parts = append(parts, "--voice3", v)
	}
	if o.TTSSpeed != 0 {
		parts = append(parts, fmt.Sprintf("--tts-speed %.2f", o.TTSSpeed))
	}
	if o.TTSStability != 0 {
		parts = append(parts, fmt.Sprintf("--tts-stability %.2f", o.TTSStability))
	}
	if o.TTSPitch != 0 {
		parts = append(parts, fmt.Sprintf("--tts-pitch %.2f", o.TTSPitch))
	}
	if o.ScriptOnly {
		parts = append(parts, "--script-only")
	}
	return strings.Join(parts, " ")
}

type PipelineError struct {
	Stage   string
	Message string
	Err     error
}

func (e *PipelineError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("[%s] %s: %v", e.Stage, e.Message, e.Err)
	}
	return fmt.Sprintf("[%s] %s", e.Stage, e.Message)
}

func (e *PipelineError) Unwrap() error {
	return e.Err
}

// EnsureOutputDirs creates the podcaster-output directory structure.
func EnsureOutputDirs() error {
	dirs := []string{
		filepath.Join(OutputBaseDir, "episodes"),
		filepath.Join(OutputBaseDir, "scripts"),
		filepath.Join(OutputBaseDir, "logs"),
		filepath.Join(OutputBaseDir, "tempfiles"),
	}
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0755); err != nil {
			return fmt.Errorf("create output directory %s: %w", d, err)
		}
	}
	return nil
}

// ScriptPath returns the script JSON path for a given output filename.
func ScriptPath(output string) string {
	base := filepath.Base(output)
	ext := filepath.Ext(base)
	name := strings.TrimSuffix(base, ext)
	return filepath.Join(OutputBaseDir, "scripts", name+".json")
}

// LogFilePath returns the log file path for a given output filename.
func LogFilePath(output string) string {
	base := filepath.Base(output)
	ext := filepath.Ext(base)
	name := strings.TrimSuffix(base, ext)
	return filepath.Join(OutputBaseDir, "logs", name+".log")
}

func Run(ctx context.Context, opts Options) error {
	pipelineStart := time.Now()

	// Ensure output directories exist
	if err := EnsureOutputDirs(); err != nil {
		return fmt.Errorf("setup output directories: %w", err)
	}

	// Default voices to 2 if not set
	if opts.Voices == 0 {
		opts.Voices = 2
	}

	// Set up logging — when not verbose, write to log file only (progress bar handles stdout)
	var logWriter io.Writer = os.Stdout
	if opts.LogFile != "" {
		lf, err := os.Create(opts.LogFile)
		if err != nil {
			return fmt.Errorf("create log file: %w", err)
		}
		defer lf.Close()
		if opts.Verbose {
			logWriter = io.MultiWriter(os.Stdout, lf)
		} else {
			logWriter = lf
		}
	}
	logger := log.New(logWriter, "", log.LstdFlags)
	logf := func(format string, args ...interface{}) {
		logger.Printf(format, args...)
	}

	// Progress emit helper
	emit := func(stage progress.Stage, msg string, pct float64) {
		if opts.OnProgress != nil {
			opts.OnProgress(progress.NewEvent(stage, msg, pct, pipelineStart))
		}
	}

	ctx, cancel := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer cancel()

	if opts.Output != "" {
		logf("Pipeline started — output: %s", opts.Output)
	} else {
		logf("Pipeline started — output: (auto-naming from script title)")
	}
	format := opts.Format
	if format == "" {
		format = "conversation"
	}
	logf("Config: model=%s tts=%s tone=%s duration=%s voices=%d format=%s", opts.Model, opts.DefaultTTS, opts.Tone, opts.Duration, opts.Voices, format)
	if len(opts.Styles) > 0 {
		logf("Config: styles=%s", strings.Join(opts.Styles, ","))
	}
	logf("Equivalent CLI: %s", opts.CLICommand())

	// Resolve voice map early so we can use voice names as speaker labels in scripts
	ps := tts.NewProviderSet()
	defer ps.Close()

	if opts.TTSModel != "" || opts.TTSSpeed != 0 || opts.TTSStability != 0 || opts.TTSPitch != 0 {
		ps.SetConfig(opts.DefaultTTS, tts.ProviderConfig{
			Model:     opts.TTSModel,
			Speed:     opts.TTSSpeed,
			Stability: opts.TTSStability,
			Pitch:     opts.TTSPitch,
		})
	}

	voices := tts.VoiceMap{}
	if opts.Voice1 != "" {
		voices.Host1 = tts.Voice{ID: opts.Voice1, Name: opts.Voice1, Provider: opts.Voice1Provider}
	} else {
		p, err := ps.Get(opts.Voice1Provider)
		if err != nil {
			return &PipelineError{Stage: "tts", Message: "failed to create TTS provider", Err: err}
		}
		dv := p.DefaultVoices()
		voices.Host1 = tts.Voice{ID: dv.Host1.ID, Name: dv.Host1.Name, Provider: opts.Voice1Provider}
	}
	if opts.Voice2 != "" {
		voices.Host2 = tts.Voice{ID: opts.Voice2, Name: opts.Voice2, Provider: opts.Voice2Provider}
	} else {
		p, err := ps.Get(opts.Voice2Provider)
		if err != nil {
			return &PipelineError{Stage: "tts", Message: "failed to create TTS provider", Err: err}
		}
		dv := p.DefaultVoices()
		voices.Host2 = tts.Voice{ID: dv.Host2.ID, Name: dv.Host2.Name, Provider: opts.Voice2Provider}
	}
	if opts.Voice3 != "" {
		voices.Host3 = tts.Voice{ID: opts.Voice3, Name: opts.Voice3, Provider: opts.Voice3Provider}
	} else {
		p, err := ps.Get(opts.Voice3Provider)
		if err != nil {
			return &PipelineError{Stage: "tts", Message: "failed to create TTS provider", Err: err}
		}
		dv := p.DefaultVoices()
		voices.Host3 = tts.Voice{ID: dv.Host3.ID, Name: dv.Host3.Name, Provider: opts.Voice3Provider}
	}

	// Set dynamic speaker names from voice names
	voices.SpeakerNames = [3]string{voices.Host1.Name, voices.Host2.Name, voices.Host3.Name}

	// Build speaker names list for script generation
	var speakerNames []string
	switch opts.Voices {
	case 1:
		speakerNames = []string{voices.Host1.Name}
	case 3:
		speakerNames = []string{voices.Host1.Name, voices.Host2.Name, voices.Host3.Name}
	default:
		speakerNames = []string{voices.Host1.Name, voices.Host2.Name}
	}

	var s *script.Script

	if opts.FromScript != "" {
		logf("Loading script from %s...", opts.FromScript)
		loaded, err := script.LoadScript(opts.FromScript)
		if err != nil {
			logf("ERROR: failed to load script: %v", err)
			return &PipelineError{Stage: "script", Message: "failed to load script", Err: err}
		}
		s = loaded
		logf("Script loaded: %d segments", len(s.Segments))
	} else {
		// Stage 1: Ingest
		stageStart := time.Now()
		emit(progress.StageIngest, "Ingesting content...", 0.0)
		logf("Stage 1/4: Ingesting content from %s", opts.Input)
		ingester := ingest.NewIngester(opts.Input)
		content, err := ingester.Ingest(ctx, opts.Input)
		if err != nil {
			logf("ERROR: ingest failed: %v", err)
			return &PipelineError{Stage: "ingest", Message: "failed to extract content", Err: err}
		}
		logf("Ingest complete: %d words from %s (%s)", content.WordCount, content.Source, time.Since(stageStart).Round(time.Millisecond))
		emit(progress.StageIngest, "Ingest complete", 0.05)

		if opts.Verbose {
			logf("  Title: %s", content.Title)
			logf("  Source type: %s", ingest.DetectSource(opts.Input))
			logf("  Content size: %d bytes", len(content.Text))
		}

		if content.WordCount < 100 {
			logf("ERROR: input too short (%d words)", content.WordCount)
			return &PipelineError{
				Stage:   "ingest",
				Message: fmt.Sprintf("input too short (%d words) — need at least 100 words to generate a meaningful conversation", content.WordCount),
			}
		}

		// Stage 2: Script Generation
		stageStart = time.Now()
		modelName := script.ModelDisplayName(opts.Model)
		emit(progress.StageScript, fmt.Sprintf("Generating script (%s)...", modelName), 0.05)
		logf("Stage 2/4: Generating script with %s...", modelName)
		gen, err := script.NewGenerator(opts.Model)
		if err != nil {
			logf("ERROR: failed to create script generator: %v", err)
			return &PipelineError{Stage: "script", Message: "failed to create script generator", Err: err}
		}
		genOpts := script.GenerateOptions{
			Topic:        opts.Topic,
			Tone:         opts.Tone,
			Duration:     opts.Duration,
			Styles:       opts.Styles,
			Model:        opts.Model,
			Voices:       opts.Voices,
			Format:       opts.Format,
			SpeakerNames: speakerNames,
		}
		s, err = gen.Generate(ctx, content.Text, genOpts)
		if err != nil {
			logf("ERROR: script generation failed: %v", err)
			return &PipelineError{Stage: "script", Message: "failed to generate script", Err: err}
		}
		logf("Script complete: %d segments, ~%d min (%s)", len(s.Segments), estimateMinutes(s), time.Since(stageStart).Round(time.Millisecond))
		emit(progress.StageScript, "Script complete", 0.18)

		// Stage 2b: Script review (always-on)
		logf("Stage 2b: Reviewing script quality...")
		reviewer, revErr := script.NewReviewer(opts.Model)
		if revErr != nil {
			logf("WARNING: could not create reviewer: %v", revErr)
		} else {
			result, revErr := reviewer.Review(ctx, s, content.Text, genOpts)
			if revErr != nil {
				logf("WARNING: script review failed: %v", revErr)
			} else {
				for _, issue := range result.Issues {
					logf("  Review [%s] %s: %s", issue.Severity, issue.Category, issue.Message)
				}
				if result.Approved {
					logf("Script review passed")
				} else if result.Revised != nil {
					logf("Script revised: %d → %d segments", len(s.Segments), len(result.Revised.Segments))
					s = result.Revised
				} else {
					logf("Script review found issues but revision was not possible")
				}
			}
		}
		emit(progress.StageScript, "Review complete", 0.20)
	}

	// Auto-name output from script title if output was not specified
	if opts.Output == "" {
		autoName := AutoOutputName(s.Title)
		opts.Output = filepath.Join(OutputBaseDir, "episodes", autoName)
		opts.LogFile = LogFilePath(autoName)

		// Re-open log file with new name
		if opts.LogFile != "" {
			lf2, err := os.Create(opts.LogFile)
			if err == nil {
				defer lf2.Close()
				if opts.Verbose {
					logger.SetOutput(io.MultiWriter(os.Stdout, lf2))
				} else {
					logger.SetOutput(lf2)
				}
			}
		}
		logf("Auto-named output: %s", opts.Output)
	}

	// Save the script to the scripts subdirectory
	scriptPath := ScriptPath(opts.Output)
	if opts.ScriptOnly {
		// For script-only mode, also save to the scripts dir
		scriptPath = ScriptPath(opts.Output)
	}
	if err := script.SaveScript(s, scriptPath); err != nil {
		logf("WARNING: failed to save intermediate script: %v", err)
	} else {
		logf("Script saved to %s (use --from-script to resume)", scriptPath)
	}

	if opts.ScriptOnly {
		emit(progress.StageComplete, fmt.Sprintf("Script saved to %s", scriptPath), 1.0)
		return nil
	}

	// Stage 3: TTS
	stageStart := time.Now()
	emit(progress.StageTTS, fmt.Sprintf("Synthesizing audio (%d segments)...", len(s.Segments)), 0.20)

	// Log voice routing
	logf("Voice routing: %s→%s, %s→%s", voices.Host1.Name, voices.Host1.Provider, voices.Host2.Name, voices.Host2.Provider)
	if opts.Voices >= 3 {
		logf("Voice routing: %s→%s", voices.Host3.Name, voices.Host3.Provider)
	}

	// Determine if all voices use the same provider
	singleProvider := voices.Host1.Provider == voices.Host2.Provider
	if opts.Voices >= 3 {
		singleProvider = singleProvider && voices.Host1.Provider == voices.Host3.Provider
	}

	if singleProvider {
		logf("Mode: single-provider (%s)", voices.Host1.Provider)
	} else {
		logf("Mode: mixed-provider")
	}

	if singleProvider {
		logf("Stage 3/4: Synthesizing audio via %s", voices.Host1.Provider)
	} else {
		logf("Stage 3/4: Synthesizing audio via mixed providers (per-segment)")
	}
	logf("  Voice 1 (%s): %s [%s]", voices.Host1.Name, voices.Host1.ID, voices.Host1.Provider)
	logf("  Voice 2 (%s): %s [%s]", voices.Host2.Name, voices.Host2.ID, voices.Host2.Provider)
	if opts.Voices >= 3 {
		logf("  Voice 3 (%s): %s [%s]", voices.Host3.Name, voices.Host3.ID, voices.Host3.Provider)
	}

	if singleProvider {
		provider, err := ps.Get(voices.Host1.Provider)
		if err != nil {
			return &PipelineError{Stage: "tts", Message: "failed to create TTS provider", Err: err}
		}

		// Check if provider supports batch synthesis (e.g., Gemini multi-speaker)
		if bp, ok := provider.(tts.BatchProvider); ok {
			result, err := bp.SynthesizeBatch(ctx, s.Segments, voices)
			if err != nil {
				logf("ERROR: batch synthesis failed: %v", err)
				return &PipelineError{Stage: "tts", Message: "batch synthesis failed", Err: err}
			}

			logf("TTS complete: format=%s (%s)", result.Format, time.Since(stageStart).Round(time.Millisecond))
			emit(progress.StageTTS, "TTS complete", 0.90)

			// Convert to MP3 if needed, or write directly
			if result.Format != tts.FormatMP3 {
				tmpDir, err := os.MkdirTemp(filepath.Join(OutputBaseDir, "tempfiles"), "run-*")
				if err != nil {
					return &PipelineError{Stage: "tts", Message: "failed to create temp directory", Err: err}
				}

				rawPath := filepath.Join(tmpDir, "batch_output.raw")
				if err := os.WriteFile(rawPath, result.Data, 0644); err != nil {
					return &PipelineError{Stage: "tts", Message: "failed to write raw audio", Err: err}
				}
				emit(progress.StageAssembly, "Assembling episode...", 0.90)
				logf("Stage 4/4: Converting to MP3...")
				if err := assembly.ConvertToMP3(ctx, rawPath, string(result.Format), opts.Output); err != nil {
					logf("ERROR: MP3 conversion failed: %v", err)
					logf("  Raw audio preserved in: %s", tmpDir)
					return &PipelineError{Stage: "assembly", Message: "failed to convert audio to MP3", Err: err}
				}
				os.RemoveAll(tmpDir)
			} else {
				if err := os.WriteFile(opts.Output, result.Data, 0644); err != nil {
					return &PipelineError{Stage: "tts", Message: "failed to write output", Err: err}
				}
			}

			logf("Assembly skipped (batch provider)")
		} else {
			// Single provider, per-segment synthesis
			tmpDir, err := os.MkdirTemp(filepath.Join(OutputBaseDir, "tempfiles"), "run-*")
			if err != nil {
				return &PipelineError{Stage: "tts", Message: "failed to create temp directory", Err: err}
			}
			logf("  Temp directory: %s", tmpDir)

			audioFiles, err := synthesizeSegments(ctx, provider, s.Segments, voices, tmpDir, logf, opts.OnProgress, pipelineStart)
			if err != nil {
				logf("ERROR: TTS synthesis failed: %v", err)
				logf("  Segments preserved in: %s", tmpDir)
				return &PipelineError{Stage: "tts", Message: "failed to synthesize audio", Err: err}
			}

			logf("TTS complete: %d segments (%s)", len(audioFiles), time.Since(stageStart).Round(time.Millisecond))

			if opts.Verbose {
				var totalBytes int64
				for _, f := range audioFiles {
					if info, err := os.Stat(f); err == nil {
						totalBytes += info.Size()
					}
				}
				logf("  Total audio data: %d bytes (%d files)", totalBytes, len(audioFiles))
			}

			// Stage 4: Assembly
			stageStart = time.Now()
			emit(progress.StageAssembly, "Assembling episode...", 0.90)
			logf("Stage 4/4: Assembling episode...")
			assembler := assembly.NewFFmpegAssembler()
			if err := assembler.Assemble(ctx, audioFiles, tmpDir, opts.Output); err != nil {
				logf("ERROR: assembly failed: %v", err)
				logf("  Segments preserved in: %s", tmpDir)
				logf("  Script preserved in: %s", scriptPath)
				return &PipelineError{Stage: "assembly", Message: "failed to assemble episode", Err: err}
			}
			logf("Assembly complete (%s)", time.Since(stageStart).Round(time.Millisecond))

			os.RemoveAll(tmpDir)
		}
	} else {
		// Mixed providers — per-segment with routing
		tmpDir, err := os.MkdirTemp(filepath.Join(OutputBaseDir, "tempfiles"), "run-*")
		if err != nil {
			return &PipelineError{Stage: "tts", Message: "failed to create temp directory", Err: err}
		}
		logf("  Temp directory: %s", tmpDir)

		audioFiles, err := synthesizeSegmentsMixed(ctx, ps, s.Segments, voices, tmpDir, logf, opts.OnProgress, pipelineStart)
		if err != nil {
			logf("ERROR: TTS synthesis failed: %v", err)
			logf("  Segments preserved in: %s", tmpDir)
			return &PipelineError{Stage: "tts", Message: "failed to synthesize audio", Err: err}
		}

		logf("TTS complete: %d segments (%s)", len(audioFiles), time.Since(stageStart).Round(time.Millisecond))

		if opts.Verbose {
			var totalBytes int64
			for _, f := range audioFiles {
				if info, err := os.Stat(f); err == nil {
					totalBytes += info.Size()
				}
			}
			logf("  Total audio data: %d bytes (%d files)", totalBytes, len(audioFiles))
		}

		// Stage 4: Assembly
		stageStart = time.Now()
		emit(progress.StageAssembly, "Assembling episode...", 0.90)
		logf("Stage 4/4: Assembling episode...")
		assembler := assembly.NewFFmpegAssembler()
		if err := assembler.Assemble(ctx, audioFiles, tmpDir, opts.Output); err != nil {
			logf("ERROR: assembly failed: %v", err)
			logf("  Segments preserved in: %s", tmpDir)
			logf("  Script preserved in: %s", scriptPath)
			return &PipelineError{Stage: "assembly", Message: "failed to assemble episode", Err: err}
		}
		logf("Assembly complete (%s)", time.Since(stageStart).Round(time.Millisecond))

		os.RemoveAll(tmpDir)
	}

	// Report final output
	var completionEvent progress.Event
	completionEvent.Stage = progress.StageComplete
	completionEvent.LogFile = opts.LogFile
	completionEvent.Elapsed = time.Since(pipelineStart)

	info, err := os.Stat(opts.Output)
	if err == nil {
		sizeMB := float64(info.Size()) / (1024 * 1024)
		duration := ProbeDuration(opts.Output)
		completionEvent.OutputFile = opts.Output
		completionEvent.SizeMB = sizeMB
		completionEvent.Duration = duration
		completionEvent.Percent = 1.0
		if duration != "" {
			logf("Episode saved to %s (%s, %.1f MB)", opts.Output, duration, sizeMB)
			completionEvent.Message = fmt.Sprintf("Episode saved to %s (%s, %.1f MB)", opts.Output, duration, sizeMB)
		} else {
			logf("Episode saved to %s (%.1f MB)", opts.Output, sizeMB)
			completionEvent.Message = fmt.Sprintf("Episode saved to %s (%.1f MB)", opts.Output, sizeMB)
		}
	}

	logf("Total pipeline time: %s", time.Since(pipelineStart).Round(time.Millisecond))

	if opts.OnProgress != nil {
		opts.OnProgress(completionEvent)
	}

	return nil
}

// synthesizeSegments runs per-segment TTS with progress output, converting
// non-MP3 formats to MP3 as needed.
func synthesizeSegments(ctx context.Context, provider tts.Provider, segments []script.Segment, voices tts.VoiceMap, tmpDir string, logf func(string, ...interface{}), onProgress progress.Callback, pipelineStart time.Time) ([]string, error) {
	total := len(segments)
	files := make([]string, 0, total)

	for i, seg := range segments {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}

		voice := tts.VoiceForSpeaker(seg.Speaker, voices)

		logf("  Synthesizing segment %d/%d (%s, %d chars)", i+1, total, seg.Speaker, len(seg.Text))

		if onProgress != nil {
			pct := 0.20 + 0.70*float64(i)/float64(total)
			onProgress(progress.Event{
				Stage:        progress.StageTTS,
				Message:      fmt.Sprintf("Synthesizing segment %d/%d (%s, %s)", i+1, total, seg.Speaker, voice.Provider),
				Percent:      pct,
				SegmentNum:   i + 1,
				SegmentTotal: total,
				Elapsed:      time.Since(pipelineStart),
			})
		}

		var result tts.AudioResult
		err := tts.WithRetry(ctx, func() error {
			var synthErr error
			result, synthErr = provider.Synthesize(ctx, seg.Text, voice)
			return synthErr
		})
		if err != nil {
			return nil, fmt.Errorf("segment %d (%s): %w", i+1, seg.Speaker, err)
		}

		// If provider returns non-MP3, convert via FFmpeg
		filename := filepath.Join(tmpDir, fmt.Sprintf("segment_%03d.mp3", i))
		if result.Format != tts.FormatMP3 {
			rawPath := filepath.Join(tmpDir, fmt.Sprintf("segment_%03d.raw", i))
			if err := os.WriteFile(rawPath, result.Data, 0644); err != nil {
				return nil, fmt.Errorf("write raw segment %d: %w", i+1, err)
			}
			if err := assembly.ConvertToMP3(ctx, rawPath, string(result.Format), filename); err != nil {
				return nil, fmt.Errorf("convert segment %d: %w", i+1, err)
			}
		} else {
			if err := os.WriteFile(filename, result.Data, 0644); err != nil {
				return nil, fmt.Errorf("write segment %d: %w", i+1, err)
			}
		}

		files = append(files, filename)
	}

	// Emit TTS complete
	if onProgress != nil {
		onProgress(progress.Event{
			Stage:   progress.StageTTS,
			Message: "TTS complete",
			Percent: 0.90,
			Elapsed: time.Since(pipelineStart),
		})
	}

	return files, nil
}

// synthesizeSegmentsMixed runs per-segment TTS with provider routing for
// mixed-provider episodes. Each segment is routed to the provider specified
// in the voice's Provider field via ProviderSet.
func synthesizeSegmentsMixed(ctx context.Context, ps *tts.ProviderSet, segments []script.Segment, voices tts.VoiceMap, tmpDir string, logf func(string, ...interface{}), onProgress progress.Callback, pipelineStart time.Time) ([]string, error) {
	total := len(segments)
	files := make([]string, 0, total)

	for i, seg := range segments {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}

		voice := tts.VoiceForSpeaker(seg.Speaker, voices)
		provider, err := ps.Get(voice.Provider)
		if err != nil {
			return nil, fmt.Errorf("segment %d (%s): get provider %s: %w", i+1, seg.Speaker, voice.Provider, err)
		}

		logf("  Synthesizing segment %d/%d (%s, %d chars, %s)", i+1, total, seg.Speaker, len(seg.Text), voice.Provider)

		if onProgress != nil {
			pct := 0.20 + 0.70*float64(i)/float64(total)
			onProgress(progress.Event{
				Stage:        progress.StageTTS,
				Message:      fmt.Sprintf("Synthesizing segment %d/%d (%s, %s)", i+1, total, seg.Speaker, voice.Provider),
				Percent:      pct,
				SegmentNum:   i + 1,
				SegmentTotal: total,
				Elapsed:      time.Since(pipelineStart),
			})
		}

		var result tts.AudioResult
		err = tts.WithRetry(ctx, func() error {
			var synthErr error
			result, synthErr = provider.Synthesize(ctx, seg.Text, voice)
			return synthErr
		})
		if err != nil {
			return nil, fmt.Errorf("segment %d (%s): %w", i+1, seg.Speaker, err)
		}

		// If provider returns non-MP3, convert via FFmpeg
		filename := filepath.Join(tmpDir, fmt.Sprintf("segment_%03d.mp3", i))
		if result.Format != tts.FormatMP3 {
			rawPath := filepath.Join(tmpDir, fmt.Sprintf("segment_%03d.raw", i))
			if err := os.WriteFile(rawPath, result.Data, 0644); err != nil {
				return nil, fmt.Errorf("write raw segment %d: %w", i+1, err)
			}
			if err := assembly.ConvertToMP3(ctx, rawPath, string(result.Format), filename); err != nil {
				return nil, fmt.Errorf("convert segment %d: %w", i+1, err)
			}
		} else {
			if err := os.WriteFile(filename, result.Data, 0644); err != nil {
				return nil, fmt.Errorf("write segment %d: %w", i+1, err)
			}
		}

		files = append(files, filename)
	}

	// Emit TTS complete
	if onProgress != nil {
		onProgress(progress.Event{
			Stage:   progress.StageTTS,
			Message: "TTS complete",
			Percent: 0.90,
			Elapsed: time.Since(pipelineStart),
		})
	}

	return files, nil
}

func ProbeDuration(path string) string {
	out, err := exec.Command("ffprobe",
		"-v", "quiet",
		"-show_entries", "format=duration",
		"-of", "default=noprint_wrappers=1:nokey=1",
		path,
	).Output()
	if err != nil {
		return ""
	}
	s := strings.TrimSpace(string(out))
	var secs float64
	if _, err := fmt.Sscanf(s, "%f", &secs); err != nil {
		return ""
	}
	mins := int(secs) / 60
	remainSecs := int(secs) % 60
	return fmt.Sprintf("%d:%02d", mins, remainSecs)
}

func estimateMinutes(s *script.Script) int {
	totalWords := 0
	for _, seg := range s.Segments {
		totalWords += wordCount(seg.Text)
	}
	minutes := totalWords / 150
	if minutes < 1 {
		minutes = 1
	}
	return minutes
}

func wordCount(s string) int {
	count := 0
	inWord := false
	for _, r := range s {
		if r == ' ' || r == '\t' || r == '\n' || r == '\r' {
			inWord = false
		} else if !inWord {
			inWord = true
			count++
		}
	}
	return count
}

var slugNonAlnum = regexp.MustCompile(`[^a-z0-9]+`)

// slugify converts a title into a URL-safe filename slug.
// Lowercase, replace non-alphanumeric with hyphens, collapse consecutive hyphens, max 50 chars.
func slugify(title string) string {
	s := strings.ToLower(title)
	// Replace non-alphanumeric with hyphens
	s = slugNonAlnum.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	// Truncate to 50 chars, trimming trailing hyphens
	if len(s) > 50 {
		s = s[:50]
		s = strings.TrimRight(s, "-")
	}
	return s
}

// AutoOutputName generates a filename from the script title + timestamp.
func AutoOutputName(title string) string {
	slug := slugify(title)
	if slug == "" {
		slug = "podcast"
	}
	ts := time.Now().Format("20060102-1504")
	return slug + "-" + ts + ".mp3"
}

