const GATEWAY_URL = process.env.GATEWAY_URL || "";

interface MCPResponse {
  jsonrpc: string;
  id: number;
  result?: { content: Array<{ type: string; text: string }> };
  error?: { code: number; message: string };
}

async function mcpRequest(
  apiKey: string,
  body: Record<string, unknown>,
  sessionId?: string,
): Promise<{ data: MCPResponse; sessionId?: string }> {
  const headers: Record<string, string> = {
    "Content-Type": "application/json",
    Authorization: `Bearer ${apiKey}`,
  };
  if (sessionId) {
    headers["Mcp-Session-Id"] = sessionId;
  }

  const res = await fetch(GATEWAY_URL, {
    method: "POST",
    headers,
    body: JSON.stringify(body),
  });

  if (!res.ok) {
    const text = await res.text();
    throw new Error(`MCP proxy error (${res.status}): ${text}`);
  }

  const data = (await res.json()) as MCPResponse;
  const newSessionId = res.headers.get("mcp-session-id") || sessionId;
  return { data, sessionId: newSessionId || undefined };
}

async function initializeSession(
  apiKey: string,
): Promise<string | undefined> {
  const { sessionId } = await mcpRequest(apiKey, {
    jsonrpc: "2.0",
    id: 1,
    method: "initialize",
    params: {
      protocolVersion: "2024-11-05",
      clientInfo: { name: "podcaster-portal" },
      capabilities: {},
    },
  });
  return sessionId;
}

export async function callMCPTool(
  apiKey: string,
  toolName: string,
  args: Record<string, unknown>,
): Promise<unknown> {
  // Initialize a session, then call the tool
  const sessionId = await initializeSession(apiKey);

  const { data } = await mcpRequest(
    apiKey,
    {
      jsonrpc: "2.0",
      id: 2,
      method: "tools/call",
      params: { name: toolName, arguments: args },
    },
    sessionId,
  );

  if (data.error) {
    throw new Error(data.error.message || "MCP tool call failed");
  }

  // Parse the text content from the response
  const textContent = data.result?.content?.find((c) => c.type === "text");
  if (!textContent?.text) {
    return null;
  }

  try {
    return JSON.parse(textContent.text);
  } catch {
    return textContent.text;
  }
}
