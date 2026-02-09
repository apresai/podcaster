import { NextResponse } from "next/server";
import { auth } from "@/lib/auth";
import { revokeAPIKey } from "@/lib/db";

export async function DELETE(
  _request: Request,
  { params }: { params: Promise<{ prefix: string }> }
) {
  const session = await auth();
  if (!session?.user?.id) {
    return NextResponse.json({ error: "Unauthorized" }, { status: 401 });
  }
  const { prefix } = await params;
  await revokeAPIKey(prefix);
  return NextResponse.json({ success: true });
}
