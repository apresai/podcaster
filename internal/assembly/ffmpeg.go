package assembly

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Audio quality constants for consistent output across all FFmpeg operations.
const (
	AudioBitrate    = "192k"
	AudioSampleRate = "44100"
	AudioChannels   = "2"
	AudioCodec      = "libmp3lame"
	AudioQuality    = "0" // LAME quality (0 = best)
	AudioResampler  = "aresample=resampler=soxr"
)

type Assembler interface {
	Assemble(ctx context.Context, segments []string, tmpDir string, output string) error
}

type FFmpegAssembler struct{}

func NewFFmpegAssembler() *FFmpegAssembler {
	return &FFmpegAssembler{}
}

func (a *FFmpegAssembler) Assemble(ctx context.Context, segments []string, tmpDir string, output string) error {
	if len(segments) == 0 {
		return fmt.Errorf("no audio segments to assemble")
	}

	// Generate silence file (200ms)
	silencePath := filepath.Join(tmpDir, "silence.mp3")
	if err := generateSilence(ctx, silencePath); err != nil {
		return fmt.Errorf("generate silence: %w", err)
	}

	// Build concat list
	listPath := filepath.Join(tmpDir, "concat.txt")
	if err := buildConcatList(segments, silencePath, listPath); err != nil {
		return fmt.Errorf("build concat list: %w", err)
	}

	// Run FFmpeg concat
	if err := runFFmpegConcat(ctx, listPath, output); err != nil {
		return fmt.Errorf("ffmpeg concat: %w", err)
	}

	return nil
}

func generateSilence(ctx context.Context, output string) error {
	cmd := exec.CommandContext(ctx, "ffmpeg",
		"-f", "lavfi",
		"-i", fmt.Sprintf("anullsrc=r=%s:cl=stereo", AudioSampleRate),
		"-t", "0.2",
		"-c:a", AudioCodec,
		"-b:a", AudioBitrate,
		"-y",
		output,
	)
	var stderr strings.Builder
	cmd.Stderr = &stderr
	cmd.Stdout = nil

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("ffmpeg silence generation failed: %w\n%s", err, stderr.String())
	}
	return nil
}

func buildConcatList(segments []string, silencePath string, listPath string) error {
	var lines []string
	for i, seg := range segments {
		lines = append(lines, fmt.Sprintf("file '%s'", seg))
		// Add silence between segments (not after the last one)
		if i < len(segments)-1 {
			lines = append(lines, fmt.Sprintf("file '%s'", silencePath))
		}
	}

	content := strings.Join(lines, "\n") + "\n"
	if err := os.WriteFile(listPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("write concat list: %w", err)
	}
	return nil
}

// ConvertToMP3 converts raw audio (PCM/LPCM/WAV) to MP3 via FFmpeg.
// The format parameter determines the input interpretation:
//   - "pcm":  raw 24kHz 16-bit signed little-endian mono
//   - "lpcm": raw 24kHz 16-bit signed little-endian mono (same as pcm)
//   - "wav":  standard WAV header (auto-detected by FFmpeg)
func ConvertToMP3(ctx context.Context, input string, format string, output string) error {
	var args []string
	switch format {
	case "pcm", "lpcm":
		args = []string{
			"-f", "s16le",
			"-ar", "24000",
			"-ac", "1",
			"-i", input,
			"-af", AudioResampler,
			"-c:a", AudioCodec,
			"-b:a", AudioBitrate,
			"-q:a", AudioQuality,
			"-ar", AudioSampleRate,
			"-ac", AudioChannels,
			"-y",
			output,
		}
	case "wav":
		args = []string{
			"-i", input,
			"-af", AudioResampler,
			"-c:a", AudioCodec,
			"-b:a", AudioBitrate,
			"-q:a", AudioQuality,
			"-ar", AudioSampleRate,
			"-ac", AudioChannels,
			"-y",
			output,
		}
	default:
		return fmt.Errorf("unsupported audio format for conversion: %s", format)
	}

	cmd := exec.CommandContext(ctx, "ffmpeg", args...)
	var stderr strings.Builder
	cmd.Stderr = &stderr
	cmd.Stdout = nil

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("ffmpeg conversion (%s → mp3) failed: %w\n%s", format, err, stderr.String())
	}
	return nil
}

func runFFmpegConcat(ctx context.Context, listPath string, output string) error {
	cmd := exec.CommandContext(ctx, "ffmpeg",
		"-f", "concat",
		"-safe", "0",
		"-i", listPath,
		"-af", AudioResampler,
		"-c:a", AudioCodec,
		"-b:a", AudioBitrate,
		"-q:a", AudioQuality,
		"-ar", AudioSampleRate,
		"-ac", AudioChannels,
		"-y",
		output,
	)
	var stderr strings.Builder
	cmd.Stderr = &stderr
	cmd.Stdout = nil

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("ffmpeg concat failed: %w\n%s", err, stderr.String())
	}

	// Verify output exists and has non-zero size
	info, err := os.Stat(output)
	if err != nil {
		return fmt.Errorf("output file not created: %w", err)
	}
	if info.Size() == 0 {
		return fmt.Errorf("output file is empty")
	}

	return nil
}
