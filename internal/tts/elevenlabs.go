package tts

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

const (
	elevenLabsDefaultVoice1 = "xuqUPASjAdyZvCNoMTEj"  // Chad
	elevenLabsDefaultVoice2 = "56bWURjYFHyYyVf490Dp"  // Emma
	elevenLabsDefaultVoice3 = "TxGEqnHWrfWFTfGW9XjX"  // Josh

	elevenLabsBaseURL      = "https://api.elevenlabs.io/v1/text-to-speech"
	elevenLabsVoicesURL    = "https://api.elevenlabs.io/v1/voices"
	elevenLabsModelID      = "eleven_flash_v2_5"
	elevenLabsOutputFormat = "mp3_44100_192"
)

type elevenLabsRequest struct {
	Text          string                 `json:"text"`
	ModelID       string                 `json:"model_id"`
	VoiceSettings *elevenLabsVoiceParams `json:"voice_settings,omitempty"`
}

type elevenLabsVoiceParams struct {
	Stability       float64 `json:"stability"`
	SimilarityBoost float64 `json:"similarity_boost"`
	Style           float64 `json:"style"`
	UseSpeakerBoost bool    `json:"use_speaker_boost"`
	Speed           float64 `json:"speed"`
}

// ElevenLabsProvider implements Provider using the ElevenLabs TTS API.
type ElevenLabsProvider struct {
	voices     VoiceMap
	apiKey     string
	httpClient *http.Client
}

func NewElevenLabsProvider(voice1, voice2, voice3 string) *ElevenLabsProvider {
	v1 := elevenLabsDefaultVoice1
	v2 := elevenLabsDefaultVoice2
	v3 := elevenLabsDefaultVoice3
	if voice1 != "" {
		v1 = voice1
	}
	if voice2 != "" {
		v2 = voice2
	}
	if voice3 != "" {
		v3 = voice3
	}
	return &ElevenLabsProvider{
		voices: VoiceMap{
			Host1: Voice{ID: v1, Name: "Chad"},
			Host2: Voice{ID: v2, Name: "Emma"},
			Host3: Voice{ID: v3, Name: "Josh"},
		},
		apiKey:     os.Getenv("ELEVENLABS_API_KEY"),
		httpClient: &http.Client{Timeout: 60 * time.Second},
	}
}

func (p *ElevenLabsProvider) Name() string { return "elevenlabs" }

func (p *ElevenLabsProvider) DefaultVoices() VoiceMap {
	return VoiceMap{
		Host1: Voice{ID: elevenLabsDefaultVoice1, Name: "Chad"},
		Host2: Voice{ID: elevenLabsDefaultVoice2, Name: "Emma"},
		Host3: Voice{ID: elevenLabsDefaultVoice3, Name: "Josh"},
	}
}

func (p *ElevenLabsProvider) Synthesize(ctx context.Context, text string, voice Voice) (AudioResult, error) {
	start := time.Now()
	reqBody := elevenLabsRequest{
		Text:    text,
		ModelID: elevenLabsModelID,
		VoiceSettings: &elevenLabsVoiceParams{
			Stability:       0.5,
			SimilarityBoost: 0.75,
			Style:           0.0,
			UseSpeakerBoost: true,
			Speed:           1.0,
		},
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return AudioResult{}, fmt.Errorf("marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/%s?output_format=%s", elevenLabsBaseURL, voice.ID, elevenLabsOutputFormat)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return AudioResult{}, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("xi-api-key", p.apiKey)
	req.Header.Set("Content-Type", "application/json")

	res, err := p.httpClient.Do(req)
	if err != nil {
		return AudioResult{}, fmt.Errorf("send request: %w", err)
	}
	defer res.Body.Close()

	if res.StatusCode == http.StatusTooManyRequests ||
		res.StatusCode >= http.StatusInternalServerError {
		errBody, _ := io.ReadAll(res.Body)
		return AudioResult{}, &RetryableError{
			StatusCode: res.StatusCode,
			Body:       string(errBody),
		}
	}

	if res.StatusCode != http.StatusOK {
		errBody, _ := io.ReadAll(res.Body)
		return AudioResult{}, fmt.Errorf("ElevenLabs API error (status %d): %s", res.StatusCode, string(errBody))
	}

	data, err := io.ReadAll(res.Body)
	if err != nil {
		return AudioResult{}, fmt.Errorf("read response: %w", err)
	}

	fmt.Fprintf(os.Stderr, "    ElevenLabs: %d chars → %d bytes (%s)\n", len(text), len(data), time.Since(start).Round(time.Millisecond))
	return AudioResult{Data: data, Format: FormatMP3}, nil
}

func (p *ElevenLabsProvider) Close() error { return nil }

// elevenLabsVoicesResponse is the API response from GET /v1/voices.
type elevenLabsVoicesResponse struct {
	Voices []elevenLabsVoice `json:"voices"`
}

type elevenLabsVoice struct {
	VoiceID     string            `json:"voice_id"`
	Name        string            `json:"name"`
	Category    string            `json:"category"`
	Description string            `json:"description"`
	Labels      map[string]string `json:"labels"`
}

// fetchElevenLabsVoices calls the ElevenLabs API to get the user's voice library.
func fetchElevenLabsVoices(apiKey string) ([]VoiceInfo, error) {
	client := &http.Client{Timeout: 10 * time.Second}

	req, err := http.NewRequest(http.MethodGet, elevenLabsVoicesURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("xi-api-key", apiKey)

	res, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch voices: %w", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(res.Body)
		return nil, fmt.Errorf("ElevenLabs voices API error (status %d): %s", res.StatusCode, string(body))
	}

	var resp elevenLabsVoicesResponse
	if err := json.NewDecoder(res.Body).Decode(&resp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	voices := make([]VoiceInfo, 0, len(resp.Voices))
	for _, v := range resp.Voices {
		info := VoiceInfo{
			ID:     v.VoiceID,
			Name:   v.Name,
			Gender: v.Labels["gender"],
		}

		// Build brief description from accent + description labels only.
		var parts []string
		if accent := v.Labels["accent"]; accent != "" {
			parts = append(parts, accent)
		}
		if desc := v.Labels["description"]; desc != "" {
			parts = append(parts, desc)
		}
		info.Description = strings.Join(parts, ", ")

		// Tag default voices.
		switch v.VoiceID {
		case elevenLabsDefaultVoice1:
			info.DefaultFor = "Voice 1"
		case elevenLabsDefaultVoice2:
			info.DefaultFor = "Voice 2"
		case elevenLabsDefaultVoice3:
			info.DefaultFor = "Voice 3"
		}

		voices = append(voices, info)
	}

	return voices, nil
}

func elevenLabsAvailableVoices() []VoiceInfo {
	// Try live fetch if API key is available.
	if apiKey := os.Getenv("ELEVENLABS_API_KEY"); apiKey != "" {
		if voices, err := fetchElevenLabsVoices(apiKey); err == nil {
			return voices
		}
	}

	// Fallback to hardcoded list.
	return []VoiceInfo{
		{ID: "xuqUPASjAdyZvCNoMTEj", Name: "Chad", Gender: "male", Description: "Cloned voice", DefaultFor: "Voice 1"},
		{ID: "56bWURjYFHyYyVf490Dp", Name: "Emma", Gender: "female", Description: "Warm Australian female", DefaultFor: "Voice 2"},
		{ID: "TxGEqnHWrfWFTfGW9XjX", Name: "Josh", Gender: "male", Description: "Deep, smooth male", DefaultFor: "Voice 3"},
		{ID: "JBFqnCBsd6RMkjVDRZzb", Name: "George", Gender: "male", Description: "Warm British male"},
		{ID: "EXAVITQu4vr4xnSDxMaL", Name: "Sarah", Gender: "female", Description: "Soft American female"},
		{ID: "pNInz6obpgDQGcFmaJgB", Name: "Adam", Gender: "male", Description: "Deep American male"},
		{ID: "ErXwobaYiN019PkySvjV", Name: "Antoni", Gender: "male", Description: "Young, conversational male"},
		{ID: "MF3mGyEYCl7XYWbV9V6O", Name: "Elli", Gender: "female", Description: "Bright, expressive female"},
		{ID: "VR6AewLTigWG4xSOukaG", Name: "Arnold", Gender: "male", Description: "Deep, gravelly male"},
		{ID: "onwK4e9ZLuTAKqWW03F9", Name: "Daniel", Gender: "male", Description: "Authoritative British male"},
		{ID: "XB0fDUnXU5powFXDhCwa", Name: "Charlotte", Gender: "female", Description: "Warm Swedish-English female"},
		{ID: "pFZP5JQG7iQjIQuC4Bku", Name: "Lily", Gender: "female", Description: "Warm British female"},
	}
}
