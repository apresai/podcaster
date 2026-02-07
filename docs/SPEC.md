# Podcaster — Technical Specification

## Overview

Podcaster is a Go CLI that converts written content into two-host podcast-style audio conversations. It implements a four-stage pipeline: content ingestion, script generation, text-to-speech synthesis, and audio assembly.

## System Architecture

```
┌─────────────────────────────────────────────────────────┐
│                     CLI (cobra)                         │
│              podcaster generate -i <src> -o <out>       │
└────────────────────────┬────────────────────────────────┘
                         │
                         ▼
┌─────────────────────────────────────────────────────────┐
│                   Pipeline Orchestrator                  │
│            Coordinates stages, handles errors            │
└────┬──────────┬──────────────┬──────────────┬───────────┘
     │          │              │              │
     ▼          ▼              ▼              ▼
┌─────────┐ ┌──────────┐ ┌──────────┐ ┌───────────┐
│ Ingest  │ │  Script  │ │   TTS    │ │ Assembly  │
│         │ │   Gen    │ │          │ │           │
│ URL     │ │ Claude   │ │ Eleven   │ │ FFmpeg    │
│ PDF     │ │ API      │ │ Labs API │ │ concat    │
│ Text    │ │          │ │          │ │           │
└─────────┘ └──────────┘ └──────────┘ └───────────┘
```

### Pipeline Flow

```
Input (URL/PDF/Text)
    │
    ▼
[1. Ingest] ─── Extract readable text from source
    │
    ▼
Content string (plain text)
    │
    ▼
[2. Script Gen] ─── Claude API generates two-host dialogue
    │
    ▼
Script ([]Segment JSON)
    │
    ▼
[3. TTS] ─── ElevenLabs converts each segment to audio
    │
    ▼
[]Audio files (temp MP3s)
    │
    ▼
[4. Assembly] ─── FFmpeg concatenates with silence gaps
    │
    ▼
Output MP3 file
```

## Component Design

### 1. Content Ingestion (`internal/ingest/`)

Responsible for extracting plain text from various source types.

#### Interface

```go
type Ingester interface {
    Ingest(ctx context.Context, source string) (*Content, error)
}

type Content struct {
    Text     string
    Title    string
    Source   string
    WordCount int
}
```

#### Implementations

| Type | Package | Detection | Notes |
|------|---------|-----------|-------|
| URL | `go-shiori/go-readability` | Starts with `http://` or `https://` | Extracts article content, strips nav/ads/scripts |
| PDF | `ledongthuc/pdf` | Ends with `.pdf` or file magic bytes | Page-by-page text extraction |
| Text | stdlib `os.ReadFile` | Default fallback | Reads raw file contents |

#### Source Detection

```go
func DetectSource(input string) SourceType {
    if strings.HasPrefix(input, "http://") || strings.HasPrefix(input, "https://") {
        return SourceURL
    }
    if strings.HasSuffix(strings.ToLower(input), ".pdf") {
        return SourcePDF
    }
    return SourceText
}
```

### 2. Script Generation (`internal/script/`)

Converts extracted content into a structured two-host conversation using the Claude API.

#### Data Models

```go
type Script struct {
    Title    string    `json:"title"`
    Summary  string    `json:"summary"`
    Segments []Segment `json:"segments"`
}

type Segment struct {
    Speaker string `json:"speaker"` // "Alex" or "Sam"
    Text    string `json:"text"`
}

type GenerateOptions struct {
    Topic    string // Focus area (optional)
    Tone     string // "casual", "technical", "educational"
    Duration string // Target duration: "short" (~5min), "medium" (~10min), "long" (~20min)
}
```

#### Claude API Integration

- **SDK**: `anthropic-sdk-go` (official Go SDK)
- **Model**: `claude-sonnet-4-5-20250929` (balance of quality and cost)
- **Max tokens**: 8192 (sufficient for ~20 min episode scripts)
- **Temperature**: 0.7 (creative but coherent)

#### Prompt Strategy

The prompt uses three key techniques:

**1. Persona Definitions**

```
Alex (Host): Drives the conversation. Introduces topics, provides context,
makes connections between ideas. Speaks with enthusiasm and clarity.
Uses analogies to explain complex concepts.

Sam (Analyst): Asks probing questions. Challenges assumptions, adds depth,
plays devil's advocate when appropriate. More measured and analytical tone.
Brings up counterpoints and edge cases.
```

**2. Scratchpad Reasoning**

The prompt instructs Claude to use an internal scratchpad (within `<scratchpad>` tags) to:
- Identify the 3–5 key themes in the source material
- Plan the conversation arc (intro → exploration → key insights → conclusion)
- Note which points deserve deeper discussion vs. brief mentions
- Estimate segment count for the target duration

The scratchpad content is stripped from the final output.

**3. Structured Output**

The prompt requests JSON output matching the `Script` struct. Example:

```json
{
  "title": "The Future of Battery Technology",
  "summary": "Alex and Sam discuss breakthroughs in solid-state batteries...",
  "segments": [
    {"speaker": "Alex", "text": "Welcome back! Today we're diving into..."},
    {"speaker": "Sam", "text": "This is fascinating because..."},
    ...
  ]
}
```

#### Duration Mapping

| Flag Value | Target Minutes | Approx Segments | Approx Word Count |
|-----------|---------------|-----------------|-------------------|
| `short` | ~5 | 15–25 | 750–1250 |
| `medium` | ~10 | 30–50 | 1500–2500 |
| `long` | ~20 | 60–100 | 3000–5000 |

Rough heuristic: ~150 words per minute of spoken audio.

### 3. Text-to-Speech (`internal/tts/`)

Converts script segments to audio using the ElevenLabs API.

#### Interface

```go
type TTSClient interface {
    Synthesize(ctx context.Context, segment Segment, voiceID string) ([]byte, error)
}
```

#### ElevenLabs Configuration

| Setting | Value |
|---------|-------|
| Model | `eleven_multilingual_v2` |
| Output format | `mp3_44100_128` |
| Stability | `0.5` |
| Similarity boost | `0.75` |
| Style | `0.0` (default) |
| Use speaker boost | `true` |

#### API Endpoint

```
POST https://api.elevenlabs.io/v1/text-to-speech/{voice_id}
```

Request body:

```json
{
  "text": "segment text here",
  "model_id": "eleven_multilingual_v2",
  "output_format": "mp3_44100_128",
  "voice_settings": {
    "stability": 0.5,
    "similarity_boost": 0.75,
    "style": 0.0,
    "use_speaker_boost": true
  }
}
```

Response: raw MP3 audio bytes.

#### Voice Assignment

Default voices (ElevenLabs pre-made voices):

| Persona | Default Voice | Characteristics |
|---------|--------------|-----------------|
| Alex | Selected at implementation | Clear, warm, conversational male voice |
| Sam | Selected at implementation | Articulate, thoughtful female voice |

Users override defaults with `--voice-alex <id>` and `--voice-sam <id>`.

#### Processing Strategy

Segments are processed **sequentially** (not concurrently) to:
1. Respect ElevenLabs rate limits
2. Maintain predictable resource usage
3. Allow progress reporting per segment

Each segment's audio is written to a temp file. A retry wrapper handles transient failures (429, 500, 503) with exponential backoff (3 max attempts, starting at 1s).

### 4. Audio Assembly (`internal/assembly/`)

Concatenates individual segment audio files into a single MP3 using FFmpeg.

#### Interface

```go
type Assembler interface {
    Assemble(ctx context.Context, segments []string, output string) error
}
```

#### FFmpeg Strategy

**Concat demuxer approach:**

1. Generate silence audio file (200ms of silence as MP3)
2. Create a concat list file:
   ```
   file 'segment_001.mp3'
   file 'silence.mp3'
   file 'segment_002.mp3'
   file 'silence.mp3'
   ...
   ```
3. Run FFmpeg:
   ```
   ffmpeg -f concat -safe 0 -i list.txt -c copy output.mp3
   ```

The `-c copy` flag avoids re-encoding, keeping the process fast and lossless.

#### Silence Generation

```
ffmpeg -f lavfi -i anullsrc=r=44100:cl=stereo -t 0.2 -c:a libmp3lame -b:a 128k silence.mp3
```

Generates 200ms of silence at matching sample rate and bitrate.

## CLI Interface Design

### Command Structure (cobra)

```
podcaster
├── generate          Primary command — run the full pipeline
│   ├── -i, --input       Source content (URL, PDF path, or text file path) [required]
│   ├── -o, --output      Output MP3 file path [required]
│   ├── --topic           Focus the conversation on a specific topic
│   ├── --tone            Conversation tone: casual, technical, educational [default: casual]
│   ├── --duration        Target duration: short, medium, long [default: medium]
│   ├── --voice-alex      ElevenLabs voice ID for Alex
│   ├── --voice-sam       ElevenLabs voice ID for Sam
│   ├── --script-only     Output script JSON only, skip TTS and assembly
│   ├── --from-script     Generate audio from an existing script JSON file
│   └── --verbose         Enable detailed logging
└── version           Print version information
```

### Environment Variables

| Variable | Purpose |
|----------|---------|
| `ANTHROPIC_API_KEY` | Claude API authentication |
| `ELEVENLABS_API_KEY` | ElevenLabs API authentication |

The CLI checks for these at startup and exits with a clear message if missing.

### Progress Output

```
$ podcaster generate -i paper.pdf -o episode.mp3

  Ingesting content... done (3,247 words from paper.pdf)
  Generating script... done (42 segments, ~10 min)
  Synthesizing audio... [18/42] ████████████░░░░░░░░░ 43%
  Assembling episode... done

  ✓ Episode saved to episode.mp3 (10:23, 9.8 MB)
```

## Project Structure

```
podcaster/
├── cmd/
│   └── podcaster/
│       └── main.go              # Entry point
├── internal/
│   ├── cli/
│   │   └── root.go              # Cobra command definitions
│   ├── pipeline/
│   │   └── pipeline.go          # Orchestrator — runs stages in sequence
│   ├── ingest/
│   │   ├── ingest.go            # Ingester interface + source detection
│   │   ├── url.go               # URL/article extraction
│   │   ├── pdf.go               # PDF text extraction
│   │   └── text.go              # Plain text file reader
│   ├── script/
│   │   ├── script.go            # Script generator interface
│   │   ├── claude.go            # Claude API client + prompt construction
│   │   └── prompt.go            # Prompt templates and persona definitions
│   ├── tts/
│   │   ├── tts.go               # TTS interface
│   │   └── elevenlabs.go        # ElevenLabs API client
│   └── assembly/
│       └── ffmpeg.go            # FFmpeg concat assembly
├── docs/
│   ├── PR-FAQ.md
│   ├── PRD.md
│   └── SPEC.md
├── go.mod
├── go.sum
├── Makefile
├── CLAUDE.md
└── README.md
```

## Error Handling

### Strategy

Every stage returns errors that propagate up to the pipeline orchestrator. Errors include context about which stage failed and why.

```go
type PipelineError struct {
    Stage   string // "ingest", "script", "tts", "assembly"
    Message string
    Err     error
}
```

### Retry Logic

Retries apply to external API calls (Claude and ElevenLabs):

| Parameter | Value |
|-----------|-------|
| Max attempts | 3 |
| Initial backoff | 1 second |
| Backoff multiplier | 2x |
| Max backoff | 10 seconds |
| Retryable status codes | 429, 500, 502, 503 |

### Common Failure Modes

| Failure | Stage | Handling |
|---------|-------|----------|
| Invalid URL / 404 | Ingest | Exit with "could not fetch URL" message |
| PDF corrupted or encrypted | Ingest | Exit with "could not read PDF" message |
| Missing API key | Startup | Exit with message specifying which key is missing |
| Claude API rate limited | Script | Retry with backoff |
| ElevenLabs rate limited | TTS | Retry with backoff |
| Claude returns invalid JSON | Script | Retry (up to 3x), then exit with parse error |
| FFmpeg not installed | Assembly | Exit with "FFmpeg not found — install with: brew install ffmpeg" |
| Disk full | Assembly | Exit with OS error (passthrough) |

### Temp File Cleanup

TTS audio segments are stored in a temp directory created per run. The pipeline defers cleanup:

```go
tmpDir, _ := os.MkdirTemp("", "podcaster-*")
defer os.RemoveAll(tmpDir)
```

On failure, temp files are still cleaned up. On success, only the final output file remains.

## Implementation Phases

### Phase 1: Foundation
- [ ] Initialize Go module, set up project structure
- [ ] Implement cobra CLI with `generate` and `version` commands
- [ ] Parse and validate all flags
- [ ] Implement pipeline orchestrator skeleton (stages called in sequence)
- [ ] Wire up environment variable loading for API keys

### Phase 2: Ingestion
- [ ] Implement source type detection
- [ ] Implement plain text reader
- [ ] Implement URL ingestion with `go-readability`
- [ ] Implement PDF ingestion with `ledongthuc/pdf`
- [ ] Add word count and title extraction

### Phase 3: Script Generation
- [ ] Set up Claude API client with `anthropic-sdk-go`
- [ ] Build prompt templates with persona definitions
- [ ] Implement scratchpad reasoning in prompt
- [ ] Parse structured JSON response into Script type
- [ ] Implement `--script-only` output mode
- [ ] Implement `--topic`, `--tone`, `--duration` flag handling in prompts

### Phase 4: Text-to-Speech
- [ ] Implement ElevenLabs HTTP client
- [ ] Wire up voice settings (stability, similarity boost)
- [ ] Process segments sequentially with progress output
- [ ] Implement retry with exponential backoff
- [ ] Implement `--voice-alex` and `--voice-sam` overrides
- [ ] Implement `--from-script` input mode

### Phase 5: Audio Assembly
- [ ] Implement silence generation via FFmpeg
- [ ] Build concat list file from segment audio paths
- [ ] Execute FFmpeg concat
- [ ] Verify output file exists and has reasonable size
- [ ] Report final episode duration and file size

### Phase 6: Polish
- [ ] Progress bar / spinner output
- [ ] `--verbose` logging throughout pipeline
- [ ] Error messages for all failure modes
- [ ] Makefile with `build`, `install`, `clean` targets
- [ ] README with install and usage instructions
