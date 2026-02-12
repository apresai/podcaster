# Podcaster

CLI tool that converts written content (URLs, PDFs, text files) into two-host podcast-style audio conversations.

## Tech Stack

- **Language**: Go
- **CLI framework**: cobra
- **Script generation**: Claude API via `anthropic-sdk-go`, or Gemini API via raw HTTP
- **Text-to-speech**: Gemini TTS (default), Vertex AI Express (API key), Vertex AI (ADC), ElevenLabs, or Google Cloud TTS
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
make deploy                  # clean → build → portal → CDK deploy → docker push → AgentCore update → verify
make deploy-infra            # CDK deploy only (ECR, CloudFront, Lambda, IAM)
make docker-push             # Build + push ARM64 container to ECR
make force-update-agentcore  # Force all AgentCore runtimes to re-pull container
make verify-deploy           # Poll AgentCore runtime until READY (3-min timeout)

# Testing
make smoke-test              # Quick MCP ping test against deployed AgentCore server
make smoke-test-local        # Quick MCP ping test against local server (localhost:8000)
make smoke-test-proxy        # Quick MCP ping test against public proxy (requires API_KEY=pk_...)
make build-proxy             # Build proxy Lambda binary to deploy/proxy-build/bootstrap

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
│   ├── mcp-server/main.go       # Remote MCP server entry point (AgentCore)
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
│   │   ├── express.go           # Vertex AI Express (API key auth)
│   │   ├── gemini.go            # Gemini multi-speaker TTS (AI Studio)
│   │   ├── vertex.go            # Vertex AI TTS (ADC/OAuth2 auth)
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
├── portal/                      # Next.js web portal (OpenNext → Lambda + CloudFront)
│   ├── src/app/                 # App Router pages + API routes
│   │   ├── (authenticated)/     # Auth-gated pages (dashboard, create, api-keys, usage, docs)
│   │   │   └── create/          # Create podcast page (form → MCP proxy → AgentCore)
│   │   └── api/                 # API routes (keys, usage, generate, voices)
│   ├── src/lib/db.ts            # DynamoDB operations + API key encryption
│   ├── src/lib/mcp.ts           # MCP client (portal → proxy Lambda → AgentCore)
│   └── open-next.config.ts      # OpenNext build configuration
├── deploy/
│   ├── Dockerfile               # Multi-stage ARM64 container for MCP server
│   └── infrastructure/          # CDK stack (ECR, CloudFront, Lambda, DynamoDB, S3, IAM)
├── scripts/
│   └── migrate-data/main.go     # One-time DynamoDB + S3 migration script
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
| `VERTEX_AI_API_KEY` | Vertex AI Express (TTS) | `--tts vertex-express` |
| `ELEVENLABS_API_KEY` | ElevenLabs (TTS) | `--tts elevenlabs` |
| `GCP_PROJECT` | GCP project ID | `--tts gemini-vertex` |
| `GCP_REGION` | GCP region (default: us-central1) | `--tts gemini-vertex` (optional) |
| ADC / `GOOGLE_APPLICATION_CREDENTIALS` | GCP OAuth2 | `--tts gemini-vertex` or `--tts google` |
| `GCP_SERVICE_ACCOUNT_JSON` | Secrets Manager only | Auto-sets `GOOGLE_APPLICATION_CREDENTIALS` + `GCP_PROJECT` on AgentCore |
| *(AWS default creds)* | AWS Polly (TTS) | `--tts polly` (no API key — uses AWS credential chain) |

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
| `gemini-flash` | `gemini-3-flash-preview` | Google |
| `gemini-pro` | `gemini-3-pro-preview` | Google |
| `nova-lite` | `us.amazon.nova-2-lite-v1:0` | AWS Bedrock |

## TTS Providers

| Flag value | Endpoint | Auth | Rate Limit | Notes |
|------------|----------|------|------------|-------|
| `gemini` (default) | AI Studio (`generativelanguage.googleapis.com`) | API key (`GEMINI_API_KEY`) | 10 RPM, 100 RPD | 7s inter-segment throttle |
| `vertex-express` | Vertex AI Express (`aiplatform.googleapis.com`) | API key (`VERTEX_AI_API_KEY`) | TBD (higher than AI Studio) | GA model names, no `-preview`; recommended for testing |
| `gemini-vertex` | Vertex AI (`{region}-aiplatform.googleapis.com`) | ADC/OAuth2 | 30,000 RPM | 500ms polite delay; requires `GCP_PROJECT` env var |
| `elevenlabs` | ElevenLabs API | API key | Varies by plan | |
| `google` | Cloud TTS gRPC (`texttospeech.googleapis.com`) | ADC/OAuth2 | 150 RPM | Chirp 3 HD voices (different from Gemini voices) |
| `polly` | AWS Polly (`polly.{region}.amazonaws.com`) | AWS default creds | Standard AWS limits | Generative engine only, 7 English voices, MP3 output |

All Gemini TTS providers (gemini, vertex-express, gemini-vertex) share the same voice names (Charon, Leda, Fenrir, etc.).

**vertex-express vs gemini**: Both use API key auth. `vertex-express` hits the Vertex AI endpoint (`aiplatform.googleapis.com`) with GA model names and requires `"role": "user"` in the request contents. It uses `VERTEX_AI_API_KEY` (a Google Cloud API key for Vertex AI, not an AI Studio key). Created to test whether Vertex AI express mode has higher daily quotas than AI Studio.

## MCP Server

Remote MCP server deployed on AWS Bedrock AgentCore. Runs the pipeline as a goroutine, tracks via DynamoDB, uploads MP3 to S3, served via CloudFront CDN.

### MCP Tools

| Tool | Description |
|------|-------------|
| `generate_podcast` | Start async generation. Params: `input_url`/`input_text`, `model`, `tts`, `format`, `duration`, `tone`, `topic`, `style`, `voices`, `voice1`/`voice2`/`voice3`, `tts_model`, `tts_speed`, `tts_stability`, `tts_pitch`, plus BYOK API keys. |
| `get_podcast` | Poll status by podcast_id. Returns progress, audio_url when complete. |
| `list_podcasts` | Paginated list of podcasts with `limit` and `cursor`. |
| `list_voices` | List available voices for a TTS provider (required param: `provider`). |
| `list_options` | List all formats, styles, TTS providers, models, and durations (no params). |
| `server_info` | Runtime diagnostics. |

### Resources

Podcaster owns all its AWS resources (fully independent from other projects):
- **S3 audio bucket**: `podcaster-audio-{account_id}` — podcast MP3 files
- **DynamoDB table**: `podcaster-prod` — podcast metadata, users, API keys, usage
- **CloudFront distribution**: `podcasts.apresai.dev` — serves portal + `/audio/*` CDN
- **Route53 hosted zone**: `apresai.dev` (lookup only — shared across projects, never created/deleted by any stack)

### Local Testing

```bash
# Run MCP server locally (uses env vars for API keys, AWS creds for DynamoDB/S3)
S3_BUCKET=podcaster-audio-228029809749 DYNAMODB_TABLE=podcaster-prod \
  SECRET_PREFIX="" go run ./cmd/mcp-server

# Test via curl (MCP StreamableHTTP on port 8000)
# 1. Initialize
curl -s http://localhost:8000/mcp -d '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","clientInfo":{"name":"test"},"capabilities":{}}}' -H 'Content-Type: application/json'

# 2. Call generate_podcast (use gemini-flash + short to minimize cost)
curl -s http://localhost:8000/mcp -d '{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"generate_podcast","arguments":{"input_url":"https://example.com","model":"gemini-flash","duration":"short"}}}' -H 'Content-Type: application/json' -H 'Mcp-Session-Id: <session_id_from_init>'

# 3. Poll get_podcast
curl -s http://localhost:8000/mcp -d '{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"get_podcast","arguments":{"podcast_id":"<id>"}}}' -H 'Content-Type: application/json' -H 'Mcp-Session-Id: <session_id>'
```

**Cost-saving tip**: Use `gemini-flash` model + `short` duration for testing. This uses only Gemini API (no Anthropic costs) and generates ~3-4min/15 segments.

### MCP Proxy (Public Endpoint)

A thin Go Lambda at `cmd/mcp-proxy/main.go` acts as an auth proxy, served via CloudFront at `https://podcasts.apresai.dev/mcp`.

- Validates `Authorization: Bearer pk_...` API keys against DynamoDB
- Injects `_user_id`/`_key_id` into `tools/call` arguments
- Forwards to AgentCore via `invoke-agent-runtime` (SigV4, automatic via Lambda role)
- Accepts `application/json, text/event-stream` responses (SSE support)
- Returns AgentCore response to client

**Build**: `make build-proxy` (produces `deploy/proxy-build/bootstrap`)
**Test**: `make smoke-test-proxy API_KEY=pk_...`

**CloudFront behavior**: `/mcp` -> proxy Lambda Function URL (POST, no caching, forwards Authorization header)

### Portal Create Page

The portal at `podcasts.apresai.dev/create` lets authenticated users generate podcasts directly from the browser:

- Multi-step form: source URL/text → model + TTS → format + style + duration
- Calls portal API routes (`/api/generate`, `/api/generate/[id]`, `/api/voices`)
- Portal backend uses the user's encrypted API key to call the MCP proxy on their behalf
- MCP client helper: `portal/src/lib/mcp.ts` — sends JSON-RPC to `GATEWAY_URL` (proxy Lambda Function URL)
- `GATEWAY_URL` is injected into the portal Lambda by CDK (points to the proxy Lambda Function URL)

### API Key Encryption

API keys are encrypted at rest in DynamoDB using AES-256-GCM so the portal can decrypt them for server-side MCP calls:

- **Encryption key**: CDK-managed Secrets Manager secret at `/podcaster/portal/PORTAL_ENCRYPTION_KEY` (64-char hex = 32 bytes)
- **Portal Lambda env var**: `PORTAL_ENCRYPTION_KEY` (resolved at deploy time via `unsafeUnwrap()`)
- **Storage format**: `iv:ciphertext:authTag` (all base64) in the `encryptedKey` field of the `APIKEY#` DynamoDB item
- **Implementation**: `encryptAPIKey()` / `decryptAPIKey()` / `getActiveAPIKeyForUser()` in `portal/src/lib/db.ts`
- Keys created before encryption was added won't have `encryptedKey` — users can revoke and recreate

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

## Debugging AWS Failures

When something fails on AWS (deploys, AgentCore errors, Lambda failures, networking issues, etc.), **always spawn a cloud-architect agent** to investigate the AWS logs first before attempting fixes. The agent should:
1. Check CloudWatch logs for the relevant service (AgentCore, Lambda, API Gateway, etc.)
2. Look for error patterns, timing, and root causes
3. Report findings before any code changes are made

Do not guess at the cause of AWS failures — check the logs. Do not retry the same failing action repeatedly without understanding why it failed.

## Gemini TTS Rate Limits

AI Studio's strict rate limits are the bottleneck for the default `gemini` TTS provider (Paid Tier 1):

| Model | RPM | TPM | RPD |
|-------|-----|-----|-----|
| Flash TTS (`gemini-2.5-flash-preview-tts`) | 10 | 10K | 100 |
| Pro TTS (`gemini-2.5-pro-preview-tts`) | 10 | 10K | 50 |

At 10 RPM, a 40-segment standard podcast uses 40 of 100 daily requests — about 2 podcasts/day.

**Mitigations for `--tts gemini` on AgentCore:**
- `DisableBatch=true` in `tasks.go` — per-segment mode with 7s throttle to stay under 10 RPM
- Batch mode is only for CLI/local use (single request, no rate limit concern)
- Graceful shutdown: SIGTERM cancels pipeline goroutines → FailJob writes to DynamoDB

**Better alternatives:** Use `--tts vertex-express` (API key auth, higher quotas TBD) or `--tts gemini-vertex` (ADC auth, 30K RPM).

## Vertex AI / Cloud TTS Endpoints

Three Vertex AI TTS endpoints are available:

### 1. Vertex AI Express (implemented as `vertex-express`)
- Endpoint: `aiplatform.googleapis.com` (no region prefix)
- Auth: Google Cloud API key (`VERTEX_AI_API_KEY`) — no service account needed
- Request format: single `contents` field with `"role": "user"` (required, unlike AI Studio)
- GA model names: `gemini-2.5-flash-tts`, `gemini-2.5-pro-tts` (no `-preview`)
- Rate limits TBD — expected higher daily quotas than AI Studio
- Implementation: `internal/tts/express.go`

### 2. Cloud Text-to-Speech API
- Endpoint: `{REGION}-texttospeech.googleapis.com`
- 150 RPM Flash / 125 RPM Pro (15x AI Studio)
- Request format: separate `prompt` and `text` fields (max 5K bytes each, 8K combined)
- Auth: Service account / ADC (not API key)
- GA model names: `gemini-2.5-flash-tts`, `gemini-2.5-pro-tts` (no `-preview`)

### 3. Vertex AI API (implemented as `gemini-vertex`)
- Endpoint: `{REGION}-aiplatform.googleapis.com`
- System limit: 30,000 RPM per model per region
- Request format: single `contents` field (like AI Studio)
- Auth: service account / ADC
- Supports `temperature` control (0.0-2.0)
- Implementation: `internal/tts/vertex.go`

### Requirements for ADC-based providers (gemini-vertex, google)
- GCP project with billing enabled (`chadneal-learning-1`)
- Enable Cloud Text-to-Speech API or Vertex AI API
- Service account with `roles/aiplatform.user`
- **Local**: Set `GOOGLE_APPLICATION_CREDENTIALS` to the SA JSON file path
- **AgentCore**: Store SA JSON in Secrets Manager as `GCP_SERVICE_ACCOUNT_JSON` — `loadSecrets()` writes it to a temp file and sets `GOOGLE_APPLICATION_CREDENTIALS` + `GCP_PROJECT` automatically
- Upload with: `GCP_SERVICE_ACCOUNT_FILE=~/path/to/sa.json make create-secrets`

### Go SDK options
- `cloud.google.com/go/vertexai/genai` — Vertex AI SDK
- `cloud.google.com/go/texttospeech` — Cloud TTS SDK
- Or raw HTTP (current approach, just change endpoint + auth)

## Development Notes

- Default script model: Haiku 4.5 (`--model haiku`)
- Default TTS provider: Gemini (`--tts gemini`)
- ElevenLabs output format: `mp3_44100_192` (44.1kHz, 192kbps)
- Gemini PCM→MP3 conversion: 192kbps, soxr resampler, LAME quality 0
- Silence between segments: 200ms
- FFmpeg concat uses `-c copy` (no re-encoding)
- Go module path: `github.com/apresai/podcaster`
