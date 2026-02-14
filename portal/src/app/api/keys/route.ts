import { NextResponse } from "next/server";
import { auth, canCreate } from "@/lib/auth";
import { createAPIKey, listAPIKeys } from "@/lib/db";

export async function GET() {
  const session = await auth();
  if (!session?.user?.id) {
    return NextResponse.json({ error: "Unauthorized" }, { status: 401 });
  }
  if (!canCreate(session.user.role)) {
    return NextResponse.json({ error: "Creator access required" }, { status: 403 });
  }
  const keys = await listAPIKeys(session.user.id);
  return NextResponse.json(keys);
}

export async function POST(request: Request) {
  const session = await auth();
  if (!session?.user?.id) {
    return NextResponse.json({ error: "Unauthorized" }, { status: 401 });
  }
  if (!canCreate(session.user.role)) {
    return NextResponse.json({ error: "Creator access required" }, { status: 403 });
  }
  const { name } = await request.json();
  if (!name || typeof name !== "string") {
    return NextResponse.json({ error: "Name is required" }, { status: 400 });
  }
  const result = await createAPIKey(session.user.id, name.trim());
  return NextResponse.json(result);
}
