package tts

import (
	"context"
	"fmt"
	"time"

	"github.com/chad/podcaster/internal/script"
)

// AudioFormat represents the audio encoding returned by a provider.
type AudioFormat string

const (
	FormatMP3  AudioFormat = "mp3"
	FormatPCM  AudioFormat = "pcm"  // raw PCM (needs FFmpeg conversion)
	FormatWAV AudioFormat = "wav"
)

// Voice holds a provider-specific voice identifier.
type Voice struct {
	ID   string // Provider-specific voice identifier
	Name string // Human-readable label
}

// VoiceMap maps podcast hosts to voices.
type VoiceMap struct {
	Alex Voice
	Sam  Voice
}

// AudioResult is the output of a synthesis call.
type AudioResult struct {
	Data   []byte
	Format AudioFormat
}

// Provider synthesizes speech from text segments.
type Provider interface {
	Name() string
	Synthesize(ctx context.Context, text string, voice Voice) (AudioResult, error)
	DefaultVoices() VoiceMap
	Close() error
}

// BatchProvider can synthesize an entire multi-speaker script at once.
// The pipeline prefers this over per-segment synthesis when available.
type BatchProvider interface {
	Provider
	SynthesizeBatch(ctx context.Context, segments []script.Segment, voices VoiceMap) (AudioResult, error)
}

// VoiceInfo describes an available voice for display in the registry.
type VoiceInfo struct {
	ID          string
	Name        string
	Gender      string // "male" or "female"
	Description string
	DefaultFor  string // "Alex", "Sam", or ""
}

// AvailableVoices returns the voice catalog for the named provider.
func AvailableVoices(providerName string) ([]VoiceInfo, error) {
	switch providerName {
	case "elevenlabs":
		return elevenLabsAvailableVoices(), nil
	case "google":
		return googleAvailableVoices(), nil
	case "gemini":
		return geminiAvailableVoices(), nil
	default:
		return nil, fmt.Errorf("unknown TTS provider %q", providerName)
	}
}

// Retry constants shared by all providers.
const (
	defaultMaxAttempts    = 3
	defaultInitialBackoff = 1 * time.Second
	defaultBackoffMulti   = 2
	defaultMaxBackoff     = 10 * time.Second
)

// RetryableError signals that the operation can be retried.
type RetryableError struct {
	StatusCode int
	Body       string
}

func (e *RetryableError) Error() string {
	return fmt.Sprintf("API error (status %d): %s", e.StatusCode, e.Body)
}

// WithRetry executes fn with exponential backoff on RetryableError.
func WithRetry(ctx context.Context, fn func() error) error {
	var lastErr error
	backoff := defaultInitialBackoff

	for attempt := 1; attempt <= defaultMaxAttempts; attempt++ {
		if err := fn(); err == nil {
			return nil
		} else if _, ok := err.(*RetryableError); !ok {
			return err
		} else {
			lastErr = err
		}

		if attempt < defaultMaxAttempts {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(backoff):
			}
			backoff *= time.Duration(defaultBackoffMulti)
			if backoff > defaultMaxBackoff {
				backoff = defaultMaxBackoff
			}
		}
	}

	return lastErr
}

// NewProvider creates a TTS provider by name. voiceAlex and voiceSam are
// optional provider-specific voice ID overrides.
func NewProvider(name string, voiceAlex, voiceSam string) (Provider, error) {
	switch name {
	case "elevenlabs":
		return NewElevenLabsProvider(voiceAlex, voiceSam), nil
	case "google":
		return NewGoogleProvider(voiceAlex, voiceSam)
	case "gemini":
		return NewGeminiProvider(voiceAlex, voiceSam), nil
	default:
		return nil, fmt.Errorf("unknown TTS provider %q: choose elevenlabs, google, or gemini", name)
	}
}
