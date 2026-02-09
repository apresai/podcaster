# Podcaster

CLI tool that converts written content (URLs, PDFs, text files) into podcast-style audio conversations with 1-3 AI hosts.

## Quick Start

```bash
# Install dependencies
brew install ffmpeg
go install github.com/apresai/podcaster/cmd/podcaster@latest

# Set API key (Gemini is the default for both script gen and TTS)
export GEMINI_API_KEY="..."

# Generate from a URL
podcaster generate -i https://example.com/article

# Generate from a PDF
podcaster generate -i paper.pdf -o episode.mp3

# Interactive setup wizard
podcaster generate --tui
```

Output filename is auto-generated from the episode title when `-o` is omitted.

## Build from Source

```bash
git clone https://github.com/apresai/podcaster.git
cd podcaster
make build       # Build to ./podcaster
make install     # Install to $GOPATH/bin
```

## Requirements

- **Go 1.23+**
- **FFmpeg** — `brew install ffmpeg`
- **Gemini API key** — [aistudio.google.com](https://aistudio.google.com/) (default for script gen + TTS)
- **Anthropic API key** (optional) — [console.anthropic.com](https://console.anthropic.com/) (for `--model haiku` or `--model sonnet`)
- **ElevenLabs API key** (optional) — [elevenlabs.io](https://elevenlabs.io/) (for `--tts elevenlabs`)

## Usage

```bash
podcaster generate -i <source> [options]
```

### Options

| Flag | Short | Description | Default |
|------|-------|-------------|---------|
| `--input` | `-i` | Source content (URL, PDF path, or text file) | required |
| `--output` | `-o` | Output MP3 path (auto-named from title if omitted) | auto |
| `--model` | `-m` | Script model: `haiku`, `sonnet`, `gemini-flash`, `gemini-pro` | `haiku` |
| `--tts` | `-T` | TTS provider: `gemini`, `elevenlabs`, `google` | `gemini` |
| `--format` | `-F` | Show format: `conversation`, `interview`, `deep-dive`, `explainer`, `debate`, `news`, `storytelling`, `challenger` | `conversation` |
| `--duration` | `-d` | Target length: `short` (~8min), `standard` (~18min), `long` (~35min), `deep` (~55min) | `standard` |
| `--tone` | `-n` | Conversation tone: `casual`, `technical`, `educational` | `casual` |
| `--topic` | `-p` | Focus the conversation on a specific topic | — |
| `--style` | `-s` | Conversation styles (comma-separated): `humor`, `wow`, `serious`, `debate`, `storytelling` | — |
| `--voices` | `-V` | Number of podcast hosts (1-3) | `2` |
| `--voice1` | `-1` | Voice for host 1 (`provider:voiceID` or plain voiceID) | — |
| `--voice2` | `-2` | Voice for host 2 | — |
| `--voice3` | `-3` | Voice for host 3 | — |
| `--tts-model` | | TTS model ID override | provider default |
| `--tts-speed` | | Speech speed (ElevenLabs: 0.7-1.2, Google: 0.25-2.0) | — |
| `--tts-stability` | | Voice stability, ElevenLabs only (0.0-1.0) | — |
| `--tts-pitch` | | Pitch in semitones, Google only (-20.0 to 20.0) | — |
| `--script-only` | `-S` | Output script JSON only, skip audio | `false` |
| `--from-script` | `-f` | Generate audio from existing script JSON | — |
| `--tui` | `-t` | Interactive setup wizard | `false` |
| `--verbose` | `-v` | Detailed logging | `false` |

API key flags (`--anthropic-api-key`, `--gemini-api-key`, `--elevenlabs-api-key`) override their respective environment variables.

### Script Workflow

Generate a script, review/edit it, then produce audio:

```bash
# Step 1: Generate script only
podcaster generate -i article.txt -o script.json --script-only

# Step 2: Edit script.json (optional — tweak dialogue, fix errors)

# Step 3: Generate audio from script
podcaster generate --from-script script.json -o episode.mp3
```

### Examples

```bash
# Quick test with Gemini (lowest cost)
podcaster generate -i https://example.com/blog --model gemini-flash --duration short

# Interview format with 3 hosts
podcaster generate -i paper.pdf --format interview --voices 3

# Technical deep-dive with Claude Sonnet
podcaster generate -i paper.pdf --model sonnet --tone technical --topic "security implications"

# Use ElevenLabs TTS with custom voices
podcaster generate -i notes.txt --tts elevenlabs --voice1 JBFqnCBsd6RMkjVDRZzb --voice2 EXAVITQu4vr4xnSDxMaL

# Cross-provider voice mixing
podcaster generate -i article.txt --voice1 gemini:Kore --voice2 elevenlabs:JBFqnCBsd6RMkjVDRZzb

# Interactive wizard
podcaster generate --tui
```

## How It Works

Four-stage pipeline:

1. **Ingest** — Extracts plain text from URL (via readability), PDF, or text file
2. **Script Gen** — AI generates a multi-host dialogue as structured JSON (with automatic script refinement)
3. **TTS** — Converts each segment to speech via Gemini, ElevenLabs, or Google Cloud TTS
4. **Assembly** — FFmpeg concatenates segments with 200ms silence gaps into final MP3

Script refinement (always on) checks segment count, speaker balance, and filler phrases. If issues are found, an LLM review pass revises the script automatically.

## Hosts

Host names are dynamically set from voice selection. Default names:

| Host | Default Role | Style |
|------|-------------|-------|
| **Alex** | Host/driver | Introduces topics, provides context, uses analogies |
| **Sam** | Analyst/questioner | Asks probing questions, challenges assumptions |
| **Jordan** | Connector (3-host mode) | Bridges ideas, draws parallels, synthesizes insights |

## Show Formats

| Format | Description |
|--------|-------------|
| `conversation` | Casual back-and-forth discussion (default) |
| `interview` | Structured Q&A with an interviewer and expert(s) |
| `deep-dive` | Thorough exploration of a complex topic |
| `explainer` | Educational walkthrough for a general audience |
| `debate` | Opposing viewpoints with structured arguments |
| `news` | Current events reporting and analysis |
| `storytelling` | Narrative-driven exploration |
| `challenger` | Devil's advocate format with rigorous questioning |

## Environment Variables

| Variable | Required | Purpose |
|----------|----------|---------|
| `GEMINI_API_KEY` | Yes (default config) | Gemini script gen + TTS |
| `ANTHROPIC_API_KEY` | Only for `--model haiku/sonnet` | Claude script generation |
| `ELEVENLABS_API_KEY` | Only for `--tts elevenlabs` | ElevenLabs TTS |

## Script Generation Models

| Flag value | Model | Provider |
|------------|-------|----------|
| `haiku` (default) | Claude Haiku 4.5 | Anthropic |
| `sonnet` | Claude Sonnet 4.5 | Anthropic |
| `gemini-flash` | Gemini 2.5 Flash | Google |
| `gemini-pro` | Gemini 2.5 Pro | Google |

## MCP Server

Podcaster is also available as a remote MCP server deployed on AWS Bedrock AgentCore. AI assistants can generate podcasts by calling MCP tools — no CLI needed.

**Tools**: `generate_podcast` (async), `get_podcast` (poll status), `list_podcasts` (browse history)

Audio is served via CloudFront CDN at `podcasts.apresai.dev`.

See [CLAUDE.md](CLAUDE.md) for deployment instructions and local testing.

### Connecting to the Remote MCP Server

The AgentCore MCP server is accessed via the AWS CLI `invoke-agent-runtime` API (not a public HTTP endpoint). MCP clients need a wrapper script that bridges stdio to the AWS API.

**Claude Desktop** (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "podcaster": {
      "command": "/path/to/podcaster-mcp-bridge.sh",
      "args": []
    }
  }
}
```

**Claude Code** (`~/.claude.json`):

```json
{
  "mcpServers": {
    "podcaster": {
      "command": "/path/to/podcaster-mcp-bridge.sh",
      "args": []
    }
  }
}
```

**VS Code** (`.vscode/mcp.json`):

```json
{
  "servers": {
    "podcaster": {
      "command": "/path/to/podcaster-mcp-bridge.sh",
      "args": []
    }
  }
}
```

The bridge script translates stdio MCP messages to `aws bedrock-agentcore-runtime invoke-agent-runtime` calls. Requires AWS CLI configured with credentials that have `bedrock-agentcore-runtime:InvokeAgentRuntime` permission.

> **Note**: The `awslabs.amazon-bedrock-agentcore-mcp-server` npm package is a deployment tool for AgentCore, not a client connector for custom runtimes.

## Cost

Typical cost per 10-minute episode with default settings (Gemini Flash + Gemini TTS): **~$0.10**.
Using Claude + ElevenLabs: **< $1**.

## Privacy

Source content is sent to the configured AI provider (Anthropic or Google) for script generation, and generated speech text is sent to the TTS provider. No content is stored by this tool beyond the output file. The MCP server stores podcast metadata in DynamoDB and audio in S3.

## License

MIT
