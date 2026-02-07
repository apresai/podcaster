# Podcaster

CLI tool that converts written content (URLs, PDFs, text files) into two-host podcast-style audio conversations.

## Quick Start

```bash
# Install dependencies
brew install ffmpeg
go install github.com/apresai/podcaster/cmd/podcaster@latest

# Set API keys
export ANTHROPIC_API_KEY="sk-ant-..."
export ELEVENLABS_API_KEY="..."

# Generate from a URL
podcaster generate -i https://example.com/article -o episode.mp3

# Generate from a PDF
podcaster generate -i paper.pdf -o episode.mp3

# Generate from a text file
podcaster generate -i notes.txt -o episode.mp3
```

## Build from Source

```bash
git clone https://github.com/apresai/podcaster.git
cd podcaster
make build       # Build to ./bin/podcaster
make install     # Install to $GOPATH/bin
```

## Requirements

- **Go 1.23+**
- **FFmpeg** — `brew install ffmpeg`
- **Anthropic API key** — [console.anthropic.com](https://console.anthropic.com/)
- **ElevenLabs API key** — [elevenlabs.io](https://elevenlabs.io/)

## Usage

### Basic Generation

```bash
podcaster generate -i <source> -o <output.mp3>
```

### Options

| Flag | Description | Default |
|------|-------------|---------|
| `-i, --input` | Source content (URL, PDF, or text file) | required |
| `-o, --output` | Output file path | required |
| `--topic` | Focus conversation on a specific topic | — |
| `--tone` | Conversation tone: `casual`, `technical`, `educational` | `casual` |
| `--duration` | Target length: `short` (~5min), `medium` (~10min), `long` (~20min) | `medium` |
| `--voice-alex` | ElevenLabs voice ID for host Alex | George |
| `--voice-sam` | ElevenLabs voice ID for analyst Sam | Sarah |
| `--script-only` | Output script JSON only, skip audio generation | `false` |
| `--from-script` | Generate audio from an existing script JSON file | — |
| `--verbose` | Show detailed logging per stage | `false` |

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
# Short casual episode from a blog post
podcaster generate -i https://example.com/blog -o ep.mp3 --duration short

# Technical deep-dive focused on security
podcaster generate -i paper.pdf -o ep.mp3 --tone technical --topic "security implications"

# Long educational episode with custom voices
podcaster generate -i textbook.txt -o ep.mp3 --duration long --tone educational \
  --voice-alex YOUR_VOICE_ID --voice-sam YOUR_VOICE_ID

# Verbose output for debugging
podcaster generate -i article.txt -o ep.mp3 --verbose
```

## How It Works

Four-stage pipeline:

1. **Ingest** — Extracts plain text from URL (via readability), PDF, or text file
2. **Script Gen** — Claude API generates a two-host dialogue as structured JSON
3. **TTS** — ElevenLabs converts each segment to speech (sequential, with retry)
4. **Assembly** — FFmpeg concatenates segments with 200ms silence gaps

## Hosts

| Host | Role | Style |
|------|------|-------|
| **Alex** | Host/driver | Introduces topics, provides context, uses analogies |
| **Sam** | Analyst/questioner | Asks probing questions, challenges assumptions, adds depth |

## Environment Variables

| Variable | Required | Purpose |
|----------|----------|---------|
| `ANTHROPIC_API_KEY` | Yes (unless `--from-script`) | Claude API for script generation |
| `ELEVENLABS_API_KEY` | Yes (unless `--script-only`) | ElevenLabs TTS |

## Cost

Typical cost per 10-minute episode: **< $1** (Claude API + ElevenLabs combined).

## Privacy

Source content is sent to Anthropic (script generation) and generated speech text is sent to ElevenLabs (TTS). No content is stored by this tool beyond the output file.

## License

MIT
