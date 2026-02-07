package tts

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/chad/podcaster/internal/script"
)

const (
	DefaultVoiceAlex = "JBFqnCBsd6RMkjVDRZzb" // George
	DefaultVoiceSam  = "EXAVITQu4vr4xnSDxMaL"  // Sarah

	apiBaseURL   = "https://api.elevenlabs.io/v1/text-to-speech"
	modelID      = "eleven_multilingual_v2"
	outputFormat = "mp3_44100_128"

	maxAttempts    = 3
	initialBackoff = 1 * time.Second
	backoffMulti   = 2
	maxBackoff     = 10 * time.Second
)

type ttsRequest struct {
	Text          string         `json:"text"`
	ModelID       string         `json:"model_id"`
	VoiceSettings *voiceSettings `json:"voice_settings,omitempty"`
}

type voiceSettings struct {
	Stability       float64 `json:"stability"`
	SimilarityBoost float64 `json:"similarity_boost"`
	Style           float64 `json:"style"`
	UseSpeakerBoost bool    `json:"use_speaker_boost"`
	Speed           float64 `json:"speed"`
}

type RetryableError struct {
	StatusCode int
	Body       string
}

func (e *RetryableError) Error() string {
	return fmt.Sprintf("ElevenLabs API error (status %d): %s", e.StatusCode, e.Body)
}

type ElevenLabsClient struct {
	voiceAlex  string
	voiceSam   string
	apiKey     string
	httpClient *http.Client
}

func NewElevenLabsClient(voiceAlex, voiceSam string) *ElevenLabsClient {
	if voiceAlex == "" {
		voiceAlex = DefaultVoiceAlex
	}
	if voiceSam == "" {
		voiceSam = DefaultVoiceSam
	}
	return &ElevenLabsClient{
		voiceAlex:  voiceAlex,
		voiceSam:   voiceSam,
		apiKey:     os.Getenv("ELEVENLABS_API_KEY"),
		httpClient: &http.Client{Timeout: 60 * time.Second},
	}
}

func (c *ElevenLabsClient) VoiceAlexID() string { return c.voiceAlex }
func (c *ElevenLabsClient) VoiceSamID() string  { return c.voiceSam }

func (c *ElevenLabsClient) Synthesize(ctx context.Context, segment script.Segment, voiceID string) ([]byte, error) {
	reqBody := ttsRequest{
		Text:    segment.Text,
		ModelID: modelID,
		VoiceSettings: &voiceSettings{
			Stability:       0.5,
			SimilarityBoost: 0.75,
			Style:           0.0,
			UseSpeakerBoost: true,
			Speed:           1.0,
		},
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/%s?output_format=%s", apiBaseURL, voiceID, outputFormat)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("xi-api-key", c.apiKey)
	req.Header.Set("Content-Type", "application/json")

	res, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}
	defer res.Body.Close()

	if res.StatusCode == http.StatusTooManyRequests ||
		res.StatusCode >= http.StatusInternalServerError {
		errBody, _ := io.ReadAll(res.Body)
		return nil, &RetryableError{
			StatusCode: res.StatusCode,
			Body:       string(errBody),
		}
	}

	if res.StatusCode != http.StatusOK {
		errBody, _ := io.ReadAll(res.Body)
		return nil, fmt.Errorf("ElevenLabs API error (status %d): %s", res.StatusCode, string(errBody))
	}

	return io.ReadAll(res.Body)
}

func (c *ElevenLabsClient) SynthesizeAll(ctx context.Context, segments []script.Segment, tmpDir string) ([]string, error) {
	total := len(segments)
	files := make([]string, 0, total)

	for i, seg := range segments {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}

		voiceID := GetVoiceID(seg.Speaker, c.voiceAlex, c.voiceSam)

		// Progress output
		pct := (i * 100) / total
		bar := progressBar(pct, 20)
		fmt.Printf("\r  Synthesizing audio... [%d/%d] %s %d%%", i+1, total, bar, pct)

		audio, err := c.synthesizeWithRetry(ctx, seg, voiceID)
		if err != nil {
			fmt.Println()
			return nil, fmt.Errorf("segment %d (%s): %w", i+1, seg.Speaker, err)
		}

		filename := filepath.Join(tmpDir, fmt.Sprintf("segment_%03d.mp3", i))
		if err := os.WriteFile(filename, audio, 0644); err != nil {
			fmt.Println()
			return nil, fmt.Errorf("write segment %d: %w", i+1, err)
		}

		files = append(files, filename)
	}

	// Final progress
	bar := progressBar(100, 20)
	fmt.Printf("\r  Synthesizing audio... [%d/%d] %s 100%% done\n", total, total, bar)

	return files, nil
}

func (c *ElevenLabsClient) synthesizeWithRetry(ctx context.Context, seg script.Segment, voiceID string) ([]byte, error) {
	var lastErr error
	backoff := initialBackoff

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		audio, err := c.Synthesize(ctx, seg, voiceID)
		if err == nil {
			return audio, nil
		}

		if _, ok := err.(*RetryableError); !ok {
			return nil, err // Non-retryable error
		}

		lastErr = err
		if attempt < maxAttempts {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff):
			}
			backoff *= time.Duration(backoffMulti)
			if backoff > maxBackoff {
				backoff = maxBackoff
			}
		}
	}

	return nil, lastErr
}

func progressBar(pct, width int) string {
	filled := (pct * width) / 100
	if filled > width {
		filled = width
	}
	empty := width - filled
	return fmt.Sprintf("%s%s", repeat("█", filled), repeat("░", empty))
}

func repeat(s string, n int) string {
	result := ""
	for i := 0; i < n; i++ {
		result += s
	}
	return result
}
