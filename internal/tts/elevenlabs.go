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
	elevenLabsDefaultVoiceAlex = "xuqUPASjAdyZvCNoMTEj"  // Chad
	elevenLabsDefaultVoiceSam  = "56bWURjYFHyYyVf490Dp" // Emma

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

func NewElevenLabsProvider(voiceAlex, voiceSam string) *ElevenLabsProvider {
	alexID := elevenLabsDefaultVoiceAlex
	samID := elevenLabsDefaultVoiceSam
	if voiceAlex != "" {
		alexID = voiceAlex
	}
	if voiceSam != "" {
		samID = voiceSam
	}
	return &ElevenLabsProvider{
		voices: VoiceMap{
			Alex: Voice{ID: alexID, Name: "Chad"},
			Sam:  Voice{ID: samID, Name: "Emma"},
		},
		apiKey:     os.Getenv("ELEVENLABS_API_KEY"),
		httpClient: &http.Client{Timeout: 60 * time.Second},
	}
}

func (p *ElevenLabsProvider) Name() string { return "elevenlabs" }

func (p *ElevenLabsProvider) DefaultVoices() VoiceMap {
	return VoiceMap{
		Alex: Voice{ID: elevenLabsDefaultVoiceAlex, Name: "Chad"},
		Sam:  Voice{ID: elevenLabsDefaultVoiceSam, Name: "Emma"},
	}
}

func (p *ElevenLabsProvider) Synthesize(ctx context.Context, text string, voice Voice) (AudioResult, error) {
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

		// Build description from labels and top-level description.
		var parts []string
		if accent := v.Labels["accent"]; accent != "" {
			parts = append(parts, accent)
		}
		if desc := v.Labels["description"]; desc != "" {
			parts = append(parts, desc)
		}
		if v.Category != "" {
			parts = append(parts, v.Category)
		}
		if v.Description != "" {
			parts = append(parts, v.Description)
		}
		info.Description = strings.Join(parts, ", ")

		// Tag default voices.
		switch v.VoiceID {
		case elevenLabsDefaultVoiceAlex:
			info.DefaultFor = "Alex"
		case elevenLabsDefaultVoiceSam:
			info.DefaultFor = "Sam"
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
		{ID: "xuqUPASjAdyZvCNoMTEj", Name: "Chad", Gender: "male", Description: "Cloned voice", DefaultFor: "Alex"},
		{ID: "56bWURjYFHyYyVf490Dp", Name: "Emma", Gender: "female", Description: "Warm Australian female, engaging and conversational", DefaultFor: "Sam"},
		{ID: "JBFqnCBsd6RMkjVDRZzb", Name: "George", Gender: "male", Description: "Warm British male, clear and authoritative"},
		{ID: "EXAVITQu4vr4xnSDxMaL", Name: "Sarah", Gender: "female", Description: "Soft American female, friendly and engaging"},
		{ID: "pNInz6obpgDQGcFmaJgB", Name: "Adam", Gender: "male", Description: "Deep American male, confident narrator"},
		{ID: "ErXwobaYiN019PkySvjV", Name: "Antoni", Gender: "male", Description: "Young American male, conversational"},
		{ID: "MF3mGyEYCl7XYWbV9V6O", Name: "Elli", Gender: "female", Description: "Young American female, bright and expressive"},
		{ID: "TxGEqnHWrfWFTfGW9XjX", Name: "Josh", Gender: "male", Description: "Young American male, deep and smooth"},
		{ID: "VR6AewLTigWG4xSOukaG", Name: "Arnold", Gender: "male", Description: "Deep gravelly male, commanding presence"},
		{ID: "onwK4e9ZLuTAKqWW03F9", Name: "Daniel", Gender: "male", Description: "British male, authoritative news anchor"},
		{ID: "XB0fDUnXU5powFXDhCwa", Name: "Charlotte", Gender: "female", Description: "Swedish-English female, warm and natural"},
		{ID: "pFZP5JQG7iQjIQuC4Bku", Name: "Lily", Gender: "female", Description: "British female, warm storyteller"},
	}
}
