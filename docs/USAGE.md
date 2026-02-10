# Podcaster MCP Server Usage Guide

Turn any article, blog post, or text into a multi-host podcast episode using the Podcaster MCP server.

## Quick Start

Connect any MCP client to Podcaster with just a URL and an API key. Get your API key at [podcasts.apresai.dev](https://podcasts.apresai.dev).

### Claude Code

```bash
claude mcp add podcaster https://podcasts.apresai.dev/mcp \
  --transport http \
  --header "Authorization: Bearer pk_YOUR_API_KEY"
```

### Claude Desktop

Add to your `claude_desktop_config.json`:

```json
{
  "mcpServers": {
    "podcaster": {
      "transport": "streamable-http",
      "url": "https://podcasts.apresai.dev/mcp",
      "headers": { "Authorization": "Bearer pk_YOUR_API_KEY" }
    }
  }
}
```

### curl

```bash
# Initialize
curl -s https://podcasts.apresai.dev/mcp \
  -H "Authorization: Bearer pk_YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","clientInfo":{"name":"test"},"capabilities":{}}}'

# Generate a podcast
curl -s https://podcasts.apresai.dev/mcp \
  -H "Authorization: Bearer pk_YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"generate_podcast","arguments":{"input_url":"https://example.com/article","duration":"short"}}}'

# Check status (replace PODCAST_ID)
curl -s https://podcasts.apresai.dev/mcp \
  -H "Authorization: Bearer pk_YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"get_podcast","arguments":{"podcast_id":"PODCAST_ID"}}}'
```

## Architecture

```
MCP Client -> POST https://podcasts.apresai.dev/mcp (Bearer pk_...)
  -> CloudFront
  -> Lambda proxy (API key validation)
  -> AgentCore invoke-agent-runtime (SigV4)
  -> MCP Server (container)
```

- **Protocol**: MCP over StreamableHTTP (JSON-RPC 2.0)
- **Auth**: API key (`Authorization: Bearer pk_...`)
- **Endpoint**: `https://podcasts.apresai.dev/mcp`
- **Audio CDN**: `https://podcasts.apresai.dev/audio/...` (CloudFront -> S3)

## Available Tools

### generate_podcast

Generates a podcast episode from a URL or text input. The task runs asynchronously -- it returns a `podcast_id` immediately, and you poll with `get_podcast` to check progress.

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `input_url` | string | -- | URL of content to convert into a podcast |
| `input_text` | string | -- | Raw text to convert (alternative to `input_url`) |
| `model` | string | `"haiku"` | Script generation LLM (writes the conversation): `haiku` (default, Claude Haiku 4.5), `sonnet`, `gemini-flash`, `gemini-pro` |
| `tts` | string | `"gemini"` | TTS provider (synthesizes audio): `gemini` (default), `gemini-vertex`, `vertex-express`, `elevenlabs`, `google` |
| `tone` | string | `"casual"` | Conversation tone: `casual`, `technical`, `educational` |
| `duration` | string | `"standard"` | Episode length: `short` (~8min), `standard` (~18min), `long` (~35min), `deep` (~55min) |
| `format` | string | `"conversation"` | Show format (see below) |
| `voices` | integer | `2` | Number of hosts (1-3) |
| `topic` | string | -- | Focus topic to emphasize in the conversation |

Either `input_url` or `input_text` is required.

**Show formats:**

| Format | Description |
|--------|-------------|
| `conversation` | Casual back-and-forth discussion (default) |
| `interview` | One host interviews the other |
| `deep-dive` | Thorough technical exploration |
| `explainer` | Break down a complex topic for a general audience |
| `debate` | Hosts take opposing perspectives |
| `news` | News-style reporting and analysis |
| `storytelling` | Narrative-driven episode |
| `challenger` | One host challenges the other's assumptions |

### get_podcast

Check the status of a podcast and retrieve details when complete.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `podcast_id` | string | Yes | The podcast ID returned from `generate_podcast` |

**Response fields:**

| Field | Description |
|-------|-------------|
| `status` | `submitted`, `processing`, `completed`, or `failed` |
| `progress_percent` | 0-100 progress indicator |
| `stage_message` | Current processing stage description |
| `audio_url` | Direct MP3 link (available when `completed`) |
| `script_url` | Script JSON link (available when `completed`) |
| `title` | Generated episode title |
| `summary` | Brief episode summary |
| `duration` | Episode duration |
| `file_size_mb` | MP3 file size |

### list_podcasts

List your generated podcasts, newest first.

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `limit` | integer | `20` | Maximum number of results |
| `cursor` | string | -- | Pagination cursor from a previous `list_podcasts` call |

### list_voices

List available voices for a TTS provider.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `provider` | string | Yes | TTS provider name (e.g. `gemini`, `elevenlabs`) |

### list_options

List all available formats, styles, TTS providers, models, and durations. No parameters required.

## Pricing Estimates

Costs depend on the script generation model and TTS provider used:

| Model | TTS | Estimated Cost |
|-------|-----|----------------|
| `gemini-flash` | `gemini` | ~$0.001-0.01 |
| `haiku` | `gemini` | ~$0.01-0.05 |
| `gemini-pro` | `gemini` | ~$0.02-0.15 |
| `sonnet` | `gemini` | ~$0.05-0.20 |
| Any model | `elevenlabs` | Add ~$0.50-2.00 for TTS |

Costs scale with episode duration. The `short` duration is the most economical for testing.

## Troubleshooting

**Podcast stuck in `processing`**
Generation typically takes 2-5 minutes for `short` episodes and up to 15 minutes for `deep` episodes. If a podcast remains in `processing` for more than 20 minutes, it likely encountered an internal error -- try generating again.

**URL content extraction fails**
Some websites block automated content extraction (e.g., sites behind Cloudflare bot protection). Try providing the content directly via `input_text` instead.

**Supported input formats**
- Web URLs (articles, blog posts, documentation)
- Plain text
- PDF files (via URL)

**Rate limits**
One concurrent podcast generation per account. Wait for the current generation to complete before starting another.

## Advanced: Direct AgentCore Access

For users with AWS credentials and `bedrock-agentcore:InvokeAgentRuntime` permission, you can bypass the proxy and invoke AgentCore directly:

```bash
RUNTIME_ARN="arn:aws:bedrock-agentcore:us-east-1:228029809749:runtime/podcaster_mcp-t01dg1G007"

aws bedrock-agentcore invoke-agent-runtime \
  --agent-runtime-arn "$RUNTIME_ARN" \
  --region us-east-1 \
  --cli-binary-format raw-in-base64-out \
  --accept "application/json, text/event-stream" \
  --payload '{
    "jsonrpc": "2.0",
    "id": 1,
    "method": "initialize",
    "params": {
      "protocolVersion": "2024-11-05",
      "clientInfo": {"name": "test"},
      "capabilities": {}
    }
  }' /tmp/init.json
```
