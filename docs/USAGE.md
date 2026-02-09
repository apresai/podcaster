# Podcaster MCP Server Usage Guide

Turn any article, blog post, or text into a multi-host podcast episode using the Podcaster MCP server.

## Getting Started

1. **Sign up** at [podcasts.apresai.dev](https://podcasts.apresai.dev)
2. **Wait for approval** -- accounts are admin-approved to manage usage and costs
3. **Create an API key** from the dashboard after your account is approved

## Claude Desktop Setup

Add the following to your Claude Desktop MCP configuration file (`~/Library/Application Support/Claude/claude_desktop_config.json` on macOS):

```json
{
  "mcpServers": {
    "podcaster": {
      "type": "streamableHttp",
      "url": "https://i656dtw3u7brkptuw2uejmzy6i0dtmii.lambda-url.us-east-1.on.aws",
      "headers": {
        "Authorization": "Bearer pk_your_api_key_here"
      }
    }
  }
}
```

Replace `pk_your_api_key_here` with the API key from your dashboard. Restart Claude Desktop after saving.

## Available Tools

### generate_podcast

Generates a podcast episode from a URL or text input. The task runs asynchronously -- it returns a `podcast_id` immediately, and you poll with `get_podcast` to check progress.

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `input_url` | string | -- | URL of content to convert into a podcast |
| `input_text` | string | -- | Raw text to convert (alternative to `input_url`) |
| `model` | string | `"haiku"` | Script generation model: `haiku`, `sonnet`, `gemini-flash`, `gemini-pro` |
| `tts` | string | `"gemini"` | TTS provider: `gemini`, `elevenlabs`, `google` |
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

## Developer Quickstart

You can interact with the MCP server directly using curl. The server uses the MCP StreamableHTTP protocol with JSON-RPC 2.0.

**Step 1: Initialize the session**

```bash
curl -s https://i656dtw3u7brkptuw2uejmzy6i0dtmii.lambda-url.us-east-1.on.aws \
  -H 'Content-Type: application/json' \
  -H 'Authorization: Bearer pk_your_api_key_here' \
  -d '{
    "jsonrpc": "2.0",
    "id": 1,
    "method": "initialize",
    "params": {
      "protocolVersion": "2024-11-05",
      "clientInfo": {"name": "my-app"},
      "capabilities": {}
    }
  }'
```

Save the `Mcp-Session-Id` response header for subsequent requests.

**Step 2: Generate a podcast**

```bash
curl -s https://i656dtw3u7brkptuw2uejmzy6i0dtmii.lambda-url.us-east-1.on.aws \
  -H 'Content-Type: application/json' \
  -H 'Authorization: Bearer pk_your_api_key_here' \
  -H 'Mcp-Session-Id: SESSION_ID_FROM_STEP_1' \
  -d '{
    "jsonrpc": "2.0",
    "id": 2,
    "method": "tools/call",
    "params": {
      "name": "generate_podcast",
      "arguments": {
        "input_url": "https://example.com/article",
        "duration": "short",
        "format": "conversation"
      }
    }
  }'
```

The response includes a `podcast_id`.

**Step 3: Poll for completion**

```bash
curl -s https://i656dtw3u7brkptuw2uejmzy6i0dtmii.lambda-url.us-east-1.on.aws \
  -H 'Content-Type: application/json' \
  -H 'Authorization: Bearer pk_your_api_key_here' \
  -H 'Mcp-Session-Id: SESSION_ID_FROM_STEP_1' \
  -d '{
    "jsonrpc": "2.0",
    "id": 3,
    "method": "tools/call",
    "params": {
      "name": "get_podcast",
      "arguments": {
        "podcast_id": "PODCAST_ID_FROM_STEP_2"
      }
    }
  }'
```

Poll every 10-15 seconds. When `status` is `completed`, the response includes an `audio_url` with a direct link to the MP3 file.

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

**"Authentication required" error**
Ensure your API key is included in the `Authorization` header as `Bearer pk_...`. Verify the key is active in your dashboard.

**"either input_url or input_text is required"**
You must provide one of `input_url` or `input_text` to the `generate_podcast` tool.

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
