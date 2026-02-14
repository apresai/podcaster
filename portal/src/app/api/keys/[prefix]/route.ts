import { NextResponse } from "next/server";
import { auth, canCreate } from "@/lib/auth";
import { revokeAPIKey } from "@/lib/db";

export async function DELETE(
  _request: Request,
  { params }: { params: Promise<{ prefix: string }> }
) {
  const session = await auth();
  if (!session?.user?.id) {
    return NextResponse.json({ error: "Unauthorized" }, { status: 401 });
  }
  if (!canCreate(session.user.role)) {
    return NextResponse.json({ error: "Creator access required" }, { status: 403 });
  }
  const { prefix } = await params;
  await revokeAPIKey(prefix);
  return NextResponse.json({ success: true });
}
