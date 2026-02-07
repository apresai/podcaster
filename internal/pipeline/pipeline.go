package pipeline

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/chad/podcaster/internal/assembly"
	"github.com/chad/podcaster/internal/ingest"
	"github.com/chad/podcaster/internal/script"
	"github.com/chad/podcaster/internal/tts"
)

type Options struct {
	Input       string
	Output      string
	Topic       string
	Tone        string
	Duration    string
	Styles      []string
	VoiceAlex   string
	VoiceSam    string
	ScriptOnly  bool
	FromScript  string
	Verbose     bool
	TTSProvider string
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

func Run(ctx context.Context, opts Options) error {
	pipelineStart := time.Now()

	ctx, cancel := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer cancel()

	var s *script.Script

	if opts.FromScript != "" {
		fmt.Printf("  Loading script from %s...", opts.FromScript)
		loaded, err := script.LoadScript(opts.FromScript)
		if err != nil {
			fmt.Println(" failed")
			return &PipelineError{Stage: "script", Message: "failed to load script", Err: err}
		}
		s = loaded
		fmt.Printf(" done (%d segments)\n", len(s.Segments))
	} else {
		// Stage 1: Ingest
		stageStart := time.Now()
		fmt.Printf("  Ingesting content...")
		ingester := ingest.NewIngester(opts.Input)
		content, err := ingester.Ingest(ctx, opts.Input)
		if err != nil {
			fmt.Println(" failed")
			return &PipelineError{Stage: "ingest", Message: "failed to extract content", Err: err}
		}
		fmt.Printf(" done (%d words from %s)\n", content.WordCount, content.Source)

		if opts.Verbose {
			fmt.Printf("    Title: %s\n", content.Title)
			fmt.Printf("    Source type: %s\n", ingest.DetectSource(opts.Input))
			fmt.Printf("    Content size: %d bytes\n", len(content.Text))
			fmt.Printf("    Ingest time: %s\n", time.Since(stageStart).Round(time.Millisecond))
		}

		if content.WordCount < 100 {
			return &PipelineError{
				Stage:   "ingest",
				Message: fmt.Sprintf("input too short (%d words) — need at least 100 words to generate a meaningful conversation", content.WordCount),
			}
		}

		// Stage 2: Script Generation
		stageStart = time.Now()
		fmt.Printf("  Generating script...")
		gen := script.NewClaudeGenerator()
		genOpts := script.GenerateOptions{
			Topic:    opts.Topic,
			Tone:     opts.Tone,
			Duration: opts.Duration,
			Styles:   opts.Styles,
		}
		s, err = gen.Generate(ctx, content.Text, genOpts)
		if err != nil {
			fmt.Println(" failed")
			return &PipelineError{Stage: "script", Message: "failed to generate script", Err: err}
		}
		fmt.Printf(" done (%d segments, ~%d min)\n", len(s.Segments), estimateMinutes(s))

		if opts.Verbose {
			fmt.Printf("    Model: %s\n", "claude-sonnet-4-5-20250929")
			fmt.Printf("    Script gen time: %s\n", time.Since(stageStart).Round(time.Millisecond))
		}
	}

	if opts.ScriptOnly {
		if err := script.SaveScript(s, opts.Output); err != nil {
			return &PipelineError{Stage: "script", Message: "failed to save script", Err: err}
		}
		fmt.Printf("\n  Script saved to %s\n", opts.Output)
		return nil
	}

	// Stage 3: TTS
	stageStart := time.Now()
	provider, err := tts.NewProvider(opts.TTSProvider, opts.VoiceAlex, opts.VoiceSam)
	if err != nil {
		return &PipelineError{Stage: "tts", Message: "failed to create TTS provider", Err: err}
	}
	defer provider.Close()
	voices := provider.DefaultVoices()
	if opts.VoiceAlex != "" {
		voices.Alex = tts.Voice{ID: opts.VoiceAlex, Name: opts.VoiceAlex}
	}
	if opts.VoiceSam != "" {
		voices.Sam = tts.Voice{ID: opts.VoiceSam, Name: opts.VoiceSam}
	}

	fmt.Printf("  Synthesizing audio via %s...\n", provider.Name())

	if opts.Verbose {
		fmt.Printf("    Voice Alex: %s\n", voices.Alex.ID)
		fmt.Printf("    Voice Sam: %s\n", voices.Sam.ID)
	}

	// Check if provider supports batch synthesis (e.g., Gemini multi-speaker)
	if bp, ok := provider.(tts.BatchProvider); ok {
		result, err := bp.SynthesizeBatch(ctx, s.Segments, voices)
		if err != nil {
			return &PipelineError{Stage: "tts", Message: "batch synthesis failed", Err: err}
		}

		if opts.Verbose {
			fmt.Printf("    TTS time: %s\n", time.Since(stageStart).Round(time.Millisecond))
			fmt.Printf("    Output format: %s\n", result.Format)
		}

		// Convert to MP3 if needed, or write directly
		if result.Format != tts.FormatMP3 {
			tmpDir, err := os.MkdirTemp("", "podcaster-*")
			if err != nil {
				return &PipelineError{Stage: "tts", Message: "failed to create temp directory", Err: err}
			}
			defer os.RemoveAll(tmpDir)

			rawPath := filepath.Join(tmpDir, "batch_output.raw")
			if err := os.WriteFile(rawPath, result.Data, 0644); err != nil {
				return &PipelineError{Stage: "tts", Message: "failed to write raw audio", Err: err}
			}
			if err := assembly.ConvertToMP3(ctx, rawPath, string(result.Format), opts.Output); err != nil {
				return &PipelineError{Stage: "assembly", Message: "failed to convert audio to MP3", Err: err}
			}
		} else {
			if err := os.WriteFile(opts.Output, result.Data, 0644); err != nil {
				return &PipelineError{Stage: "tts", Message: "failed to write output", Err: err}
			}
		}

		fmt.Println("  Assembly skipped (batch provider)")
	} else {
		// Per-segment synthesis path
		tmpDir, err := os.MkdirTemp("", "podcaster-*")
		if err != nil {
			return &PipelineError{Stage: "tts", Message: "failed to create temp directory", Err: err}
		}
		defer os.RemoveAll(tmpDir)

		if opts.Verbose {
			fmt.Printf("    Temp directory: %s\n", tmpDir)
		}

		audioFiles, err := synthesizeSegments(ctx, provider, s.Segments, voices, tmpDir)
		if err != nil {
			return &PipelineError{Stage: "tts", Message: "failed to synthesize audio", Err: err}
		}

		if opts.Verbose {
			fmt.Printf("    TTS time: %s\n", time.Since(stageStart).Round(time.Millisecond))
			var totalBytes int64
			for _, f := range audioFiles {
				if info, err := os.Stat(f); err == nil {
					totalBytes += info.Size()
				}
			}
			fmt.Printf("    Total audio data: %d bytes (%d files)\n", totalBytes, len(audioFiles))
		}

		// Stage 4: Assembly
		stageStart = time.Now()
		fmt.Printf("  Assembling episode...")
		assembler := assembly.NewFFmpegAssembler()
		if err := assembler.Assemble(ctx, audioFiles, tmpDir, opts.Output); err != nil {
			fmt.Println(" failed")
			return &PipelineError{Stage: "assembly", Message: "failed to assemble episode", Err: err}
		}
		fmt.Println(" done")

		if opts.Verbose {
			fmt.Printf("    Assembly time: %s\n", time.Since(stageStart).Round(time.Millisecond))
		}
	}

	// Report final output
	info, err := os.Stat(opts.Output)
	if err == nil {
		sizeMB := float64(info.Size()) / (1024 * 1024)
		duration := probeDuration(opts.Output)
		if duration != "" {
			fmt.Printf("\n  Episode saved to %s (%s, %.1f MB)\n", opts.Output, duration, sizeMB)
		} else {
			fmt.Printf("\n  Episode saved to %s (%.1f MB)\n", opts.Output, sizeMB)
		}
	}

	if opts.Verbose {
		fmt.Printf("    Total pipeline time: %s\n", time.Since(pipelineStart).Round(time.Millisecond))
	}

	return nil
}

// synthesizeSegments runs per-segment TTS with progress output, converting
// non-MP3 formats to MP3 as needed.
func synthesizeSegments(ctx context.Context, provider tts.Provider, segments []script.Segment, voices tts.VoiceMap, tmpDir string) ([]string, error) {
	total := len(segments)
	files := make([]string, 0, total)

	for i, seg := range segments {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}

		voice := tts.VoiceForSpeaker(seg.Speaker, voices)

		pct := (i * 100) / total
		bar := progressBar(pct, 20)
		fmt.Printf("\r  Synthesizing audio... [%d/%d] %s %d%%", i+1, total, bar, pct)

		var result tts.AudioResult
		err := tts.WithRetry(ctx, func() error {
			var synthErr error
			result, synthErr = provider.Synthesize(ctx, seg.Text, voice)
			return synthErr
		})
		if err != nil {
			fmt.Println()
			return nil, fmt.Errorf("segment %d (%s): %w", i+1, seg.Speaker, err)
		}

		// If provider returns non-MP3, convert via FFmpeg
		filename := filepath.Join(tmpDir, fmt.Sprintf("segment_%03d.mp3", i))
		if result.Format != tts.FormatMP3 {
			rawPath := filepath.Join(tmpDir, fmt.Sprintf("segment_%03d.raw", i))
			if err := os.WriteFile(rawPath, result.Data, 0644); err != nil {
				fmt.Println()
				return nil, fmt.Errorf("write raw segment %d: %w", i+1, err)
			}
			if err := assembly.ConvertToMP3(ctx, rawPath, string(result.Format), filename); err != nil {
				fmt.Println()
				return nil, fmt.Errorf("convert segment %d: %w", i+1, err)
			}
		} else {
			if err := os.WriteFile(filename, result.Data, 0644); err != nil {
				fmt.Println()
				return nil, fmt.Errorf("write segment %d: %w", i+1, err)
			}
		}

		files = append(files, filename)
	}

	bar := progressBar(100, 20)
	fmt.Printf("\r  Synthesizing audio... [%d/%d] %s 100%% done\n", total, total, bar)

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
