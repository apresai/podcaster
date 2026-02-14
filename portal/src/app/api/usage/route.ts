import { NextResponse } from "next/server";
import { auth, canCreate } from "@/lib/auth";
import { listMonthlyUsage } from "@/lib/db";

export async function GET() {
  const session = await auth();
  if (!session?.user?.id) {
    return NextResponse.json({ error: "Unauthorized" }, { status: 401 });
  }
  if (!canCreate(session.user.role)) {
    return NextResponse.json({ error: "Creator access required" }, { status: 403 });
  }
  const usage = await listMonthlyUsage(session.user.id);
  return NextResponse.json(usage);
}
