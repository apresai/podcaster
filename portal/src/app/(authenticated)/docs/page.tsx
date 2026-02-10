import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { Separator } from "@/components/ui/separator";

function Code({ children }: { children: React.ReactNode }) {
  return (
    <pre className="rounded-lg bg-muted p-4 overflow-x-auto text-sm font-mono">
      {children}
    </pre>
  );
}

function InlineCode({ children }: { children: React.ReactNode }) {
  return (
    <code className="rounded bg-muted px-1.5 py-0.5 text-sm font-mono">
      {children}
    </code>
  );
}

const MCP_ENDPOINT = "https://podcasts.apresai.dev/mcp";
const RUNTIME_ARN =
  "arn:aws:bedrock-agentcore:us-east-1:228029809749:runtime/podcaster_mcp-t01dg1G007";

export default function DocsPage() {
  return (
    <div className="space-y-8">
      <div>
        <h1 className="text-3xl font-bold">Documentation</h1>
        <p className="mt-1 text-muted-foreground">
          Learn how to use Podcaster with your AI assistant
        </p>
      </div>

      <Tabs defaultValue="getting-started">
        <TabsList>
          <TabsTrigger value="getting-started">Getting started</TabsTrigger>
          <TabsTrigger value="architecture">Architecture</TabsTrigger>
          <TabsTrigger value="tools">Tools reference</TabsTrigger>
          <TabsTrigger value="examples">Examples</TabsTrigger>
        </TabsList>

        <TabsContent value="getting-started" className="space-y-6 mt-6">
          <Card>
            <CardHeader>
              <CardTitle>Connect to Podcaster</CardTitle>
            </CardHeader>
            <CardContent className="space-y-4">
              <p className="text-sm text-muted-foreground">
                Connect any MCP client to Podcaster with just a URL and your API
                key. Get your API key from the{" "}
                <strong>API Keys</strong> page.
              </p>
              <Separator />

              <div className="space-y-2">
                <h3 className="font-semibold">Claude Code</h3>
                <Code>
                  {`claude mcp add podcaster ${MCP_ENDPOINT} \\
  --transport http \\
  --header "Authorization: Bearer pk_YOUR_API_KEY"`}
                </Code>
              </div>

              <Separator />

              <div className="space-y-2">
                <h3 className="font-semibold">Claude Desktop</h3>
                <p className="text-sm text-muted-foreground">
                  Add to your <InlineCode>claude_desktop_config.json</InlineCode>:
                </p>
                <Code>
                  {`{
  "mcpServers": {
    "podcaster": {
      "transport": "streamable-http",
      "url": "${MCP_ENDPOINT}",
      "headers": {
        "Authorization": "Bearer pk_YOUR_API_KEY"
      }
    }
  }
}`}
                </Code>
              </div>

              <Separator />

              <div className="space-y-2">
                <h3 className="font-semibold">curl</h3>
                <p className="text-sm text-muted-foreground">
                  Initialize an MCP session, then generate a podcast:
                </p>
                <Code>
                  {`# Initialize
curl -s ${MCP_ENDPOINT} \\
  -H "Authorization: Bearer pk_YOUR_API_KEY" \\
  -H "Content-Type: application/json" \\
  -d '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","clientInfo":{"name":"test"},"capabilities":{}}}'

# Generate a podcast
curl -s ${MCP_ENDPOINT} \\
  -H "Authorization: Bearer pk_YOUR_API_KEY" \\
  -H "Content-Type: application/json" \\
  -d '{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"generate_podcast","arguments":{"input_url":"https://example.com/article","duration":"short"}}}'`}
                </Code>
                <p className="text-sm text-muted-foreground">
                  Poll with <InlineCode>get_podcast</InlineCode> until{" "}
                  <InlineCode>status</InlineCode> is{" "}
                  <InlineCode>completed</InlineCode>, then use the{" "}
                  <InlineCode>audio_url</InlineCode>.
                </p>
              </div>

              <Separator />

              <details className="rounded-lg border p-4">
                <summary className="cursor-pointer font-semibold text-sm">
                  Advanced: Direct AgentCore Access (AWS CLI)
                </summary>
                <div className="mt-4 space-y-2">
                  <p className="text-sm text-muted-foreground">
                    For users with AWS credentials and{" "}
                    <InlineCode>
                      bedrock-agentcore:InvokeAgentRuntime
                    </InlineCode>{" "}
                    permission, you can bypass the proxy and invoke AgentCore
                    directly:
                  </p>
                  <Code>
                    {`aws bedrock-agentcore invoke-agent-runtime \\
  --agent-runtime-arn "${RUNTIME_ARN}" \\
  --region us-east-1 \\
  --cli-binary-format raw-in-base64-out \\
  --accept "application/json, text/event-stream" \\
  --payload '{
    "jsonrpc": "2.0",
    "id": 1,
    "method": "initialize",
    "params": {
      "protocolVersion": "2024-11-05",
      "clientInfo": {"name": "test"},
      "capabilities": {}
    }
  }' /tmp/init.json`}
                  </Code>
                </div>
              </details>
            </CardContent>
          </Card>
        </TabsContent>

        <TabsContent value="architecture" className="space-y-6 mt-6">
          <Card>
            <CardHeader>
              <CardTitle>How it works</CardTitle>
            </CardHeader>
            <CardContent className="space-y-4">
              <p className="text-sm text-muted-foreground">
                Podcaster runs as a remote MCP server on{" "}
                <strong>AWS Bedrock AgentCore</strong>. A thin Lambda proxy at{" "}
                <InlineCode>{MCP_ENDPOINT}</InlineCode> handles API key
                validation and forwards requests to AgentCore.
              </p>
              <div className="rounded-lg bg-muted p-4 text-sm font-mono">
                MCP Client &rarr; CloudFront &rarr; Lambda proxy (API key auth)
                &rarr; AgentCore &rarr; MCP Server (container)
              </div>
              <Separator />
              <div className="space-y-2">
                <h4 className="font-semibold text-sm">Key details</h4>
                <ul className="list-disc list-inside text-sm text-muted-foreground space-y-1">
                  <li>
                    <strong>Auth</strong>: API key (
                    <InlineCode>Authorization: Bearer pk_...</InlineCode>)
                  </li>
                  <li>
                    <strong>Protocol</strong>: MCP over StreamableHTTP (JSON-RPC
                    2.0)
                  </li>
                  <li>
                    <strong>Endpoint</strong>:{" "}
                    <InlineCode>{MCP_ENDPOINT}</InlineCode>
                  </li>
                  <li>
                    <strong>Audio CDN</strong>:{" "}
                    <InlineCode>
                      https://podcasts.apresai.dev/audio/...
                    </InlineCode>{" "}
                    (CloudFront &rarr; S3)
                  </li>
                </ul>
              </div>
              <Separator />
              <div className="space-y-2">
                <h4 className="font-semibold text-sm">
                  Pipeline (what happens when you generate)
                </h4>
                <div className="rounded-lg bg-muted p-4 text-sm font-mono">
                  Input &rarr; [Ingest] &rarr; plain text &rarr; [Script Gen]
                  &rarr; JSON segments &rarr; [TTS] &rarr; MP3 files &rarr;
                  [Assembly] &rarr; final MP3
                </div>
                <p className="text-sm text-muted-foreground">
                  Generation takes 3-8 minutes depending on duration. The MP3 is
                  uploaded to S3 and served via CloudFront. The script JSON is
                  also uploaded alongside it.
                </p>
              </div>
            </CardContent>
          </Card>
        </TabsContent>

        <TabsContent value="tools" className="space-y-6 mt-6">
          <Card>
            <CardHeader>
              <CardTitle>generate_podcast</CardTitle>
            </CardHeader>
            <CardContent className="space-y-3">
              <p className="text-sm text-muted-foreground">
                Start generating a podcast from a URL or text content. Returns
                immediately with a podcast ID for polling.
              </p>
              <h4 className="font-semibold text-sm">Parameters</h4>
              <div className="rounded-lg border">
                <table className="w-full text-sm">
                  <thead>
                    <tr className="border-b bg-muted/50">
                      <th className="px-4 py-2 text-left font-medium">
                        Name
                      </th>
                      <th className="px-4 py-2 text-left font-medium">
                        Type
                      </th>
                      <th className="px-4 py-2 text-left font-medium">
                        Required
                      </th>
                      <th className="px-4 py-2 text-left font-medium">
                        Description
                      </th>
                    </tr>
                  </thead>
                  <tbody>
                    <tr className="border-b">
                      <td className="px-4 py-2 font-mono">input_url</td>
                      <td className="px-4 py-2">string</td>
                      <td className="px-4 py-2">Yes*</td>
                      <td className="px-4 py-2">
                        URL of content to convert
                      </td>
                    </tr>
                    <tr className="border-b">
                      <td className="px-4 py-2 font-mono">input_text</td>
                      <td className="px-4 py-2">string</td>
                      <td className="px-4 py-2">Yes*</td>
                      <td className="px-4 py-2">
                        Raw text content to convert
                      </td>
                    </tr>
                    <tr className="border-b">
                      <td className="px-4 py-2 font-mono">model</td>
                      <td className="px-4 py-2">string</td>
                      <td className="px-4 py-2">No</td>
                      <td className="px-4 py-2">
                        Script generation LLM (writes the conversation): haiku
                        (default, Claude Haiku 4.5), sonnet, gemini-flash,
                        gemini-pro
                      </td>
                    </tr>
                    <tr className="border-b">
                      <td className="px-4 py-2 font-mono">tts</td>
                      <td className="px-4 py-2">string</td>
                      <td className="px-4 py-2">No</td>
                      <td className="px-4 py-2">
                        Text-to-speech provider (synthesizes audio): gemini
                        (default), gemini-vertex, vertex-express, elevenlabs,
                        google
                      </td>
                    </tr>
                    <tr className="border-b">
                      <td className="px-4 py-2 font-mono">duration</td>
                      <td className="px-4 py-2">string</td>
                      <td className="px-4 py-2">No</td>
                      <td className="px-4 py-2">
                        short (~8min), standard (~18min), long (~35min), deep
                        (~55min)
                      </td>
                    </tr>
                    <tr className="border-b">
                      <td className="px-4 py-2 font-mono">format</td>
                      <td className="px-4 py-2">string</td>
                      <td className="px-4 py-2">No</td>
                      <td className="px-4 py-2">
                        conversation, interview, deep-dive, explainer, debate,
                        news, storytelling, challenger
                      </td>
                    </tr>
                    <tr className="border-b">
                      <td className="px-4 py-2 font-mono">topic</td>
                      <td className="px-4 py-2">string</td>
                      <td className="px-4 py-2">No</td>
                      <td className="px-4 py-2">
                        Focus topic for the podcast
                      </td>
                    </tr>
                    <tr>
                      <td className="px-4 py-2 font-mono">tone</td>
                      <td className="px-4 py-2">string</td>
                      <td className="px-4 py-2">No</td>
                      <td className="px-4 py-2">
                        Tone of the conversation (e.g. casual, technical)
                      </td>
                    </tr>
                  </tbody>
                </table>
              </div>
              <p className="text-xs text-muted-foreground">
                * Either input_url or input_text is required
              </p>
            </CardContent>
          </Card>

          <Card>
            <CardHeader>
              <CardTitle>get_podcast</CardTitle>
            </CardHeader>
            <CardContent className="space-y-3">
              <p className="text-sm text-muted-foreground">
                Get the status and details of a podcast by its ID. Use this to
                poll for completion after calling generate_podcast. Completed
                podcasts include <InlineCode>audio_url</InlineCode> and{" "}
                <InlineCode>script_url</InlineCode> fields.
              </p>
              <h4 className="font-semibold text-sm">Parameters</h4>
              <div className="rounded-lg border">
                <table className="w-full text-sm">
                  <thead>
                    <tr className="border-b bg-muted/50">
                      <th className="px-4 py-2 text-left font-medium">
                        Name
                      </th>
                      <th className="px-4 py-2 text-left font-medium">
                        Type
                      </th>
                      <th className="px-4 py-2 text-left font-medium">
                        Required
                      </th>
                      <th className="px-4 py-2 text-left font-medium">
                        Description
                      </th>
                    </tr>
                  </thead>
                  <tbody>
                    <tr>
                      <td className="px-4 py-2 font-mono">podcast_id</td>
                      <td className="px-4 py-2">string</td>
                      <td className="px-4 py-2">Yes</td>
                      <td className="px-4 py-2">
                        The podcast ID returned by generate_podcast
                      </td>
                    </tr>
                  </tbody>
                </table>
              </div>
            </CardContent>
          </Card>

          <Card>
            <CardHeader>
              <CardTitle>list_podcasts</CardTitle>
            </CardHeader>
            <CardContent className="space-y-3">
              <p className="text-sm text-muted-foreground">
                List your recent podcasts, newest first.
              </p>
              <h4 className="font-semibold text-sm">Parameters</h4>
              <div className="rounded-lg border">
                <table className="w-full text-sm">
                  <thead>
                    <tr className="border-b bg-muted/50">
                      <th className="px-4 py-2 text-left font-medium">
                        Name
                      </th>
                      <th className="px-4 py-2 text-left font-medium">
                        Type
                      </th>
                      <th className="px-4 py-2 text-left font-medium">
                        Required
                      </th>
                      <th className="px-4 py-2 text-left font-medium">
                        Description
                      </th>
                    </tr>
                  </thead>
                  <tbody>
                    <tr className="border-b">
                      <td className="px-4 py-2 font-mono">limit</td>
                      <td className="px-4 py-2">number</td>
                      <td className="px-4 py-2">No</td>
                      <td className="px-4 py-2">
                        Max results to return (default 20)
                      </td>
                    </tr>
                    <tr>
                      <td className="px-4 py-2 font-mono">cursor</td>
                      <td className="px-4 py-2">string</td>
                      <td className="px-4 py-2">No</td>
                      <td className="px-4 py-2">
                        Pagination cursor from previous response
                      </td>
                    </tr>
                  </tbody>
                </table>
              </div>
            </CardContent>
          </Card>
        </TabsContent>

        <TabsContent value="examples" className="space-y-6 mt-6">
          <Card>
            <CardHeader>
              <CardTitle>API key examples (curl)</CardTitle>
            </CardHeader>
            <CardContent className="space-y-6">
              <p className="text-sm text-muted-foreground">
                All requests use your API key in the{" "}
                <InlineCode>Authorization</InlineCode> header. Replace{" "}
                <InlineCode>pk_YOUR_API_KEY</InlineCode> with your key from the
                API Keys page.
              </p>
              <Separator />
              <div className="space-y-2">
                <h4 className="font-semibold text-sm">
                  Initialize MCP session
                </h4>
                <Code>
                  {`curl -s ${MCP_ENDPOINT} \\
  -H "Authorization: Bearer pk_YOUR_API_KEY" \\
  -H "Content-Type: application/json" \\
  -d '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","clientInfo":{"name":"test"},"capabilities":{}}}'`}
                </Code>
              </div>
              <Separator />
              <div className="space-y-2">
                <h4 className="font-semibold text-sm">Generate a podcast</h4>
                <Code>
                  {`curl -s ${MCP_ENDPOINT} \\
  -H "Authorization: Bearer pk_YOUR_API_KEY" \\
  -H "Content-Type: application/json" \\
  -d '{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"generate_podcast","arguments":{"input_url":"https://example.com/article","duration":"short"}}}'`}
                </Code>
              </div>
              <Separator />
              <div className="space-y-2">
                <h4 className="font-semibold text-sm">
                  Check podcast status
                </h4>
                <Code>
                  {`curl -s ${MCP_ENDPOINT} \\
  -H "Authorization: Bearer pk_YOUR_API_KEY" \\
  -H "Content-Type: application/json" \\
  -d '{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"get_podcast","arguments":{"podcast_id":"PODCAST_ID"}}}'`}
                </Code>
                <p className="text-sm text-muted-foreground">
                  Poll every 10-15 seconds. When{" "}
                  <InlineCode>status</InlineCode> is{" "}
                  <InlineCode>completed</InlineCode>, the response includes{" "}
                  <InlineCode>audio_url</InlineCode> and{" "}
                  <InlineCode>script_url</InlineCode>.
                </p>
              </div>
              <Separator />
              <div className="space-y-2">
                <h4 className="font-semibold text-sm">List your podcasts</h4>
                <Code>
                  {`curl -s ${MCP_ENDPOINT} \\
  -H "Authorization: Bearer pk_YOUR_API_KEY" \\
  -H "Content-Type: application/json" \\
  -d '{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"list_podcasts","arguments":{"limit":5}}}'`}
                </Code>
              </div>
            </CardContent>
          </Card>

          <Card>
            <CardHeader>
              <CardTitle>Advanced: Direct AgentCore Access (AWS CLI)</CardTitle>
            </CardHeader>
            <CardContent className="space-y-6">
              <p className="text-sm text-muted-foreground">
                For users with AWS credentials and{" "}
                <InlineCode>
                  bedrock-agentcore:InvokeAgentRuntime
                </InlineCode>{" "}
                permission, you can bypass the proxy and invoke AgentCore
                directly via the AWS CLI.
              </p>
              <Separator />
              <div className="space-y-2">
                <h4 className="font-semibold text-sm">
                  Initialize MCP session
                </h4>
                <Code>
                  {`aws bedrock-agentcore invoke-agent-runtime \\
  --agent-runtime-arn "${RUNTIME_ARN}" \\
  --region us-east-1 \\
  --cli-binary-format raw-in-base64-out \\
  --accept "application/json, text/event-stream" \\
  --payload '{
    "jsonrpc": "2.0",
    "id": 1,
    "method": "initialize",
    "params": {
      "protocolVersion": "2024-11-05",
      "clientInfo": {"name": "test"},
      "capabilities": {}
    }
  }' /tmp/init.json && cat /tmp/init.json | python3 -m json.tool`}
                </Code>
              </div>
              <Separator />
              <div className="space-y-2">
                <h4 className="font-semibold text-sm">Generate a podcast</h4>
                <Code>
                  {`aws bedrock-agentcore invoke-agent-runtime \\
  --agent-runtime-arn "${RUNTIME_ARN}" \\
  --region us-east-1 \\
  --cli-binary-format raw-in-base64-out \\
  --accept "application/json, text/event-stream" \\
  --payload '{
    "jsonrpc": "2.0",
    "id": 2,
    "method": "tools/call",
    "params": {
      "name": "generate_podcast",
      "arguments": {
        "input_url": "https://example.com/article",
        "duration": "short"
      }
    }
  }' /tmp/generate.json && cat /tmp/generate.json | python3 -m json.tool`}
                </Code>
              </div>
              <Separator />
              <div className="space-y-2">
                <h4 className="font-semibold text-sm">
                  Check podcast status
                </h4>
                <Code>
                  {`aws bedrock-agentcore invoke-agent-runtime \\
  --agent-runtime-arn "${RUNTIME_ARN}" \\
  --region us-east-1 \\
  --cli-binary-format raw-in-base64-out \\
  --accept "application/json, text/event-stream" \\
  --payload '{
    "jsonrpc": "2.0",
    "id": 3,
    "method": "tools/call",
    "params": {
      "name": "get_podcast",
      "arguments": {
        "podcast_id": "PODCAST_ID"
      }
    }
  }' /tmp/status.json && cat /tmp/status.json | python3 -m json.tool`}
                </Code>
                <p className="text-sm text-muted-foreground">
                  Poll every 10-15 seconds. When{" "}
                  <InlineCode>status</InlineCode> is{" "}
                  <InlineCode>completed</InlineCode>, the response includes{" "}
                  <InlineCode>audio_url</InlineCode> and{" "}
                  <InlineCode>script_url</InlineCode>.
                </p>
              </div>
            </CardContent>
          </Card>
        </TabsContent>
      </Tabs>
    </div>
  );
}
