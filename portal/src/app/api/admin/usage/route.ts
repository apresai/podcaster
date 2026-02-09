import { NextResponse } from "next/server";
import { auth } from "@/lib/auth";
import { listAllUsage } from "@/lib/db";

export async function GET() {
  const session = await auth();
  if (!session?.user?.id || session.user.role !== "admin") {
    return NextResponse.json({ error: "Forbidden" }, { status: 403 });
  }
  const usage = await listAllUsage();
  return NextResponse.json(usage);
}
