# Podcaster

CLI tool that converts written content (URLs, PDFs, text files) into two-host podcast-style audio conversations.

## Tech Stack

- **Language**: Go
- **CLI framework**: cobra
- **Script generation**: Claude API via `anthropic-sdk-go`
- **Text-to-speech**: ElevenLabs API (`eleven_multilingual_v2`)
- **Audio assembly**: FFmpeg (concat demuxer)
- **PDF extraction**: `ledongthuc/pdf`
- **URL extraction**: `go-shiori/go-readability`

## Commands

```bash
make build       # Build binary to ./bin/podcaster
make install     # Build and install to $GOPATH/bin
make clean       # Remove build artifacts
make dev         # Build and run with sample input
```

## Running

```bash
# Required environment variables
export ANTHROPIC_API_KEY="sk-ant-..."
export ELEVENLABS_API_KEY="..."

# Generate from URL
podcaster generate -i https://example.com/article -o episode.mp3

# Generate from PDF
podcaster generate -i paper.pdf -o episode.mp3

# Script only (no audio)
podcaster generate -i article.txt -o script.json --script-only

# Audio from existing script
podcaster generate --from-script script.json -o episode.mp3

# With options
podcaster generate -i input.txt -o out.mp3 --topic "key findings" --tone technical --duration long
```

## Project Structure

```
podcaster/
├── cmd/podcaster/main.go        # Entry point
├── internal/
│   ├── cli/root.go              # Cobra command definitions
│   ├── pipeline/pipeline.go     # Orchestrator — runs stages in sequence
│   ├── ingest/                  # Content extraction (URL, PDF, text)
│   │   ├── ingest.go            # Interface + source detection
│   │   ├── url.go
│   │   ├── pdf.go
│   │   └── text.go
│   ├── script/                  # Script generation via Claude
│   │   ├── script.go            # Interface + types
│   │   ├── claude.go            # Claude API client
│   │   └── prompt.go            # Prompt templates
│   ├── tts/                     # Text-to-speech via ElevenLabs
│   │   ├── tts.go               # Interface
│   │   └── elevenlabs.go        # ElevenLabs client
│   └── assembly/
│       └── ffmpeg.go            # FFmpeg audio concatenation
├── docs/                        # PR-FAQ, PRD, SPEC
├── go.mod
├── Makefile
└── CLAUDE.md
```

## Key Patterns

- **Pipeline architecture**: Four stages (ingest → script → TTS → assembly) run sequentially via the orchestrator in `internal/pipeline/`
- **Interfaces per stage**: Each stage defines an interface, making components swappable
- **Structured output**: Script generation returns JSON parsed into Go structs — never use `map[string]interface{}`
- **Sequential TTS**: Segments are synthesized one at a time (not concurrently) to respect rate limits
- **Retry with backoff**: All external API calls use exponential backoff (3 attempts, 1s initial, 2x multiplier)
- **Temp file cleanup**: TTS segments go in a per-run temp dir, cleaned up via `defer os.RemoveAll()`

## API Keys

| Variable | Service | Get it at |
|----------|---------|-----------|
| `ANTHROPIC_API_KEY` | Claude (script generation) | https://console.anthropic.com/ |
| `ELEVENLABS_API_KEY` | ElevenLabs (TTS) | https://elevenlabs.io/ |

## External Dependencies

- **FFmpeg** must be installed on the system: `brew install ffmpeg`

## Data Flow

```
Input → [Ingest] → plain text → [Script Gen] → []Segment JSON → [TTS] → []MP3 files → [Assembly] → final MP3
```

## Script JSON Format

```json
{
  "title": "Episode Title",
  "summary": "Brief description",
  "segments": [
    {"speaker": "Alex", "text": "Welcome to the show..."},
    {"speaker": "Sam", "text": "Thanks Alex, today we're..."}
  ]
}
```

## Voice Configuration

| Host | Role | ElevenLabs Settings |
|------|------|-------------------|
| Alex | Host/driver | stability: 0.5, similarity_boost: 0.75 |
| Sam | Analyst/questioner | stability: 0.5, similarity_boost: 0.75 |

Override with `--voice-alex <id>` and `--voice-sam <id>` flags.

## Development Notes

- Model for script gen: `claude-sonnet-4-5-20250929`
- TTS output format: `mp3_44100_128` (44.1kHz, 128kbps)
- Silence between segments: 200ms
- FFmpeg concat uses `-c copy` (no re-encoding)
- Go module path: likely `github.com/chad/podcaster` (update after repo creation)
