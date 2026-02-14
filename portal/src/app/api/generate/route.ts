import { NextResponse } from "next/server";
import { auth, canCreate } from "@/lib/auth";
import { getActiveAPIKeyForUser } from "@/lib/db";
import { callMCPTool } from "@/lib/mcp";

export async function POST(request: Request) {
  const session = await auth();
  if (!session?.user?.id) {
    return NextResponse.json({ error: "Unauthorized" }, { status: 401 });
  }
  if (!canCreate(session.user.role)) {
    return NextResponse.json({ error: "Creator access required" }, { status: 403 });
  }

  const apiKey = await getActiveAPIKeyForUser(session.user.id);
  if (!apiKey) {
    return NextResponse.json(
      { error: "No active API key. Create one on the API Keys page." },
      { status: 400 }
    );
  }

  const body = await request.json();
  if (!body.input_url && !body.input_text) {
    return NextResponse.json(
      { error: "Either input_url or input_text is required" },
      { status: 400 }
    );
  }

  // Filter out empty/null values
  const args: Record<string, unknown> = {};
  for (const [key, value] of Object.entries(body)) {
    if (value !== "" && value !== null && value !== undefined) {
      args[key] = value;
    }
  }

  try {
    const result = await callMCPTool(apiKey, "generate_podcast", args);
    return NextResponse.json(result);
  } catch (e) {
    const message = e instanceof Error ? e.message : "Unknown error";
    console.error("[api/generate] Error:", message);
    return NextResponse.json({ error: message }, { status: 502 });
  }
}
