import { NextResponse } from "next/server";
import { auth } from "@/lib/auth";
import { getActiveAPIKeyForUser } from "@/lib/db";
import { callMCPTool } from "@/lib/mcp";

export async function GET(
  _request: Request,
  { params }: { params: Promise<{ id: string }> }
) {
  const session = await auth();
  if (!session?.user?.id) {
    return NextResponse.json({ error: "Unauthorized" }, { status: 401 });
  }

  const apiKey = await getActiveAPIKeyForUser(session.user.id);
  if (!apiKey) {
    return NextResponse.json({ error: "No active API key" }, { status: 400 });
  }

  const { id } = await params;
  const result = await callMCPTool(apiKey, "get_podcast", { podcast_id: id });
  return NextResponse.json(result);
}
