# Google Cloud Text-to-Speech API — AI Integration Guide

Self-contained reference for implementing Google Cloud TTS in Go using the official SDK (`cloud.google.com/go/texttospeech`). Focused on Chirp 3: HD voices.

## Authentication

Uses Application Default Credentials (ADC). No API key needed — authenticate via `gcloud`.

```bash
# One-time setup
gcloud auth application-default login
```

The Go SDK automatically finds credentials from:
1. `GOOGLE_APPLICATION_CREDENTIALS` environment variable (path to service account JSON)
2. User credentials at `~/.config/gcloud/application_default_credentials.json`
3. Compute Engine/Cloud Run metadata server

## Go SDK Setup

```bash
go get cloud.google.com/go/texttospeech
```

Package import:
```go
import (
    texttospeech "cloud.google.com/go/texttospeech/apiv1"
    texttospeechpb "cloud.google.com/go/texttospeech/apiv1/texttospeechpb"
)
```

## Synthesize Speech

### Go Code Example

```go
func Synthesize(ctx context.Context, text string, voiceName string) ([]byte, error) {
    client, err := texttospeech.NewClient(ctx)
    if err != nil {
        return nil, fmt.Errorf("create TTS client: %w", err)
    }
    defer client.Close()

    resp, err := client.SynthesizeSpeech(ctx, &texttospeechpb.SynthesizeSpeechRequest{
        Input: &texttospeechpb.SynthesisInput{
            InputSource: &texttospeechpb.SynthesisInput_Text{
                Text: text,
            },
        },
        Voice: &texttospeechpb.VoiceSelectionParams{
            LanguageCode: "en-US",
            Name:         voiceName, // e.g., "en-US-Chirp3-HD-Charon"
        },
        AudioConfig: &texttospeechpb.AudioConfig{
            AudioEncoding: texttospeechpb.AudioEncoding_MP3,
        },
    })
    if err != nil {
        return nil, fmt.Errorf("synthesize speech: %w", err)
    }

    return resp.AudioContent, nil
}
```

### Request Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `Input.Text` | string | Yes | Text to synthesize (max 5,000 bytes) |
| `Voice.LanguageCode` | string | Yes | BCP-47 code (e.g., `"en-US"`) |
| `Voice.Name` | string | Yes | Full voice name |
| `AudioConfig.AudioEncoding` | enum | Yes | Output format |

### Response

The `SynthesizeSpeechResponse` contains:
- `AudioContent`: `[]byte` — raw audio in the requested format (MP3 by default)

## Audio Output Formats

| Encoding | Constant | Description |
|----------|----------|-------------|
| MP3 | `AudioEncoding_MP3` | MP3 (32kbps default) |
| LINEAR16 | `AudioEncoding_LINEAR16` | 16-bit PCM with WAV header |
| OGG_OPUS | `AudioEncoding_OGG_OPUS` | Opus in OGG container |
| MULAW | `AudioEncoding_MULAW` | 8-bit G.711 mu-law |
| ALAW | `AudioEncoding_ALAW` | 8-bit G.711 A-law |

This project uses `AudioEncoding_MP3` — no FFmpeg conversion needed.

## Available Chirp 3: HD Voices (en-US)

30 voices available. Voice names follow the pattern `en-US-Chirp3-HD-{Name}`.

| Voice Name | Gender | Style |
|------------|--------|-------|
| Achernar | Female | Soft |
| Achird | Male | Friendly |
| Algenib | Male | Gravelly |
| Algieba | Male | Smooth |
| Alnilam | Male | Firm |
| Aoede | Female | Breezy |
| Autonoe | Female | Bright |
| Callirrhoe | Female | Easy-going |
| **Charon** | Male | Informative (default Alex) |
| Despina | Female | Smooth |
| Enceladus | Male | Breathy |
| Erinome | Female | Clear |
| Fenrir | Male | Excitable |
| Gacrux | Female | Mature |
| Iapetus | Male | Clear |
| Kore | Female | Firm |
| Laomedeia | Female | Upbeat |
| **Leda** | Female | Youthful (default Sam) |
| Orus | Male | Firm |
| Puck | Male | Upbeat |
| Pulcherrima | Female | Forward |
| Rasalgethi | Male | Informative |
| Sadachbia | Male | Lively |
| Sadaltager | Male | Knowledgeable |
| Schedar | Male | Even |
| Sulafat | Female | Warm |
| Umbriel | Male | Easy-going |
| Vindemiatrix | Female | Gentle |
| Zephyr | Female | Bright |
| Zubenelgenubi | Male | Casual |

Also available in: en-GB, en-AU, en-IN, plus 50+ additional languages.

## AudioConfig Options

| Field | Type | Range | Default | Description |
|-------|------|-------|---------|-------------|
| `AudioEncoding` | enum | — | LINEAR16 | Output format |
| `SpeakingRate` | float64 | 0.25–2.0 | 1.0 | Speech speed |
| `Pitch` | float64 | -20.0–20.0 | 0.0 | Pitch shift in semitones |
| `VolumeGainDb` | float64 | -96.0–16.0 | 0.0 | Volume adjustment in dB |
| `SampleRateHertz` | int32 | — | auto | Sample rate override |

## Pricing

### Per-Character Rates

| Voice Type | Cost per 1M chars | Free Tier |
|------------|-------------------|-----------|
| Standard | $4.00 | 4M chars/month |
| WaveNet / Neural2 | $16.00 | 1M chars/month |
| **Chirp 3: HD** | **$30.00** | **1M chars/month** |
| Studio | $160.00 | 100K chars/month |

New accounts also get $300 in free Google Cloud credits.

### Cost Per Episode

Based on ~1,000 characters per minute of audio:

| Episode Length | Chirp 3: HD | Free Tier Covers |
|---------------|-------------|-----------------|
| Short (~5 min) | ~$0.15 | ~200 episodes/month |
| Medium (~10 min) | ~$0.30 | ~100 episodes/month |
| Long (~20 min) | ~$0.60 | ~50 episodes/month |

### Provider Comparison

| Provider | Model | 10-min Episode | Quality | Free Tier |
|----------|-------|----------------|---------|-----------|
| Google Cloud | Chirp 3: HD | **~$0.30** | Very good | 1M chars/month |
| Gemini | Flash TTS | ~$0.19 | Very good | Free tier available |
| ElevenLabs | Flash v2.5 | ~$0.50 | Excellent | 10K chars/month |
| ElevenLabs | Multilingual v2 | ~$0.99 | Best | 10K chars/month |

Google Cloud's free tier (1M chars/month) is far more generous than ElevenLabs (10K chars/month), covering ~100 ten-minute episodes per month at no cost.

## Rate Limits

| Voice Type | Requests/minute |
|------------|----------------|
| Chirp 3 | 200 RPM |
| Neural2 | 1,000 RPM |
| Standard | 1,000 RPM |
| Studio | 500 RPM |

Content limit: 5,000 bytes per request. Multi-byte characters count toward this limit.

## Error Handling

The Go SDK returns gRPC status errors. Common codes:

| gRPC Code | HTTP Equiv | Meaning | Action |
|-----------|-----------|---------|--------|
| OK | 200 | Success | Read `AudioContent` |
| INVALID_ARGUMENT | 400 | Bad params | Check voice name, language code, text length |
| UNAUTHENTICATED | 401 | No credentials | Run `gcloud auth application-default login` |
| PERMISSION_DENIED | 403 | API not enabled | Enable Text-to-Speech API in Cloud Console |
| RESOURCE_EXHAUSTED | 429 | Rate limited | Retry with exponential backoff |
| INTERNAL | 500 | Server error | Retry with exponential backoff |
| UNAVAILABLE | 503 | Service down | Retry with exponential backoff |

### Go Error Handling Pattern

```go
import "google.golang.org/grpc/status"

resp, err := client.SynthesizeSpeech(ctx, req)
if err != nil {
    if st, ok := status.FromError(err); ok {
        switch st.Code() {
        case codes.ResourceExhausted, codes.Internal, codes.Unavailable:
            // Retryable
        default:
            // Non-retryable
        }
    }
    return nil, err
}
```

## Retry Strategy

| Parameter | Value |
|-----------|-------|
| Max attempts | 3 |
| Initial backoff | 1 second |
| Backoff multiplier | 2x |
| Max backoff | 10 seconds |
| Retryable codes | RESOURCE_EXHAUSTED, INTERNAL, UNAVAILABLE |

The Go SDK has built-in retry configuration via `CallOptions`. The Podcaster project wraps this with its own `WithRetry` helper for consistent behavior across all providers.

## Complete Go Struct Reference

```go
// SynthesizeSpeechRequest — the full request structure
type SynthesizeSpeechRequest struct {
    Input       *SynthesisInput       // Required: text or SSML
    Voice       *VoiceSelectionParams // Required: voice config
    AudioConfig *AudioConfig          // Required: output format
}

// SynthesisInput — what to synthesize
type SynthesisInput struct {
    // Use one of:
    Text string // Plain text
    Ssml string // SSML markup
}

// VoiceSelectionParams — which voice to use
type VoiceSelectionParams struct {
    LanguageCode string         // BCP-47 code, e.g. "en-US"
    Name         string         // Voice name, e.g. "en-US-Chirp3-HD-Charon"
    SsmlGender   SsmlVoiceGender // MALE, FEMALE, NEUTRAL (optional)
}

// AudioConfig — output format configuration
type AudioConfig struct {
    AudioEncoding   AudioEncoding // MP3, LINEAR16, OGG_OPUS, etc.
    SpeakingRate    float64       // 0.25–2.0
    Pitch           float64       // -20.0–20.0 semitones
    VolumeGainDb    float64       // -96.0–16.0 dB
    SampleRateHertz int32         // Override sample rate
}
```
