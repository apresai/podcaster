package tts

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/apresai/podcaster/internal/script"
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
	ID       string // Provider-specific voice identifier
	Name     string // Human-readable label
	Provider string // "elevenlabs", "gemini", "google"
}

// VoiceMap maps podcast hosts to voices.
type VoiceMap struct {
	Host1 Voice // Voice 1 (maps to Alex)
	Host2 Voice // Voice 2 (maps to Sam)
	Host3 Voice // Voice 3 (maps to Jordan)
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
	DefaultFor  string // "Voice 1", "Voice 2", "Voice 3", or ""
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

// ProviderConfig holds model and voice settings passed to provider constructors.
type ProviderConfig struct {
	Model     string  // provider-specific model ID (empty = default)
	Speed     float64 // speech speed (0 = provider default)
	Stability float64 // ElevenLabs voice stability 0-1 (0 = default 0.5)
	Pitch     float64 // Google Cloud pitch in semitones (0 = default)
}

// validModels maps provider names to their valid model IDs.
var validModels = map[string]map[string]bool{
	"elevenlabs": {
		"eleven_v3":              true,
		"eleven_multilingual_v2": true,
		"eleven_turbo_v2_5":     true,
		"eleven_flash_v2_5":     true,
	},
	"gemini": {
		"gemini-2.5-pro-preview-tts":   true,
		"gemini-2.5-flash-preview-tts": true,
	},
}

// ValidateModel checks that the given model ID is valid for the provider.
// Returns nil if model is empty (use default) or valid.
func ValidateModel(provider, model string) error {
	if model == "" {
		return nil
	}
	models, ok := validModels[provider]
	if !ok {
		// Provider has no model selection (e.g., google)
		return fmt.Errorf("provider %q does not support --tts-model", provider)
	}
	if !models[model] {
		var valid []string
		for m := range models {
			valid = append(valid, m)
		}
		return fmt.Errorf("invalid TTS model %q for provider %q: valid models are %s", model, provider, strings.Join(valid, ", "))
	}
	return nil
}

// NewProvider creates a TTS provider by name. voice1, voice2, and voice3 are
// optional provider-specific voice ID overrides for hosts 1-3.
func NewProvider(name string, voice1, voice2, voice3 string, cfg ProviderConfig) (Provider, error) {
	switch name {
	case "elevenlabs":
		return NewElevenLabsProvider(voice1, voice2, voice3, cfg), nil
	case "google":
		return NewGoogleProvider(voice1, voice2, voice3, cfg)
	case "gemini":
		return NewGeminiProvider(voice1, voice2, voice3, cfg), nil
	default:
		return nil, fmt.Errorf("unknown TTS provider %q: choose elevenlabs, google, or gemini", name)
	}
}

// ParseVoiceSpec parses "provider:voiceID" or plain "voiceID".
// Returns (provider, voiceID). If no prefix, provider is empty (caller uses default).
func ParseVoiceSpec(spec string) (provider, voiceID string) {
	if i := strings.Index(spec, ":"); i > 0 {
		prefix := spec[:i]
		// Only treat as provider prefix if it's a known provider name
		switch prefix {
		case "elevenlabs", "gemini", "google":
			return prefix, spec[i+1:]
		}
	}
	return "", spec
}

// ProviderSet is a lazy pool of TTS providers, created on first use.
type ProviderSet struct {
	mu        sync.Mutex
	providers map[string]Provider
	configs   map[string]ProviderConfig
}

// NewProviderSet creates an empty provider pool.
func NewProviderSet() *ProviderSet {
	return &ProviderSet{
		providers: make(map[string]Provider),
		configs:   make(map[string]ProviderConfig),
	}
}

// SetConfig stores a ProviderConfig for the named provider.
// Must be called before Get() for that provider.
func (ps *ProviderSet) SetConfig(name string, cfg ProviderConfig) {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	ps.configs[name] = cfg
}

// Get returns a provider by name, creating it on first call.
// Voice IDs are not passed here — they are routed per-segment via Voice.ID.
func (ps *ProviderSet) Get(name string) (Provider, error) {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	if p, ok := ps.providers[name]; ok {
		return p, nil
	}

	cfg := ps.configs[name] // zero value if not set
	p, err := NewProvider(name, "", "", "", cfg)
	if err != nil {
		return nil, err
	}
	ps.providers[name] = p
	return p, nil
}

// Close closes all providers in the set.
func (ps *ProviderSet) Close() error {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	var firstErr error
	for _, p := range ps.providers {
		if err := p.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	ps.providers = make(map[string]Provider)
	return firstErr
}
