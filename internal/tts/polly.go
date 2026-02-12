package tts

import (
	"context"
	"fmt"
	"io"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/polly"
	"github.com/aws/aws-sdk-go-v2/service/polly/types"
)

const (
	pollyDefaultVoice1 = "Matthew"
	pollyDefaultVoice2 = "Ruth"
	pollyDefaultVoice3 = "Amy"
)

// pollyVoiceLang maps voice IDs to their language codes.
var pollyVoiceLang = map[string]types.LanguageCode{
	"Matthew":  types.LanguageCodeEnUs,
	"Ruth":     types.LanguageCodeEnUs,
	"Stephen":  types.LanguageCodeEnUs,
	"Danielle": types.LanguageCodeEnUs,
	"Amy":      types.LanguageCodeEnGb,
	"Olivia":   types.LanguageCodeEnAu,
	"Kajal":    types.LanguageCodeEnIn,
}

// PollyProvider implements Provider using AWS Polly (Generative engine).
type PollyProvider struct {
	voices VoiceMap
	client *polly.Client
}

func NewPollyProvider(voice1, voice2, voice3 string, cfg ProviderConfig) (*PollyProvider, error) {
	v1 := pollyDefaultVoice1
	v2 := pollyDefaultVoice2
	v3 := pollyDefaultVoice3
	if voice1 != "" {
		v1 = voice1
	}
	if voice2 != "" {
		v2 = voice2
	}
	if voice3 != "" {
		v3 = voice3
	}

	awsCfg, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		return nil, fmt.Errorf("load AWS config for Polly: %w", err)
	}

	return &PollyProvider{
		voices: VoiceMap{
			Host1: Voice{ID: v1, Name: v1},
			Host2: Voice{ID: v2, Name: v2},
			Host3: Voice{ID: v3, Name: v3},
		},
		client: polly.NewFromConfig(awsCfg),
	}, nil
}

func (p *PollyProvider) Name() string { return "polly" }

func (p *PollyProvider) DefaultVoices() VoiceMap {
	return VoiceMap{
		Host1: Voice{ID: pollyDefaultVoice1, Name: pollyDefaultVoice1},
		Host2: Voice{ID: pollyDefaultVoice2, Name: pollyDefaultVoice2},
		Host3: Voice{ID: pollyDefaultVoice3, Name: pollyDefaultVoice3},
	}
}

func (p *PollyProvider) Synthesize(ctx context.Context, text string, voice Voice) (AudioResult, error) {
	lang, ok := pollyVoiceLang[voice.ID]
	if !ok {
		lang = types.LanguageCodeEnUs
	}

	engine := types.EngineGenerative
	input := &polly.SynthesizeSpeechInput{
		Engine:       engine,
		OutputFormat: types.OutputFormatMp3,
		SampleRate:   strPtr("24000"),
		Text:         &text,
		TextType:     types.TextTypeText,
		VoiceId:      types.VoiceId(voice.ID),
		LanguageCode: lang,
	}

	resp, err := p.client.SynthesizeSpeech(ctx, input)
	if err != nil {
		// Wrap throttling as retryable
		return AudioResult{}, fmt.Errorf("Polly synthesize: %w", err)
	}
	defer resp.AudioStream.Close()

	data, err := io.ReadAll(resp.AudioStream)
	if err != nil {
		return AudioResult{}, fmt.Errorf("Polly read audio: %w", err)
	}

	return AudioResult{Data: data, Format: FormatMP3}, nil
}

func (p *PollyProvider) Close() error { return nil }

func strPtr(s string) *string { return &s }

func pollyAvailableVoices() []VoiceInfo {
	return []VoiceInfo{
		{ID: "Matthew", Name: "Matthew", Gender: "male", Description: "en-US, Generative", DefaultFor: "Voice 1"},
		{ID: "Ruth", Name: "Ruth", Gender: "female", Description: "en-US, Generative", DefaultFor: "Voice 2"},
		{ID: "Amy", Name: "Amy", Gender: "female", Description: "en-GB, Generative", DefaultFor: "Voice 3"},
		{ID: "Stephen", Name: "Stephen", Gender: "male", Description: "en-US, Generative"},
		{ID: "Danielle", Name: "Danielle", Gender: "female", Description: "en-US, Generative"},
		{ID: "Olivia", Name: "Olivia", Gender: "female", Description: "en-AU, Generative"},
		{ID: "Kajal", Name: "Kajal", Gender: "female", Description: "en-IN, Generative"},
	}
}
