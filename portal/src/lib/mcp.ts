const GATEWAY_URL = process.env.GATEWAY_URL;

interface MCPResponse {
  jsonrpc: string;
  id: number;
  result?: {
    content?: Array<{ type: string; text: string }>;
    [key: string]: unknown;
  };
  error?: { code: number; message: string };
}

export async function callMCPTool(
  apiKey: string,
  toolName: string,
  args: Record<string, unknown>
): Promise<unknown> {
  if (!GATEWAY_URL) {
    throw new Error("GATEWAY_URL not configured");
  }

  const headers = {
    "Content-Type": "application/json",
    Authorization: `Bearer ${apiKey}`,
  };

  // 1. Initialize MCP session
  console.log(`[mcp] initialize → ${GATEWAY_URL}`);
  const initRes = await fetch(GATEWAY_URL, {
    method: "POST",
    headers,
    body: JSON.stringify({
      jsonrpc: "2.0",
      id: 1,
      method: "initialize",
      params: {
        protocolVersion: "2024-11-05",
        clientInfo: { name: "podcaster-portal" },
        capabilities: {},
      },
    }),
  });

  if (!initRes.ok) {
    const body = await initRes.text().catch(() => "");
    console.error(`[mcp] initialize failed: ${initRes.status}`, body);
    throw new Error(`MCP initialize failed: ${initRes.status} — ${body}`);
  }

  const initData = await initRes.json();
  console.log(`[mcp] initialize OK, sessionId=${initData.sessionId || "none"}`);

  const sessionId =
    initRes.headers.get("Mcp-Session-Id") || initData.sessionId;
  const toolHeaders: Record<string, string> = { ...headers };
  if (sessionId) {
    toolHeaders["Mcp-Session-Id"] = sessionId;
  }

  // 2. Call the tool
  console.log(`[mcp] tools/call → ${toolName}`, JSON.stringify(args));
  const toolRes = await fetch(GATEWAY_URL, {
    method: "POST",
    headers: toolHeaders,
    body: JSON.stringify({
      jsonrpc: "2.0",
      id: 2,
      method: "tools/call",
      params: {
        name: toolName,
        arguments: args,
      },
    }),
  });

  if (!toolRes.ok) {
    const body = await toolRes.text().catch(() => "");
    console.error(`[mcp] tools/call failed: ${toolRes.status}`, body);
    throw new Error(`MCP tools/call failed: ${toolRes.status} — ${body}`);
  }

  const data: MCPResponse = await toolRes.json();
  console.log(`[mcp] tools/call response:`, JSON.stringify(data).slice(0, 500));

  if (data.error) {
    throw new Error(`MCP error: ${data.error.message}`);
  }

  // Extract the text content and parse as JSON
  const textContent = data.result?.content?.find((c) => c.type === "text");
  if (!textContent?.text) {
    return data.result;
  }

  try {
    return JSON.parse(textContent.text);
  } catch {
    return textContent.text;
  }
}
