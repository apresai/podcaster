package tts

import (
	"context"

	"github.com/chad/podcaster/internal/script"
)

type TTSClient interface {
	Synthesize(ctx context.Context, segment script.Segment, voiceID string) ([]byte, error)
	SynthesizeAll(ctx context.Context, segments []script.Segment, tmpDir string) ([]string, error)
}

type VoiceConfig struct {
	VoiceID string
	Speaker string
}

func GetVoiceID(speaker string, voiceAlex, voiceSam string) string {
	switch speaker {
	case "Alex":
		if voiceAlex != "" {
			return voiceAlex
		}
		return DefaultVoiceAlex
	case "Sam":
		if voiceSam != "" {
			return voiceSam
		}
		return DefaultVoiceSam
	default:
		return DefaultVoiceAlex
	}
}
