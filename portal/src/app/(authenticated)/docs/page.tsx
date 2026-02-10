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

export default function DocsPage() {
  const gatewayUrl =
    process.env.GATEWAY_URL ||
    "https://i656dtw3u7brkptuw2uejmzy6i0dtmii.lambda-url.us-east-1.on.aws";

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
          <TabsTrigger value="tools">Tools reference</TabsTrigger>
          <TabsTrigger value="examples">Examples</TabsTrigger>
        </TabsList>

        <TabsContent value="getting-started" className="space-y-6 mt-6">
          <Card>
            <CardHeader>
              <CardTitle>Quick start</CardTitle>
            </CardHeader>
            <CardContent className="space-y-4">
              <div className="space-y-2">
                <h3 className="font-semibold">1. Get your API key</h3>
                <p className="text-sm text-muted-foreground">
                  Go to the{" "}
                  <a href="/api-keys" className="text-primary hover:underline">
                    API Keys
                  </a>{" "}
                  page and create a new key. Copy the full key — it is shown
                  only once.
                </p>
              </div>
              <Separator />
              <div className="space-y-2">
                <h3 className="font-semibold">
                  2. Configure Claude Desktop
                </h3>
                <p className="text-sm text-muted-foreground">
                  Add the following to your Claude Desktop MCP configuration
                  file:
                </p>
                <Code>
                  {JSON.stringify(
                    {
                      mcpServers: {
                        podcaster: {
                          type: "url",
                          url: `${gatewayUrl}/mcp`,
                          headers: {
                            Authorization: "Bearer pk_YOUR_API_KEY_HERE",
                          },
                        },
                      },
                    },
                    null,
                    2
                  )}
                </Code>
                <p className="text-sm text-muted-foreground">
                  Replace <InlineCode>pk_YOUR_API_KEY_HERE</InlineCode> with
                  your actual API key.
                </p>
              </div>
              <Separator />
              <div className="space-y-2">
                <h3 className="font-semibold">3. Generate a podcast</h3>
                <p className="text-sm text-muted-foreground">
                  Ask Claude to generate a podcast. For example:
                </p>
                <Code>
                  {`"Generate a podcast from https://example.com/article"`}
                </Code>
                <p className="text-sm text-muted-foreground">
                  Claude will call the <InlineCode>generate_podcast</InlineCode>{" "}
                  tool, then poll <InlineCode>get_podcast</InlineCode> until the
                  episode is ready.
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
                        (default), sonnet, gemini-flash, gemini-pro
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
                        Max results to return (default 10)
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
              <CardTitle>curl examples</CardTitle>
            </CardHeader>
            <CardContent className="space-y-6">
              <div className="space-y-2">
                <h4 className="font-semibold text-sm">
                  Initialize MCP session
                </h4>
                <Code>
                  {`curl -s ${gatewayUrl}/mcp \\
  -H 'Content-Type: application/json' \\
  -H 'Authorization: Bearer pk_YOUR_API_KEY' \\
  -d '{
    "jsonrpc": "2.0",
    "id": 1,
    "method": "initialize",
    "params": {
      "protocolVersion": "2024-11-05",
      "clientInfo": {"name": "test"},
      "capabilities": {}
    }
  }'`}
                </Code>
              </div>
              <Separator />
              <div className="space-y-2">
                <h4 className="font-semibold text-sm">Generate a podcast</h4>
                <Code>
                  {`curl -s ${gatewayUrl}/mcp \\
  -H 'Content-Type: application/json' \\
  -H 'Authorization: Bearer pk_YOUR_API_KEY' \\
  -H 'Mcp-Session-Id: SESSION_ID_FROM_INIT' \\
  -d '{
    "jsonrpc": "2.0",
    "id": 2,
    "method": "tools/call",
    "params": {
      "name": "generate_podcast",
      "arguments": {
        "input_url": "https://example.com/article",
        "model": "gemini-flash",
        "duration": "short"
      }
    }
  }'`}
                </Code>
              </div>
              <Separator />
              <div className="space-y-2">
                <h4 className="font-semibold text-sm">
                  Check podcast status
                </h4>
                <Code>
                  {`curl -s ${gatewayUrl}/mcp \\
  -H 'Content-Type: application/json' \\
  -H 'Authorization: Bearer pk_YOUR_API_KEY' \\
  -H 'Mcp-Session-Id: SESSION_ID' \\
  -d '{
    "jsonrpc": "2.0",
    "id": 3,
    "method": "tools/call",
    "params": {
      "name": "get_podcast",
      "arguments": {
        "podcast_id": "PODCAST_ID"
      }
    }
  }'`}
                </Code>
              </div>
            </CardContent>
          </Card>
        </TabsContent>
      </Tabs>
    </div>
  );
}
