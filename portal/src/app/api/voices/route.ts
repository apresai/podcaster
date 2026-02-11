import { NextResponse } from "next/server";
import { auth } from "@/lib/auth";
import { getActiveAPIKeyForUser } from "@/lib/db";
import { callMCPTool } from "@/lib/mcp";

export async function GET(request: Request) {
  const session = await auth();
  if (!session?.user?.id) {
    return NextResponse.json({ error: "Unauthorized" }, { status: 401 });
  }

  const apiKey = await getActiveAPIKeyForUser(session.user.id);
  if (!apiKey) {
    return NextResponse.json({ error: "No API key found" }, { status: 400 });
  }

  const { searchParams } = new URL(request.url);
  const provider = searchParams.get("provider");
  if (!provider) {
    return NextResponse.json(
      { error: "provider query parameter is required" },
      { status: 400 },
    );
  }

  try {
    const result = await callMCPTool(apiKey, "list_voices", { provider });
    return NextResponse.json(result);
  } catch (err) {
    const message = err instanceof Error ? err.message : "Voice listing failed";
    return NextResponse.json({ error: message }, { status: 500 });
  }
}
