# Podcaster

CLI tool that converts written content (URLs, PDFs, text files) into two-host podcast-style audio conversations.

## Tech Stack

- **Language**: Go
- **CLI framework**: cobra
- **Script generation**: Claude API via `anthropic-sdk-go`, or Gemini API via raw HTTP
- **Text-to-speech**: Gemini TTS (default), ElevenLabs, or Google Cloud TTS
- **Audio assembly**: FFmpeg (concat demuxer)
- **PDF extraction**: `ledongthuc/pdf`
- **URL extraction**: `go-shiori/go-readability`

## Commands

```bash
# CLI
make build                   # Build binary to ./podcaster
make install                 # Build and install to $GOPATH/bin
make clean                   # Remove build artifacts
make dev                     # Build and run with sample input

# Deploy (full pipeline)
make deploy                  # clean → build → CDK deploy → docker push → AgentCore update → verify
make deploy-infra            # CDK deploy only (ECR, CloudFront, Lambda, IAM)
make docker-push             # Build + push ARM64 container to ECR
make force-update-agentcore  # Force all AgentCore runtimes to re-pull container
make verify-deploy           # Poll AgentCore runtime until READY (3-min timeout)

# Testing
make smoke-test              # Quick MCP ping test against deployed AgentCore server
make smoke-test-local        # Quick MCP ping test against local server (localhost:8000)

# First-time setup
make deploy-agentcore        # Register new AgentCore runtime (one-time)
make create-secrets          # Store API keys in Secrets Manager
```

## Running

```bash
# Required environment variables (depends on --model and --tts choices)
export ANTHROPIC_API_KEY="sk-ant-..."   # For --model haiku/sonnet
export GEMINI_API_KEY="..."              # For --model gemini-flash/gemini-pro or --tts gemini
export ELEVENLABS_API_KEY="..."          # For --tts elevenlabs

# Generate from URL (defaults: --model haiku, --tts gemini)
podcaster generate -i https://example.com/article -o episode.mp3

# Generate from PDF
podcaster generate -i paper.pdf -o episode.mp3

# Script only (no audio)
podcaster generate -i article.txt -o script.json --script-only

# Audio from existing script
podcaster generate --from-script script.json -o episode.mp3

# Choose script generation model
podcaster generate -i input.txt -o out.mp3 --model sonnet
podcaster generate -i input.txt -o out.mp3 --model gemini-flash

# With options
podcaster generate -i input.txt -o out.mp3 --topic "key findings" --tone technical --duration long
```

## Project Structure

```
podcaster/
├── cmd/
│   ├── podcaster/main.go        # CLI entry point
│   ├── mcp-server/main.go       # Remote MCP server entry point
│   └── play-counter/main.go     # CloudFront log → play count Lambda
├── internal/
│   ├── cli/
│   │   ├── root.go              # Cobra command definitions + flags
│   │   ├── interactive.go       # TUI interactive setup wizard
│   │   └── publish.go           # MCP publish command
│   ├── pipeline/pipeline.go     # Orchestrator — runs stages in sequence
│   ├── ingest/                  # Content extraction (URL, PDF, text)
│   │   ├── ingest.go            # Interface + source detection
│   │   ├── url.go
│   │   ├── pdf.go
│   │   └── text.go
│   ├── script/                  # Script generation
│   │   ├── script.go            # Interface + types + NewGenerator factory
│   │   ├── claude.go            # Claude API client (haiku/sonnet)
│   │   ├── gemini.go            # Gemini API client (flash/pro)
│   │   ├── personas.go          # Persona type + default host personalities
│   │   ├── prompt.go            # Dynamic prompt builder from personas
│   │   ├── format.go            # Show format definitions (8 formats)
│   │   └── review.go            # Script refinement (heuristic + LLM review)
│   ├── tts/                     # Text-to-speech (multi-provider)
│   │   ├── provider.go          # Interface + factory + retry + cross-provider mixing
│   │   ├── tts.go               # Voice selection helper
│   │   ├── elevenlabs.go        # ElevenLabs client
│   │   ├── gemini.go            # Gemini multi-speaker TTS
│   │   └── google.go            # Google Cloud TTS (Chirp 3 HD)
│   ├── mcpserver/               # Remote MCP server (AgentCore)
│   │   ├── server.go            # Server setup — AWS config, Secrets Manager
│   │   ├── store.go             # DynamoDB CRUD for podcast jobs
│   │   ├── storage.go           # S3 upload for MP3 files
│   │   ├── tasks.go             # Task goroutine lifecycle + progress
│   │   └── tools.go             # MCP tool definitions + handlers
│   ├── observability/           # Telemetry
│   │   ├── tracing.go           # OpenTelemetry tracing setup
│   │   ├── logging.go           # Structured logging
│   │   └── context.go           # Context helpers
│   ├── progress/                # Progress reporting
│   │   ├── progress.go          # Stage, Event, Callback types
│   │   └── renderer.go          # Terminal progress bar renderer
│   └── assembly/
│       └── ffmpeg.go            # FFmpeg audio concatenation
├── deploy/
│   ├── Dockerfile               # Multi-stage ARM64 container for MCP server
│   └── infrastructure/          # CDK stack (ECR, CloudFront, Lambda, IAM)
├── docs/                        # PR-FAQ, PRD, SPEC, roadmap
├── .claude/skills/              # Claude Code skills (generate-persona)
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

## Persona System

Host personalities are defined in `internal/script/personas.go`. Each `Persona` struct includes a backstory, speaking style, catchphrases, expertise, and an independence clause that prevents hosts from identifying with any company they discuss. The system prompt is dynamically built from persona fields via `buildSystemPrompt()` in `prompt.go`.

Use `/generate-persona` to create new personas for custom voices.

## API Keys

| Variable | Service | Needed when |
|----------|---------|-------------|
| `ANTHROPIC_API_KEY` | Claude (script gen) | `--model haiku` or `--model sonnet` |
| `GEMINI_API_KEY` | Gemini (script gen + TTS) | `--model gemini-*` or `--tts gemini` |
| `ELEVENLABS_API_KEY` | ElevenLabs (TTS) | `--tts elevenlabs` |

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

## Script Generation Models

| Flag value | Model ID | Provider |
|------------|----------|----------|
| `haiku` (default) | `claude-haiku-4-5-20251001` | Anthropic |
| `sonnet` | `claude-sonnet-4-5-20250929` | Anthropic |
| `gemini-flash` | `gemini-2.5-flash` | Google |
| `gemini-pro` | `gemini-2.5-pro` | Google |

## MCP Server

Remote MCP server deployed on AWS Bedrock AgentCore. Runs the pipeline as a goroutine, tracks via DynamoDB, uploads MP3 to S3, served via CloudFront CDN.

### Local Testing

```bash
# Run MCP server locally (uses env vars for API keys, AWS creds for DynamoDB/S3)
S3_BUCKET=apresai-podcasts-228029809749 DYNAMODB_TABLE=apresai-podcasts-prod \
  SECRET_PREFIX="" go run ./cmd/mcp-server

# Test via curl (MCP StreamableHTTP on port 8000)
# 1. Initialize
curl -s http://localhost:8000/mcp -d '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","clientInfo":{"name":"test"},"capabilities":{}}}' -H 'Content-Type: application/json'

# 2. Call generate_podcast (use gemini-flash + short to minimize cost)
curl -s http://localhost:8000/mcp -d '{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"generate_podcast","arguments":{"input_url":"https://example.com","model":"gemini-flash","duration":"short"}}}' -H 'Content-Type: application/json' -H 'Mcp-Session-Id: <session_id_from_init>'

# 3. Poll get_podcast
curl -s http://localhost:8000/mcp -d '{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"get_podcast","arguments":{"podcast_id":"<id>"}}}' -H 'Content-Type: application/json' -H 'Mcp-Session-Id: <session_id>'
```

**Cost-saving tip**: Use `gemini-flash` model + `short` duration for testing. This uses only Gemini API (no Anthropic costs) and generates ~8min/30 segments.

### Deployment

```bash
# Full deploy (recommended — runs all steps in order)
make deploy

# Individual steps (for debugging or partial deploys)
make deploy-infra                        # CDK deploy (ECR, CloudFront, Lambda, IAM)
make docker-push                         # Build + push ARM64 container to ECR
make force-update-agentcore              # Force AgentCore to pull latest image
make verify-deploy                       # Wait for AgentCore runtime to be READY

# First-time setup
make deploy-agentcore                    # Register new AgentCore runtime
make create-secrets                      # Store API keys in Secrets Manager

# Verification
make smoke-test                          # Test deployed server (initialize + tools/list)
make smoke-test-local                    # Test local server at localhost:8000
```

## Testing Policy

- **Always test AWS features on AWS** — when the feature involves AgentCore, MCP server, or any AWS-deployed service, test against the deployed AgentCore endpoint (not locally) unless explicitly told to test locally.
- Local testing (`go run ./cmd/mcp-server`) is only for quick iteration on code changes before deploying.

## Development Notes

- Default script model: Haiku 4.5 (`--model haiku`)
- Default TTS provider: Gemini (`--tts gemini`)
- ElevenLabs output format: `mp3_44100_192` (44.1kHz, 192kbps)
- Gemini PCM→MP3 conversion: 192kbps, soxr resampler, LAME quality 0
- Silence between segments: 200ms
- FFmpeg concat uses `-c copy` (no re-encoding)
- Go module path: `github.com/apresai/podcaster`
