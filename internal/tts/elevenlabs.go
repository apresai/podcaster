package tts

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

const (
	elevenLabsDefaultVoiceAlex = "JBFqnCBsd6RMkjVDRZzb" // George
	elevenLabsDefaultVoiceSam  = "EXAVITQu4vr4xnSDxMaL" // Sarah

	elevenLabsBaseURL      = "https://api.elevenlabs.io/v1/text-to-speech"
	elevenLabsModelID      = "eleven_flash_v2_5"
	elevenLabsOutputFormat = "mp3_44100_128"
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
			Alex: Voice{ID: alexID, Name: "George"},
			Sam:  Voice{ID: samID, Name: "Sarah"},
		},
		apiKey:     os.Getenv("ELEVENLABS_API_KEY"),
		httpClient: &http.Client{Timeout: 60 * time.Second},
	}
}

func (p *ElevenLabsProvider) Name() string { return "elevenlabs" }

func (p *ElevenLabsProvider) DefaultVoices() VoiceMap {
	return VoiceMap{
		Alex: Voice{ID: elevenLabsDefaultVoiceAlex, Name: "George"},
		Sam:  Voice{ID: elevenLabsDefaultVoiceSam, Name: "Sarah"},
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

func elevenLabsAvailableVoices() []VoiceInfo {
	return []VoiceInfo{
		{ID: "JBFqnCBsd6RMkjVDRZzb", Name: "George", Gender: "male", Description: "Warm British male, clear and authoritative", DefaultFor: "Alex"},
		{ID: "EXAVITQu4vr4xnSDxMaL", Name: "Sarah", Gender: "female", Description: "Soft American female, friendly and engaging", DefaultFor: "Sam"},
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
