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
	"strings"
	"syscall"
	"time"

	"github.com/apresai/podcaster/internal/assembly"
	"github.com/apresai/podcaster/internal/ingest"
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

	// Set up logging — write to both stdout and log file
	var logWriter io.Writer = os.Stdout
	if opts.LogFile != "" {
		lf, err := os.Create(opts.LogFile)
		if err != nil {
			return fmt.Errorf("create log file: %w", err)
		}
		defer lf.Close()
		logWriter = io.MultiWriter(os.Stdout, lf)
	}
	logger := log.New(logWriter, "", log.LstdFlags)
	logf := func(format string, args ...interface{}) {
		logger.Printf(format, args...)
	}

	ctx, cancel := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer cancel()

	logf("Pipeline started — output: %s", opts.Output)
	logf("Config: model=%s tts=%s tone=%s duration=%s voices=%d", opts.Model, opts.DefaultTTS, opts.Tone, opts.Duration, opts.Voices)
	if len(opts.Styles) > 0 {
		logf("Config: styles=%s", strings.Join(opts.Styles, ","))
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
		logf("Stage 1/4: Ingesting content from %s", opts.Input)
		ingester := ingest.NewIngester(opts.Input)
		content, err := ingester.Ingest(ctx, opts.Input)
		if err != nil {
			logf("ERROR: ingest failed: %v", err)
			return &PipelineError{Stage: "ingest", Message: "failed to extract content", Err: err}
		}
		logf("Ingest complete: %d words from %s (%s)", content.WordCount, content.Source, time.Since(stageStart).Round(time.Millisecond))

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
		logf("Stage 2/4: Generating script with %s...", script.ModelDisplayName(opts.Model))
		gen, err := script.NewGenerator(opts.Model)
		if err != nil {
			logf("ERROR: failed to create script generator: %v", err)
			return &PipelineError{Stage: "script", Message: "failed to create script generator", Err: err}
		}
		genOpts := script.GenerateOptions{
			Topic:    opts.Topic,
			Tone:     opts.Tone,
			Duration: opts.Duration,
			Styles:   opts.Styles,
			Model:    opts.Model,
			Voices:   opts.Voices,
		}
		s, err = gen.Generate(ctx, content.Text, genOpts)
		if err != nil {
			logf("ERROR: script generation failed: %v", err)
			return &PipelineError{Stage: "script", Message: "failed to generate script", Err: err}
		}
		logf("Script complete: %d segments, ~%d min (%s)", len(s.Segments), estimateMinutes(s), time.Since(stageStart).Round(time.Millisecond))
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
		return nil
	}

	// Stage 3: TTS
	stageStart := time.Now()

	// Build provider set for lazy provider creation
	ps := tts.NewProviderSet()
	defer ps.Close()

	// Build voice map with provider info, using defaults where not overridden
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

	// Determine if all voices use the same provider
	singleProvider := voices.Host1.Provider == voices.Host2.Provider
	if opts.Voices >= 3 {
		singleProvider = singleProvider && voices.Host1.Provider == voices.Host3.Provider
	}

	if singleProvider {
		logf("Stage 3/4: Synthesizing audio via %s", voices.Host1.Provider)
	} else {
		logf("Stage 3/4: Synthesizing audio via mixed providers (per-segment)")
	}
	logf("  Voice 1 (Alex): %s (%s) [%s]", voices.Host1.Name, voices.Host1.ID, voices.Host1.Provider)
	logf("  Voice 2 (Sam): %s (%s) [%s]", voices.Host2.Name, voices.Host2.ID, voices.Host2.Provider)
	if opts.Voices >= 3 {
		logf("  Voice 3 (Jordan): %s (%s) [%s]", voices.Host3.Name, voices.Host3.ID, voices.Host3.Provider)
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

			audioFiles, err := synthesizeSegments(ctx, provider, s.Segments, voices, tmpDir, logf)
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

		audioFiles, err := synthesizeSegmentsMixed(ctx, ps, s.Segments, voices, tmpDir, logf)
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
	info, err := os.Stat(opts.Output)
	if err == nil {
		sizeMB := float64(info.Size()) / (1024 * 1024)
		duration := probeDuration(opts.Output)
		if duration != "" {
			logf("Episode saved to %s (%s, %.1f MB)", opts.Output, duration, sizeMB)
		} else {
			logf("Episode saved to %s (%.1f MB)", opts.Output, sizeMB)
		}
	}

	logf("Total pipeline time: %s", time.Since(pipelineStart).Round(time.Millisecond))
	if opts.LogFile != "" {
		fmt.Printf("  Log written to %s\n", opts.LogFile)
	}

	return nil
}

// synthesizeSegments runs per-segment TTS with progress output, converting
// non-MP3 formats to MP3 as needed.
func synthesizeSegments(ctx context.Context, provider tts.Provider, segments []script.Segment, voices tts.VoiceMap, tmpDir string, logf func(string, ...interface{})) ([]string, error) {
	total := len(segments)
	files := make([]string, 0, total)

	for i, seg := range segments {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}

		voice := tts.VoiceForSpeaker(seg.Speaker, voices)

		logf("  Synthesizing segment %d/%d (%s, %d chars)", i+1, total, seg.Speaker, len(seg.Text))

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

	return files, nil
}

// synthesizeSegmentsMixed runs per-segment TTS with provider routing for
// mixed-provider episodes. Each segment is routed to the provider specified
// in the voice's Provider field via ProviderSet.
func synthesizeSegmentsMixed(ctx context.Context, ps *tts.ProviderSet, segments []script.Segment, voices tts.VoiceMap, tmpDir string, logf func(string, ...interface{})) ([]string, error) {
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

	return files, nil
}

func progressBar(pct, width int) string {
	filled := (pct * width) / 100
	if filled > width {
		filled = width
	}
	empty := width - filled
	return fmt.Sprintf("%s%s", repeatStr("█", filled), repeatStr("░", empty))
}

func repeatStr(s string, n int) string {
	result := ""
	for i := 0; i < n; i++ {
		result += s
	}
	return result
}

func probeDuration(path string) string {
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
