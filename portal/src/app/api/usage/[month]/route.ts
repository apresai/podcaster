import { NextResponse } from "next/server";
import { auth } from "@/lib/auth";
import { getMonthlyUsage } from "@/lib/db";

export async function GET(
  _request: Request,
  { params }: { params: Promise<{ month: string }> }
) {
  const session = await auth();
  if (!session?.user?.id) {
    return NextResponse.json({ error: "Unauthorized" }, { status: 401 });
  }
  const { month } = await params;
  const usage = await getMonthlyUsage(session.user.id, month);
  if (!usage) {
    return NextResponse.json({ error: "No usage data" }, { status: 404 });
  }
  return NextResponse.json(usage);
}
