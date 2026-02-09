import { NextResponse } from "next/server";
import { auth } from "@/lib/auth";
import { approveUser } from "@/lib/db";

export async function POST(
  _request: Request,
  { params }: { params: Promise<{ id: string }> }
) {
  const session = await auth();
  if (!session?.user?.id || session.user.role !== "admin") {
    return NextResponse.json({ error: "Forbidden" }, { status: 403 });
  }
  const { id } = await params;
  await approveUser(id);
  return NextResponse.json({ success: true });
}
