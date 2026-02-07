# Gemini TTS API — AI Integration Guide

Self-contained reference for implementing Gemini TTS in Go using `net/http`. No SDK — uses the Gemini `generateContent` endpoint with audio response modality.

## Authentication

All requests use an API key as a query parameter.

```
POST https://generativelanguage.googleapis.com/v1beta/models/{model-id}:generateContent?key=YOUR_API_KEY
```

Read from environment variable `GEMINI_API_KEY`.

## Single-Speaker TTS

Generate audio from text using a single voice.

### Request Body Schema

```json
{
  "contents": [
    {
      "parts": [
        { "text": "The text to convert to speech." }
      ]
    }
  ],
  "generationConfig": {
    "responseModalities": ["AUDIO"],
    "speechConfig": {
      "voiceConfig": {
        "prebuiltVoiceConfig": {
          "voiceName": "Charon"
        }
      }
    }
  }
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `contents[].parts[].text` | string | Yes | Text to synthesize |
| `generationConfig.responseModalities` | []string | Yes | Must be `["AUDIO"]` |
| `speechConfig.voiceConfig.prebuiltVoiceConfig.voiceName` | string | Yes | Voice name (see [Voices](#available-voices)) |

## Multi-Speaker TTS

Generate a multi-speaker dialogue as a single audio stream. This is the mode used by Podcaster for batch synthesis.

### Request Body Schema

```json
{
  "contents": [
    {
      "parts": [
        { "text": "Alex: Welcome to the show!\nSam: Thanks Alex, great to be here." }
      ]
    }
  ],
  "generationConfig": {
    "responseModalities": ["AUDIO"],
    "speechConfig": {
      "multiSpeakerVoiceConfig": {
        "speakerVoiceConfigs": [
          {
            "speaker": "Alex",
            "voiceConfig": {
              "prebuiltVoiceConfig": { "voiceName": "Charon" }
            }
          },
          {
            "speaker": "Sam",
            "voiceConfig": {
              "prebuiltVoiceConfig": { "voiceName": "Leda" }
            }
          }
        ]
      }
    }
  }
}
```

**Notes:**
- Maximum 2 speakers per request
- Speaker names in the text (`"Alex: ..."`) must match `speaker` fields in config
- Text format: `"Speaker: text\n"` for each line of dialogue

## Response Format

```json
{
  "candidates": [
    {
      "content": {
        "parts": [
          {
            "inlineData": {
              "mimeType": "audio/pcm",
              "data": "BASE64_ENCODED_PCM_DATA"
            }
          }
        ]
      },
      "finishReason": "STOP"
    }
  ]
}
```

Audio path: `candidates[0].content.parts[0].inlineData.data`

## Audio Output Format

| Property | Value |
|----------|-------|
| Encoding | Linear PCM (signed 16-bit little-endian, s16le) |
| Sample rate | 24,000 Hz |
| Bit depth | 16-bit |
| Channels | Mono (1) |
| Delivery | Base64-encoded in JSON response |

**Converting to MP3 (used by Podcaster):**
```bash
ffmpeg -f s16le -ar 24000 -ac 1 -i input.raw \
  -af aresample=resampler=soxr \
  -c:a libmp3lame -b:a 192k -q:a 0 \
  -ar 44100 -ac 2 -y output.mp3
```

## Go Struct Definitions

```go
// Request types
type GeminiRequest struct {
    Contents         []GeminiContent  `json:"contents"`
    GenerationConfig GeminiGenConfig  `json:"generationConfig"`
}

type GeminiContent struct {
    Parts []GeminiPart `json:"parts"`
}

type GeminiPart struct {
    Text string `json:"text,omitempty"`
}

type GeminiGenConfig struct {
    ResponseModalities []string           `json:"responseModalities"`
    SpeechConfig       GeminiSpeechConfig `json:"speechConfig"`
}

type GeminiSpeechConfig struct {
    VoiceConfig             *GeminiVoiceConfig        `json:"voiceConfig,omitempty"`
    MultiSpeakerVoiceConfig *GeminiMultiSpeakerConfig `json:"multiSpeakerVoiceConfig,omitempty"`
}

type GeminiVoiceConfig struct {
    PrebuiltVoiceConfig GeminiPrebuiltVoice `json:"prebuiltVoiceConfig"`
}

type GeminiMultiSpeakerConfig struct {
    SpeakerVoiceConfigs []GeminiSpeakerVoiceConfig `json:"speakerVoiceConfigs"`
}

type GeminiSpeakerVoiceConfig struct {
    Speaker     string           `json:"speaker"`
    VoiceConfig GeminiVoiceConfig `json:"voiceConfig"`
}

type GeminiPrebuiltVoice struct {
    VoiceName string `json:"voiceName"`
}

// Response types
type GeminiResponse struct {
    Candidates []GeminiCandidate `json:"candidates"`
}

type GeminiCandidate struct {
    Content GeminiRespContent `json:"content"`
}

type GeminiRespContent struct {
    Parts []GeminiRespPart `json:"parts"`
}

type GeminiRespPart struct {
    InlineData *GeminiInlineData `json:"inlineData,omitempty"`
}

type GeminiInlineData struct {
    MimeType string `json:"mimeType"`
    Data     string `json:"data"` // base64-encoded PCM
}
```

## Available Voices

30 voices available. Voice names are case-sensitive.

| Voice | Gender | Style |
|-------|--------|-------|
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

## Models

| Model ID | Description | Best For |
|----------|-------------|----------|
| `gemini-2.5-flash-preview-tts` | Low latency, single + multi-speaker | **Current default** — cost-effective batch synthesis |
| `gemini-2.5-pro-preview-tts` | Higher quality, easier to steer | Premium quality when budget allows |

This project uses `gemini-2.5-flash-preview-tts` for TTS output.

## Limits

| Constraint | Value |
|------------|-------|
| Context window | 32,000 tokens |
| Max output | 16,000 tokens (~500 seconds of audio) |
| Audio token rate | 32 tokens per second |
| Max audio duration | ~8.3 minutes per request |
| Max text input | ~4,000 bytes |
| Multi-speaker limit | 2 speakers per request |

## Pricing

### Gemini 2.5 Flash TTS

| Tier | Input (per 1M tokens) | Output (per 1M tokens) |
|------|----------------------|----------------------|
| Free | Free | Free |
| Standard | $0.50 | $10.00 |
| Batch (50% discount) | $0.25 | $5.00 |

### Gemini 2.5 Pro TTS

| Tier | Input (per 1M tokens) | Output (per 1M tokens) |
|------|----------------------|----------------------|
| Standard | $1.00 | $20.00 |
| Batch (50% discount) | $0.50 | $10.00 |

### Cost Estimates

Audio output: 32 tokens/second = 1,920 tokens/minute.

| Episode Length | Flash (Standard) | Flash (Batch) | Pro (Standard) |
|---------------|-----------------|---------------|----------------|
| Short (~5 min) | ~$0.10 | ~$0.05 | ~$0.19 |
| Medium (~10 min) | ~$0.19 | ~$0.10 | ~$0.38 |
| Long (~20 min) | ~$0.38 | ~$0.19 | ~$0.77 |

Flash TTS is competitive with Google Cloud Chirp 3 HD ($0.30/10-min episode) and significantly cheaper than ElevenLabs Flash ($0.50/10-min episode).

### Provider Comparison

| Provider | Model | 10-min Episode | Quality |
|----------|-------|----------------|---------|
| Gemini Flash TTS | gemini-2.5-flash-preview-tts | **~$0.19** | Very good |
| Google Cloud | Chirp 3: HD | ~$0.30 | Very good |
| ElevenLabs | Flash v2.5 | ~$0.50 | Excellent |
| ElevenLabs | Multilingual v2 | ~$0.99 | Best |

## Error Handling

### Error Response Format

```json
{
  "error": {
    "code": 429,
    "message": "Resource has been exhausted (e.g. check quota).",
    "status": "RESOURCE_EXHAUSTED"
  }
}
```

### Status Codes

| Code | Status | Action |
|------|--------|--------|
| 200 | Success | Decode base64 audio from response |
| 400 | INVALID_ARGUMENT | Check request body, voice name, text length |
| 401 | UNAUTHENTICATED | Check `GEMINI_API_KEY` value |
| 403 | PERMISSION_DENIED | API not enabled or insufficient permissions |
| 429 | RESOURCE_EXHAUSTED | Retry with exponential backoff |
| 500 | INTERNAL | Retry with exponential backoff |
| 503 | UNAVAILABLE | Retry with exponential backoff |

### Go Error Response Struct

```go
type GeminiError struct {
    Error struct {
        Code    int    `json:"code"`
        Message string `json:"message"`
        Status  string `json:"status"`
    } `json:"error"`
}
```

## Retry Strategy

| Parameter | Value |
|-----------|-------|
| Max attempts | 3 |
| Initial backoff | 1 second |
| Backoff multiplier | 2x |
| Max backoff | 10 seconds |
| Retryable status codes | 429, 500, 503 |

## Rate Limits

| Model | Queries/minute |
|-------|---------------|
| gemini-2.5-flash-tts | 150 QPM |
| gemini-2.5-pro-tts | 125 QPM |

Limits are defaults and may vary by tier. Check Google AI Studio for active limits.
