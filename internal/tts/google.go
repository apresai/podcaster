package tts

import (
	"context"
	"fmt"

	texttospeech "cloud.google.com/go/texttospeech/apiv1"
	texttospeechpb "cloud.google.com/go/texttospeech/apiv1/texttospeechpb"
)

const (
	googleDefaultVoiceAlex = "en-US-Chirp3-HD-Charon"
	googleDefaultVoiceSam  = "en-US-Chirp3-HD-Leda"
)

// GoogleProvider implements Provider using Google Cloud TTS (Chirp 3 HD).
type GoogleProvider struct {
	voices VoiceMap
	client *texttospeech.Client
}

func NewGoogleProvider(voiceAlex, voiceSam string) (*GoogleProvider, error) {
	alexID := googleDefaultVoiceAlex
	samID := googleDefaultVoiceSam
	if voiceAlex != "" {
		alexID = voiceAlex
	}
	if voiceSam != "" {
		samID = voiceSam
	}

	client, err := texttospeech.NewClient(context.Background())
	if err != nil {
		return nil, fmt.Errorf("create Google TTS client: %w", err)
	}

	return &GoogleProvider{
		voices: VoiceMap{
			Alex: Voice{ID: alexID, Name: "Charon"},
			Sam:  Voice{ID: samID, Name: "Leda"},
		},
		client: client,
	}, nil
}

func (p *GoogleProvider) Name() string { return "google" }

func (p *GoogleProvider) DefaultVoices() VoiceMap {
	return VoiceMap{
		Alex: Voice{ID: googleDefaultVoiceAlex, Name: "Charon"},
		Sam:  Voice{ID: googleDefaultVoiceSam, Name: "Leda"},
	}
}

func (p *GoogleProvider) Synthesize(ctx context.Context, text string, voice Voice) (AudioResult, error) {
	req := &texttospeechpb.SynthesizeSpeechRequest{
		Input: &texttospeechpb.SynthesisInput{
			InputSource: &texttospeechpb.SynthesisInput_Text{Text: text},
		},
		Voice: &texttospeechpb.VoiceSelectionParams{
			LanguageCode: "en-US",
			Name:         voice.ID,
		},
		AudioConfig: &texttospeechpb.AudioConfig{
			AudioEncoding: texttospeechpb.AudioEncoding_MP3,
		},
	}

	resp, err := p.client.SynthesizeSpeech(ctx, req)
	if err != nil {
		return AudioResult{}, fmt.Errorf("Google TTS synthesize: %w", err)
	}

	return AudioResult{Data: resp.AudioContent, Format: FormatMP3}, nil
}

func (p *GoogleProvider) Close() error { return p.client.Close() }

func googleAvailableVoices() []VoiceInfo {
	return []VoiceInfo{
		{ID: "en-US-Chirp3-HD-Charon", Name: "Charon", Gender: "male", Description: "Informative, clear male narrator", DefaultFor: "Alex"},
		{ID: "en-US-Chirp3-HD-Leda", Name: "Leda", Gender: "female", Description: "Youthful, bright female voice", DefaultFor: "Sam"},
		{ID: "en-US-Chirp3-HD-Kore", Name: "Kore", Gender: "female", Description: "Firm, confident female voice"},
		{ID: "en-US-Chirp3-HD-Fenrir", Name: "Fenrir", Gender: "male", Description: "Deep, resonant male voice"},
		{ID: "en-US-Chirp3-HD-Aoede", Name: "Aoede", Gender: "female", Description: "Bright, expressive female voice"},
		{ID: "en-US-Chirp3-HD-Puck", Name: "Puck", Gender: "male", Description: "Upbeat, energetic male voice"},
		{ID: "en-US-Chirp3-HD-Orus", Name: "Orus", Gender: "male", Description: "Warm, steady male narrator"},
		{ID: "en-US-Chirp3-HD-Zephyr", Name: "Zephyr", Gender: "female", Description: "Breezy, relaxed female voice"},
	}
}
