import { NextResponse } from "next/server";
import { auth } from "@/lib/auth";
import { setUserRole } from "@/lib/db";

export async function POST(
  request: Request,
  { params }: { params: Promise<{ id: string }> }
) {
  const session = await auth();
  if (!session?.user?.id || session.user.role !== "admin") {
    return NextResponse.json({ error: "Forbidden" }, { status: 403 });
  }
  const { id } = await params;
  const { role } = await request.json();
  if (role !== "user" && role !== "creator") {
    return NextResponse.json({ error: "Invalid role" }, { status: 400 });
  }
  try {
    await setUserRole(id, role);
    return NextResponse.json({ success: true });
  } catch (e) {
    // ConditionalCheckFailedException = tried to change admin role
    const message = e instanceof Error ? e.message : "Unknown error";
    if (message.includes("ConditionalCheckFailed")) {
      return NextResponse.json({ error: "Cannot change admin role" }, { status: 403 });
    }
    return NextResponse.json({ error: message }, { status: 500 });
  }
}
