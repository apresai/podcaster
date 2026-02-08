package tts

import (
	"context"
	"fmt"
	"os"
	"time"

	texttospeech "cloud.google.com/go/texttospeech/apiv1"
	texttospeechpb "cloud.google.com/go/texttospeech/apiv1/texttospeechpb"
)

const (
	googleDefaultVoice1 = "en-US-Chirp3-HD-Charon"
	googleDefaultVoice2 = "en-US-Chirp3-HD-Leda"
	googleDefaultVoice3 = "en-US-Chirp3-HD-Fenrir"
)

// GoogleProvider implements Provider using Google Cloud TTS (Chirp 3 HD).
type GoogleProvider struct {
	voices VoiceMap
	client *texttospeech.Client
	speed  float64
	pitch  float64
}

func NewGoogleProvider(voice1, voice2, voice3 string, cfg ProviderConfig) (*GoogleProvider, error) {
	v1 := googleDefaultVoice1
	v2 := googleDefaultVoice2
	v3 := googleDefaultVoice3
	if voice1 != "" {
		v1 = voice1
	}
	if voice2 != "" {
		v2 = voice2
	}
	if voice3 != "" {
		v3 = voice3
	}

	client, err := texttospeech.NewClient(context.Background())
	if err != nil {
		return nil, fmt.Errorf("create Google TTS client: %w", err)
	}

	return &GoogleProvider{
		voices: VoiceMap{
			Host1: Voice{ID: v1, Name: "Charon"},
			Host2: Voice{ID: v2, Name: "Leda"},
			Host3: Voice{ID: v3, Name: "Fenrir"},
		},
		client: client,
		speed:  cfg.Speed,
		pitch:  cfg.Pitch,
	}, nil
}

func (p *GoogleProvider) Name() string { return "google" }

func (p *GoogleProvider) DefaultVoices() VoiceMap {
	return VoiceMap{
		Host1: Voice{ID: googleDefaultVoice1, Name: "Charon"},
		Host2: Voice{ID: googleDefaultVoice2, Name: "Leda"},
		Host3: Voice{ID: googleDefaultVoice3, Name: "Fenrir"},
	}
}

func (p *GoogleProvider) Synthesize(ctx context.Context, text string, voice Voice) (AudioResult, error) {
	start := time.Now()
	req := &texttospeechpb.SynthesizeSpeechRequest{
		Input: &texttospeechpb.SynthesisInput{
			InputSource: &texttospeechpb.SynthesisInput_Text{Text: text},
		},
		Voice: &texttospeechpb.VoiceSelectionParams{
			LanguageCode: "en-US",
			Name:         voice.ID,
		},
		AudioConfig: p.audioConfig(),
	}

	resp, err := p.client.SynthesizeSpeech(ctx, req)
	if err != nil {
		return AudioResult{}, fmt.Errorf("Google TTS synthesize: %w", err)
	}

	fmt.Fprintf(os.Stderr, "    Google TTS: %d chars → %d bytes (%s)\n", len(text), len(resp.AudioContent), time.Since(start).Round(time.Millisecond))
	return AudioResult{Data: resp.AudioContent, Format: FormatMP3}, nil
}

func (p *GoogleProvider) audioConfig() *texttospeechpb.AudioConfig {
	cfg := &texttospeechpb.AudioConfig{
		AudioEncoding: texttospeechpb.AudioEncoding_MP3,
	}
	if p.speed != 0 {
		cfg.SpeakingRate = p.speed
	}
	if p.pitch != 0 {
		cfg.Pitch = p.pitch
	}
	return cfg
}

func (p *GoogleProvider) Close() error { return p.client.Close() }

func googleAvailableVoices() []VoiceInfo {
	return []VoiceInfo{
		{ID: "en-US-Chirp3-HD-Charon", Name: "Charon", Gender: "male", Description: "Informative, clear male narrator", DefaultFor: "Voice 1"},
		{ID: "en-US-Chirp3-HD-Leda", Name: "Leda", Gender: "female", Description: "Youthful, bright female voice", DefaultFor: "Voice 2"},
		{ID: "en-US-Chirp3-HD-Fenrir", Name: "Fenrir", Gender: "male", Description: "Deep, resonant male voice", DefaultFor: "Voice 3"},
		{ID: "en-US-Chirp3-HD-Kore", Name: "Kore", Gender: "female", Description: "Firm, confident female voice"},
		{ID: "en-US-Chirp3-HD-Aoede", Name: "Aoede", Gender: "female", Description: "Bright, expressive female voice"},
		{ID: "en-US-Chirp3-HD-Puck", Name: "Puck", Gender: "male", Description: "Upbeat, energetic male voice"},
		{ID: "en-US-Chirp3-HD-Orus", Name: "Orus", Gender: "male", Description: "Warm, steady male narrator"},
		{ID: "en-US-Chirp3-HD-Zephyr", Name: "Zephyr", Gender: "female", Description: "Breezy, relaxed female voice"},
	}
}
