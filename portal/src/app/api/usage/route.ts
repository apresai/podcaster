import { NextResponse } from "next/server";
import { auth } from "@/lib/auth";
import { listMonthlyUsage } from "@/lib/db";

export async function GET() {
  const session = await auth();
  if (!session?.user?.id) {
    return NextResponse.json({ error: "Unauthorized" }, { status: 401 });
  }
  const usage = await listMonthlyUsage(session.user.id);
  return NextResponse.json(usage);
}
