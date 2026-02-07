# ElevenLabs API — AI Integration Guide

Self-contained reference for implementing ElevenLabs TTS in Go using `net/http`. No SDK — ElevenLabs has no official Go SDK.

## Authentication

All requests require the `xi-api-key` header.

```
xi-api-key: <your-api-key>
```

Read from environment variable `ELEVENLABS_API_KEY`.

## Text-to-Speech Endpoint

Convert text to speech for a given voice.

```
POST https://api.elevenlabs.io/v1/text-to-speech/{voice_id}?output_format=mp3_44100_128
```

### Request Headers

| Header | Value |
|--------|-------|
| `xi-api-key` | Your ElevenLabs API key |
| `Content-Type` | `application/json` |

### Request Body Schema

```json
{
  "text": "The text to convert to speech.",
  "model_id": "eleven_multilingual_v2",
  "voice_settings": {
    "stability": 0.5,
    "similarity_boost": 0.75,
    "style": 0.0,
    "use_speaker_boost": true,
    "speed": 1.0
  }
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `text` | string | Yes | The text to synthesize. Max ~5000 characters per request. |
| `model_id` | string | Yes | TTS model ID (see [Models](#models)) |
| `voice_settings` | object | No | Voice configuration (see [Voice Settings](#voice-settings)). If omitted, uses voice defaults. |

### Response

**200 OK** — Raw audio bytes in the requested format. Read the full response body as `[]byte`.

**Error responses** — JSON body with error detail (see [Error Handling](#error-handling)).

### Go Code Example

Based on the official ElevenLabs docs Go snippet:

```go
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
)

type TTSRequest struct {
	Text          string         `json:"text"`
	ModelID       string         `json:"model_id"`
	VoiceSettings *VoiceSettings `json:"voice_settings,omitempty"`
}

type VoiceSettings struct {
	Stability       float64 `json:"stability"`
	SimilarityBoost float64 `json:"similarity_boost"`
	Style           float64 `json:"style"`
	UseSpeakerBoost bool    `json:"use_speaker_boost"`
	Speed           float64 `json:"speed"`
}

func Synthesize(ctx context.Context, text string, voiceID string) ([]byte, error) {
	apiKey := os.Getenv("ELEVENLABS_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("ELEVENLABS_API_KEY not set")
	}

	reqBody := TTSRequest{
		Text:    text,
		ModelID: "eleven_multilingual_v2",
		VoiceSettings: &VoiceSettings{
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

	url := fmt.Sprintf(
		"https://api.elevenlabs.io/v1/text-to-speech/%s?output_format=mp3_44100_128",
		voiceID,
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("xi-api-key", apiKey)
	req.Header.Set("Content-Type", "application/json")

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		errBody, _ := io.ReadAll(res.Body)
		return nil, fmt.Errorf("elevenlabs API error (status %d): %s", res.StatusCode, string(errBody))
	}

	audio, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	return audio, nil
}
```

## Voice Settings

All fields are optional. If `voice_settings` is omitted from the request, the voice's default settings are used.

| Field | Type | Range | Default | Description |
|-------|------|-------|---------|-------------|
| `stability` | float | 0.0–1.0 | 0.5 | Controls voice consistency. Lower = more expressive/variable. Higher = more stable/monotone. |
| `similarity_boost` | float | 0.0–1.0 | 0.75 | Controls how closely the output matches the original voice. Higher = closer match but may amplify artifacts. |
| `style` | float | 0.0–1.0 | 0.0 | Controls style exaggeration. Higher values amplify the voice's speaking style. Increases latency. |
| `use_speaker_boost` | bool | — | true | Boosts similarity to the original speaker. Increases latency slightly. |
| `speed` | float | 0.5–2.0 | 1.0 | Controls speech speed. 0.5 = half speed, 2.0 = double speed. |

### Recommended Settings for Podcaster

| Host | stability | similarity_boost | style | use_speaker_boost | speed |
|------|-----------|-------------------|-------|-------------------|-------|
| Alex | 0.5 | 0.75 | 0.0 | true | 1.0 |
| Sam | 0.5 | 0.75 | 0.0 | true | 1.0 |

These produce natural, conversational speech without excessive variation or artifacts.

## Available Output Formats

The `output_format` query parameter controls the audio encoding.

### MP3

| Format | Sample Rate | Bitrate | Notes |
|--------|-------------|---------|-------|
| `mp3_22050_32` | 22.05 kHz | 32 kbps | Lowest quality, smallest files |
| `mp3_24000_48` | 24 kHz | 48 kbps | Low quality |
| `mp3_44100_32` | 44.1 kHz | 32 kbps | Low bitrate at high sample rate |
| `mp3_44100_64` | 44.1 kHz | 64 kbps | Medium quality |
| `mp3_44100_96` | 44.1 kHz | 96 kbps | Good quality |
| `mp3_44100_128` | 44.1 kHz | 128 kbps | **Recommended for this project** — standard podcast quality |
| `mp3_44100_192` | 44.1 kHz | 192 kbps | Highest MP3 quality (Creator tier+) |

### PCM (raw, uncompressed, S16LE)

| Format | Sample Rate | Notes |
|--------|-------------|-------|
| `pcm_8000` | 8 kHz | Telephony quality |
| `pcm_16000` | 16 kHz | Standard speech |
| `pcm_22050` | 22.05 kHz | |
| `pcm_24000` | 24 kHz | |
| `pcm_32000` | 32 kHz | |
| `pcm_44100` | 44.1 kHz | CD quality (Pro tier+) |
| `pcm_48000` | 48 kHz | Highest quality (Pro tier+) |

### Telephony

| Format | Sample Rate | Notes |
|--------|-------------|-------|
| `ulaw_8000` | 8 kHz | mu-law encoding |
| `alaw_8000` | 8 kHz | A-law encoding |

### Opus

| Format | Sample Rate | Bitrate | Notes |
|--------|-------------|---------|-------|
| `opus_48000_32` | 48 kHz | 32 kbps | |
| `opus_48000_64` | 48 kHz | 64 kbps | |
| `opus_48000_96` | 48 kHz | 96 kbps | |
| `opus_48000_128` | 48 kHz | 128 kbps | |
| `opus_48000_192` | 48 kHz | 192 kbps | |

## Models

| Model ID | Description | Latency | Credit Cost | Best For |
|----------|-------------|---------|-------------|----------|
| `eleven_multilingual_v2` | Highest quality, 29 languages, emotionally aware | Higher | 1x (1 credit/char) | Best voice quality and naturalness |
| `eleven_turbo_v2_5` | High quality, 32 languages, 3x faster | ~400ms | 0.5x (1 credit/2 chars) | Balance of quality and speed |
| `eleven_flash_v2_5` | Low latency, 32 languages | ~75ms | 0.5x (1 credit/2 chars) | **Current default** — cost-sensitive, good quality |
| `eleven_flash_v2` | Low latency, English only | ~75ms | 0.5x (1 credit/2 chars) | English-only, lowest latency |

This project uses `eleven_flash_v2_5` — half the credit cost with good quality for pre-generated podcast audio.

## Pricing & Cost Estimates

### ElevenLabs Plans

| Plan | Monthly Price | Credits/month | Est. Audio (flash) | Est. Audio (multilingual v2) |
|------|-------------|---------------|---------------------|-------------------------------|
| Free | $0 | 10,000 | ~20 min | ~10 min |
| Starter | $5 | 30,000 | ~60 min | ~30 min |
| Creator | $22 | 100,000 | ~200 min | ~100 min |
| Pro | $99 | 500,000 | ~1,000 min | ~500 min |
| Scale | $330 | 2,000,000 | ~4,000 min | ~2,000 min |

### Cost Per Episode by Model

Based on ~1,000 characters per minute of audio output:

| Model | Credits/min | Cost/min (Creator) | 5-min Episode | 10-min Episode | 20-min Episode |
|-------|-------------|-------------------|---------------|----------------|----------------|
| `eleven_flash_v2_5` | ~500 | ~$0.11 | ~$0.55 | ~$1.10 | ~$2.20 |
| `eleven_flash_v2` | ~500 | ~$0.11 | ~$0.55 | ~$1.10 | ~$2.20 |
| `eleven_turbo_v2_5` | ~500 | ~$0.11 | ~$0.55 | ~$1.10 | ~$2.20 |
| `eleven_multilingual_v2` | ~1,000 | ~$0.22 | ~$1.10 | ~$2.20 | ~$4.40 |

### Episodes Per Month (Creator Plan, $22/mo)

| Model | 5-min Episodes | 10-min Episodes | 20-min Episodes |
|-------|---------------|-----------------|-----------------|
| Flash / Turbo (0.5x) | ~40 | ~20 | ~10 |
| Multilingual v2 (1x) | ~20 | ~10 | ~5 |

### Total Generation Cost (Claude API + TTS)

Claude API cost for script generation is ~$0.01-0.05 per episode (negligible).
Total cost is dominated by TTS.

| Episode Length | Flash Model | Multilingual v2 |
|---------------|-------------|-----------------|
| Short (~5 min) | **~$0.55** | ~$1.10 |
| Medium (~10 min) | **~$1.10** | ~$2.20 |
| Long (~20 min) | **~$2.20** | ~$4.40 |

### TTS Provider Comparison (Pay-as-you-go)

All costs based on ~1,000 characters per minute of podcast audio.

| Provider | Model | $/1M chars | $/min audio | 10-min Episode | Quality |
|----------|-------|-----------|-------------|----------------|---------|
| Google Cloud | Standard | $4.00 | $0.004 | **$0.04** | Basic, robotic |
| OpenAI | TTS | $3.75 | $0.004 | **$0.04** | Good |
| Amazon Polly | Standard | $4.00 | $0.004 | **$0.04** | Basic |
| OpenAI | TTS HD | $7.50 | $0.008 | **$0.08** | Very good |
| Deepgram | Aura-1 | $15.00 | $0.015 | **$0.15** | Good |
| Google Cloud | WaveNet/Neural2 | $16.00 | $0.016 | **$0.16** | Good |
| Amazon Polly | Neural | $16.00 | $0.016 | **$0.16** | Good |
| Google Cloud | Chirp 3: HD | $30.00 | $0.030 | **$0.30** | Very good |
| Amazon Polly | Generative | $30.00 | $0.030 | **$0.30** | Very good |
| Deepgram | Aura-2 | $30.00 | $0.030 | **$0.30** | Very good |
| ElevenLabs | Flash v2.5 | $49.50 | $0.050 | **$0.50** | Excellent |
| ElevenLabs | Turbo v2.5 | $49.50 | $0.050 | **$0.50** | Excellent |
| Google Cloud | Instant Custom | $60.00 | $0.060 | **$0.60** | Good (custom) |
| ElevenLabs | Multilingual v2 | $99.00 | $0.099 | **$0.99** | Best |
| Google Cloud | Studio | $160.00 | $0.160 | **$1.60** | Premium |

**Note on Gemini TTS**: Gemini 2.5 Flash TTS is priced at $0.50/1M input tokens + $10.00/1M output tokens. Pricing is token-based (not character-based) and depends on audio output length, making direct comparison difficult. Estimated cost is competitive with Google Cloud Chirp for similar quality.

#### Key Takeaways

- **Cheapest**: Google Cloud Standard / OpenAI TTS / Amazon Polly Standard (~$0.04/10-min episode)
- **Best value**: OpenAI TTS HD ($0.08/episode) or Google Cloud WaveNet ($0.16/episode)
- **Current (ElevenLabs Flash)**: $0.50/episode pay-as-you-go, ~$1.10/episode on Creator plan
- **Best quality**: ElevenLabs Multilingual v2 ($0.99/episode) — most natural and expressive

Google Cloud and OpenAI are 5-10x cheaper than ElevenLabs at pay-as-you-go rates. ElevenLabs subscription plans narrow the gap but remain more expensive. The tradeoff is voice quality — ElevenLabs voices are generally considered the most natural-sounding.

#### Free Tiers

| Provider | Free Allowance | Est. Free Audio |
|----------|---------------|-----------------|
| Google Cloud Standard | 4M chars/month | ~4,000 min |
| Google Cloud WaveNet/Neural2 | 1M chars/month | ~1,000 min |
| Google Cloud Chirp 3: HD | 1M chars/month | ~1,000 min |
| ElevenLabs | 10,000 chars/month | ~10-20 min |
| Amazon Polly | 1M chars/month (12 mo) | ~1,000 min |
| OpenAI | None | — |

Google Cloud's free tier is by far the most generous — 1M characters/month would cover ~100 ten-minute episodes on WaveNet/Chirp.

## List Voices Endpoint

Discover available voices and their IDs.

```
GET https://api.elevenlabs.io/v1/voices
```

### Request Headers

| Header | Value |
|--------|-------|
| `xi-api-key` | Your ElevenLabs API key |

### Response Structure

```json
{
  "voices": [
    {
      "voice_id": "JBFqnCBsd6RMkjVDRZzb",
      "name": "George",
      "labels": {
        "gender": "male",
        "accent": "British",
        "age": "middle-aged",
        "use_case": "narration"
      },
      "preview_url": "https://...",
      "category": "premade"
    }
  ]
}
```

### Key Response Fields

| Field | Type | Description |
|-------|------|-------------|
| `voices[].voice_id` | string | The ID to use in TTS requests |
| `voices[].name` | string | Human-readable voice name |
| `voices[].labels` | object | Tags: `gender`, `accent`, `age`, `use_case` |
| `voices[].preview_url` | string | URL to a sample audio clip |
| `voices[].category` | string | `premade`, `cloned`, `generated` |

### Go Code Example

```go
func ListVoices(ctx context.Context) ([]Voice, error) {
	apiKey := os.Getenv("ELEVENLABS_API_KEY")

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.elevenlabs.io/v1/voices", nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("xi-api-key", apiKey)

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		errBody, _ := io.ReadAll(res.Body)
		return nil, fmt.Errorf("list voices error (status %d): %s", res.StatusCode, string(errBody))
	}

	var result struct {
		Voices []Voice `json:"voices"`
	}
	if err := json.NewDecoder(res.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return result.Voices, nil
}

type Voice struct {
	VoiceID    string            `json:"voice_id"`
	Name       string            `json:"name"`
	Labels     map[string]string `json:"labels"`
	PreviewURL string            `json:"preview_url"`
	Category   string            `json:"category"`
}
```

## Voice Settings Endpoint

Get the default settings for a specific voice.

```
GET https://api.elevenlabs.io/v1/voices/{voice_id}/settings
```

### Request Headers

| Header | Value |
|--------|-------|
| `xi-api-key` | Your ElevenLabs API key |

### Response

```json
{
  "stability": 0.5,
  "similarity_boost": 0.75,
  "style": 0.0,
  "use_speaker_boost": true
}
```

Useful for discovering a voice's recommended defaults before overriding them.

## Streaming Endpoint (Reference Only)

Not used in v1 — included for future reference.

```
POST https://api.elevenlabs.io/v1/text-to-speech/{voice_id}/stream?output_format=mp3_44100_128
```

Same request body as the non-streaming endpoint. Response is a chunked audio stream instead of a complete file. Use this for real-time playback or low-latency applications in a future version.

## Error Handling

Error responses return JSON with a detail message.

### Error Response Format

```json
{
  "detail": {
    "status": "error_type",
    "message": "Human-readable error description"
  }
}
```

### Status Codes

| Code | Meaning | Action |
|------|---------|--------|
| 200 | Success | Read response body as audio bytes |
| 401 | Invalid or missing API key | Check `ELEVENLABS_API_KEY` value |
| 422 | Validation error (bad parameters) | Check request body fields, voice ID, output format |
| 429 | Rate limited | Retry with exponential backoff |
| 500 | Server error | Retry with exponential backoff |
| 502 | Bad gateway | Retry with exponential backoff |
| 503 | Service unavailable | Retry with exponential backoff |

### Go Error Response Struct

```go
type ElevenLabsError struct {
	Detail struct {
		Status  string `json:"status"`
		Message string `json:"message"`
	} `json:"detail"`
}
```

## Rate Limits and Best Practices

### Sequential Processing

Process TTS segments one at a time — do not fire concurrent requests. This:
- Stays well within rate limits
- Produces predictable progress output
- Avoids 429 errors that would slow things down more than sequential processing

### Retry Strategy

| Parameter | Value |
|-----------|-------|
| Max attempts | 3 |
| Initial backoff | 1 second |
| Backoff multiplier | 2x |
| Max backoff | 10 seconds |
| Retryable status codes | 429, 500, 502, 503 |

### Text Length

Keep each request under **5000 characters**. If a script segment exceeds this, split it at sentence boundaries before sending.

### Cost Awareness

ElevenLabs charges per character. A 10-minute episode at ~150 words/minute is ~1500 words or ~9000 characters. Monitor usage against your plan's character quota.

## Go Implementation Patterns

### Complete Struct Definitions

```go
// TTSRequest is the request body for POST /v1/text-to-speech/{voice_id}
type TTSRequest struct {
	Text          string         `json:"text"`
	ModelID       string         `json:"model_id"`
	VoiceSettings *VoiceSettings `json:"voice_settings,omitempty"`
}

// VoiceSettings controls voice characteristics in a TTS request.
type VoiceSettings struct {
	Stability       float64 `json:"stability"`
	SimilarityBoost float64 `json:"similarity_boost"`
	Style           float64 `json:"style"`
	UseSpeakerBoost bool    `json:"use_speaker_boost"`
	Speed           float64 `json:"speed"`
}

// Voice represents a voice from GET /v1/voices.
type Voice struct {
	VoiceID    string            `json:"voice_id"`
	Name       string            `json:"name"`
	Labels     map[string]string `json:"labels"`
	PreviewURL string            `json:"preview_url"`
	Category   string            `json:"category"`
}

// VoiceDefaults represents the response from GET /v1/voices/{voice_id}/settings.
type VoiceDefaults struct {
	Stability       float64 `json:"stability"`
	SimilarityBoost float64 `json:"similarity_boost"`
	Style           float64 `json:"style"`
	UseSpeakerBoost bool    `json:"use_speaker_boost"`
}

// ElevenLabsError represents an error response from the API.
type ElevenLabsError struct {
	Detail struct {
		Status  string `json:"status"`
		Message string `json:"message"`
	} `json:"detail"`
}
```

### Helper Function Signature

```go
// Synthesize converts text to speech using the ElevenLabs API.
// Returns raw MP3 audio bytes.
func Synthesize(ctx context.Context, text string, voiceID string) ([]byte, error)
```

### Full HTTP Request Construction Pattern

```go
func (c *Client) Synthesize(ctx context.Context, text string, voiceID string) ([]byte, error) {
	reqBody := TTSRequest{
		Text:    text,
		ModelID: c.modelID, // "eleven_multilingual_v2"
		VoiceSettings: &VoiceSettings{
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

	url := fmt.Sprintf(
		"https://api.elevenlabs.io/v1/text-to-speech/%s?output_format=%s",
		voiceID,
		c.outputFormat, // "mp3_44100_128"
	)

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

	// Retryable errors — caller should implement retry with backoff
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
		return nil, fmt.Errorf("elevenlabs API error (status %d): %s", res.StatusCode, string(errBody))
	}

	return io.ReadAll(res.Body)
}
```
