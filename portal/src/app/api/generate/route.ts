import { NextResponse } from "next/server";
import { auth } from "@/lib/auth";
import { getActiveAPIKeyForUser } from "@/lib/db";
import { callMCPTool } from "@/lib/mcp";

export async function POST(request: Request) {
  const session = await auth();
  if (!session?.user?.id) {
    return NextResponse.json({ error: "Unauthorized" }, { status: 401 });
  }

  const apiKey = await getActiveAPIKeyForUser(session.user.id);
  if (!apiKey) {
    return NextResponse.json(
      { error: "No API key found. Create one in API Keys first." },
      { status: 400 },
    );
  }

  const body = await request.json();
  if (!body.input_url && !body.input_text) {
    return NextResponse.json(
      { error: "Either input_url or input_text is required" },
      { status: 400 },
    );
  }

  try {
    const result = (await callMCPTool(apiKey, "generate_podcast", body)) as Record<string, unknown>;
    return NextResponse.json(result);
  } catch (err) {
    const message = err instanceof Error ? err.message : "Generation failed";
    return NextResponse.json({ error: message }, { status: 500 });
  }
}
